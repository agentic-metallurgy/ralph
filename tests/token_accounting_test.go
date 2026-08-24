package tests

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudosai/ralph-go/internal/parser"
	"github.com/cloudosai/ralph-go/internal/stats"
)

// lastResultUsage returns the usage carried on the final main-loop result line
// of a captured stream, along with the cost the CLI reported for it.
func lastResultUsage(t *testing.T, name string) (*parser.Usage, string, float64) {
	t.Helper()

	f, err := os.Open(filepath.Join("fixtures", name))
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	p := parser.NewParser()
	var usage *parser.Usage
	var cost float64
	var model string

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		parsed := p.ParseLine(sc.Text())
		if parsed == nil {
			continue
		}
		if m := p.GetModel(parsed); m != "" && model == "" {
			model = m
		}
		if u := p.GetResultUsage(parsed); u != nil && !p.IsSubagentMessage(parsed) {
			usage, cost = u, parsed.TotalCostUSD
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning fixture: %v", err)
	}
	if usage == nil {
		t.Fatalf("fixture %s has no main-loop result line carrying usage", name)
	}
	return usage, model, cost
}

// TestGetResultUsageReadsTopLevelUsage covers the shape difference that hid the
// authoritative counts: a result line carries `usage` at the top level, not
// under `message`, so GetUsage cannot see it.
func TestGetResultUsageReadsTopLevelUsage(t *testing.T) {
	p := parser.NewParser()
	line := `{"type":"result","total_cost_usd":0.5,"usage":{"input_tokens":4,"output_tokens":148,` +
		`"cache_creation_input_tokens":9771,"cache_read_input_tokens":43714,` +
		`"cache_creation":{"ephemeral_1h_input_tokens":9771,"ephemeral_5m_input_tokens":0}}}`

	parsed := p.ParseLine(line)
	if parsed == nil {
		t.Fatal("failed to parse result line")
	}
	if got := p.GetUsage(parsed); got != nil {
		t.Errorf("GetUsage on a result line = %+v, want nil (usage is top-level there)", got)
	}
	u := p.GetResultUsage(parsed)
	if u == nil {
		t.Fatal("GetResultUsage returned nil for a result line carrying usage")
	}
	if u.OutputTokens != 148 {
		t.Errorf("output tokens = %d, want 148", u.OutputTokens)
	}
	if u.CacheCreation1h() != 9771 || u.CacheCreation5m() != 0 {
		t.Errorf("cache split = 1h:%d 5m:%d, want 1h:9771 5m:0", u.CacheCreation1h(), u.CacheCreation5m())
	}

	// Non-result messages carry none.
	assistant := p.ParseLine(`{"type":"assistant","message":{"id":"m","content":[],"usage":{"output_tokens":3}}}`)
	if got := p.GetResultUsage(assistant); got != nil {
		t.Errorf("GetResultUsage on an assistant message = %+v, want nil", got)
	}
}

// TestCacheCreationSplitFallback checks the legacy shape: when the CLI omits the
// per-TTL breakdown, the whole figure is attributed to the 5-minute TTL, which
// is how it was priced before the split existed.
func TestCacheCreationSplitFallback(t *testing.T) {
	p := parser.NewParser()
	parsed := p.ParseLine(`{"type":"result","usage":{"cache_creation_input_tokens":5000}}`)
	u := p.GetResultUsage(parsed)
	if u == nil {
		t.Fatal("GetResultUsage returned nil")
	}
	if u.CacheCreation5m() != 5000 || u.CacheCreation1h() != 0 {
		t.Errorf("fallback split = 1h:%d 5m:%d, want 1h:0 5m:5000", u.CacheCreation1h(), u.CacheCreation5m())
	}

	var nilUsage *parser.Usage
	if nilUsage.CacheCreation5m() != 0 || nilUsage.CacheCreation1h() != 0 {
		t.Error("nil Usage should report zero cache creation")
	}
}

// TestEstimateCostMatchesReportedCost prices the settled usage from real captures
// and compares against the cost the CLI billed. It is the guard on the pricing
// table: cache writes bill at 2x input for a 1-hour TTL, and the CLI defaults to
// 1 hour, so applying the 1.25x five-minute rate understates a cache-heavy
// iteration by roughly a third.
func TestEstimateCostMatchesReportedCost(t *testing.T) {
	for _, name := range []string{"real_stream_usage.jsonl", "real_stream_usage_resumed.jsonl"} {
		t.Run(name, func(t *testing.T) {
			u, model, reported := lastResultUsage(t, name)
			if model == "" {
				t.Fatal("fixture carries no model identifier")
			}

			est := stats.EstimateCost(model,
				u.InputTokens, u.OutputTokens,
				u.CacheCreation5m(), u.CacheCreation1h(), u.CacheReadInputTokens)

			// 1% tolerance: the CLI rounds, and a stream may carry a stray
			// server-tool charge the usage block does not itemize.
			if diff := est - reported; diff > reported*0.01 || diff < -reported*0.01 {
				t.Errorf("cost for %s: estimated $%.6f, CLI reported $%.6f (off by %.1f%%); "+
					"cache writes were %d tokens at 1h TTL",
					model, est, reported, 100*diff/reported, u.CacheCreation1h())
			}
		})
	}
}

// TestEstimateCostCacheTTLRates asserts the two cache-write rates directly.
func TestEstimateCostCacheTTLRates(t *testing.T) {
	const million = 1_000_000
	tests := []struct {
		name       string
		model      string
		cc5m, cc1h int64
		want       float64
	}{
		{"opus 1M at 5m TTL", "claude-opus-4-8", million, 0, 6.25},
		{"opus 1M at 1h TTL", "claude-opus-4-8", 0, million, 10.00},
		{"sonnet 1M at 5m TTL", "claude-sonnet-4-6", million, 0, 3.75},
		{"sonnet 1M at 1h TTL", "claude-sonnet-4-6", 0, million, 6.00},
		{"haiku 1M at 1h TTL", "claude-haiku-4-5", 0, million, 2.00},
		{"fable 1M at 1h TTL", "claude-fable-5", 0, million, 20.00},
		{"mixed TTLs sum", "claude-opus-4-8", million, million, 16.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stats.EstimateCost(tt.model, 0, 0, tt.cc5m, tt.cc1h, 0)
			if diff := got - tt.want; diff > 1e-7 || diff < -1e-7 {
				t.Errorf("EstimateCost = %f, want %f", got, tt.want)
			}
		})
	}

	// The 1h rate is exactly 2x input and the 5m rate exactly 1.25x, for every tier.
	for _, model := range []string{"claude-opus-4-8", "claude-sonnet-4-6", "claude-haiku-4-5", "claude-fable-5"} {
		p := stats.PricingForModel(model)
		if diff := p.CacheCreation1h - p.Input*2; diff > 1e-12 || diff < -1e-12 {
			t.Errorf("%s: 1h cache write %v, want 2x input %v", model, p.CacheCreation1h, p.Input*2)
		}
		if diff := p.CacheCreation - p.Input*1.25; diff > 1e-12 || diff < -1e-12 {
			t.Errorf("%s: 5m cache write %v, want 1.25x input %v", model, p.CacheCreation, p.Input*1.25)
		}
	}
}

// TestReconcileUsage covers the swap of estimated token counts for actual ones.
func TestReconcileUsage(t *testing.T) {
	s := stats.NewTokenStats()
	s.AddUsage(4, 4, 9771, 43714) // as streamed: output is a partial snapshot

	estimated := stats.TokenDelta{Input: 4, Output: 4, CacheCreation: 9771, CacheRead: 43714}
	actual := stats.TokenDelta{Input: 4, Output: 148, CacheCreation: 9771, CacheRead: 43714}
	s.ReconcileUsage(estimated, actual)

	snap := s.Snapshot()
	if snap.OutputTokens != 148 {
		t.Errorf("output tokens = %d, want 148", snap.OutputTokens)
	}
	if snap.TotalTokensCount != 4+148+9771+43714 {
		t.Errorf("total = %d, want %d", snap.TotalTokensCount, 4+148+9771+43714)
	}

	// Subagent tokens booked outside the reconciled delta must survive.
	s2 := stats.NewTokenStats()
	s2.AddUsage(0, 0, 0, 294165) // subagent traffic
	s2.AddUsage(6, 12, 20479, 18693)
	s2.ReconcileUsage(
		stats.TokenDelta{Input: 6, Output: 12, CacheCreation: 20479, CacheRead: 18693},
		stats.TokenDelta{Input: 6, Output: 1063, CacheCreation: 20479, CacheRead: 18693},
	)
	if got := s2.Snapshot().CacheReadTokens; got != 294165+18693 {
		t.Errorf("cache read = %d, want %d — subagent tokens were dropped", got, 294165+18693)
	}

	// An over-large estimate must never drive a counter negative.
	s3 := stats.NewTokenStats()
	s3.AddUsage(10, 10, 10, 10)
	s3.ReconcileUsage(stats.TokenDelta{Input: 999, Output: 999, CacheCreation: 999, CacheRead: 999}, stats.TokenDelta{})
	if snap := s3.Snapshot(); snap.InputTokens != 0 || snap.TotalTokensCount != 0 {
		t.Errorf("counters went negative: %+v", snap)
	}
}

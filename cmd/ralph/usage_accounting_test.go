package main

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudosai/ralph-go/internal/parser"
	"github.com/cloudosai/ralph-go/internal/stats"
)

// fixturePath resolves a capture in tests/fixtures relative to cmd/ralph.
func fixturePath(name string) string {
	return filepath.Join("..", "..", "tests", "fixtures", name)
}

// replayStream feeds a captured stream-json capture through usageAccounting the
// same way processLoopOutput does, and returns the resulting stats plus the
// result line for comparison.
func replayStream(t *testing.T, name string) (*stats.TokenStats, *parser.ParsedMessage, *parser.Parser) {
	t.Helper()

	f, err := os.Open(fixturePath(name))
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	acct := newUsageAccounting()
	var result *parser.ParsedMessage

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		parsed := jsonParser.ParseLine(sc.Text())
		if parsed == nil {
			continue
		}
		acct.recordUsage(jsonParser, parsed, tokenStats)
		acct.reconcileResult(jsonParser, parsed, tokenStats)
		if parsed.Type == parser.MessageTypeResult && !jsonParser.IsSubagentMessage(parsed) {
			result = parsed
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning fixture: %v", err)
	}
	if result == nil {
		t.Fatalf("fixture %s carries no main-loop result line", name)
	}
	return tokenStats, result, jsonParser
}

// TestUsageAccountingReconcilesAgainstResult is the regression guard for token
// accounting. The fixtures are real `claude --print --output-format stream-json
// --verbose` captures, so they encode the CLI's actual emission behavior rather
// than an assumption about it.
//
// A failure on output tokens alone means the streamed placeholder is being kept
// instead of the settled figure from the result line.
func TestUsageAccountingReconcilesAgainstResult(t *testing.T) {
	for _, name := range []string{"real_stream_usage.jsonl", "real_stream_usage_resumed.jsonl"} {
		t.Run(name, func(t *testing.T) {
			tokenStats, result, jsonParser := replayStream(t, name)
			settled := jsonParser.GetResultUsage(result)
			if settled == nil {
				t.Fatal("result line carries no usage")
			}
			snap := tokenStats.Snapshot()

			for _, c := range []struct {
				field     string
				got, want int64
			}{
				{"input", snap.InputTokens, settled.InputTokens},
				{"output", snap.OutputTokens, settled.OutputTokens},
				{"cache creation", snap.CacheCreationTokens, settled.CacheCreationInputTokens},
				{"cache read", snap.CacheReadTokens, settled.CacheReadInputTokens},
			} {
				if c.got != c.want {
					t.Errorf("%s tokens: accounted %d, result line reports %d (off by %d)",
						c.field, c.got, c.want, c.want-c.got)
				}
			}
		})
	}
}

// TestUsageAccountingReconcilesCost checks that the reconciled cost lands on the
// figure the CLI reports, rather than on the streamed estimate.
func TestUsageAccountingReconcilesCost(t *testing.T) {
	for _, name := range []string{"real_stream_usage.jsonl", "real_stream_usage_resumed.jsonl"} {
		t.Run(name, func(t *testing.T) {
			tokenStats, result, _ := replayStream(t, name)
			got := tokenStats.Snapshot().TotalCostUSD
			want := result.TotalCostUSD
			if diff := got - want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("reconciled cost = $%.6f, CLI reported $%.6f", got, want)
			}
		})
	}
}

// TestUsageAccountingKeepsSubagentTokens guards the asymmetry that makes this
// reconciliation subtle: a result line's usage covers the main loop only, while
// its total_cost_usd covers subagents too. Reconciling the whole token total
// against it would silently discard every subagent's tokens.
func TestUsageAccountingKeepsSubagentTokens(t *testing.T) {
	tokenStats, result, jsonParser := replayStream(t, "subagent_cost_session.json")
	settled := jsonParser.GetResultUsage(result)
	if settled == nil {
		t.Fatal("result line carries no usage")
	}
	snap := tokenStats.Snapshot()

	// The capture's subagents stream ~294k cache-read tokens that the result
	// line does not account for; they must survive reconciliation.
	if snap.CacheReadTokens <= settled.CacheReadInputTokens {
		t.Errorf("cache read tokens = %d, want more than the result line's main-loop %d — subagent tokens were dropped",
			snap.CacheReadTokens, settled.CacheReadInputTokens)
	}
	if snap.OutputTokens < settled.OutputTokens {
		t.Errorf("output tokens = %d, want at least the result line's %d",
			snap.OutputTokens, settled.OutputTokens)
	}
	// Cost still reconciles exactly: total_cost_usd already includes subagents.
	if diff := snap.TotalCostUSD - result.TotalCostUSD; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("reconciled cost = $%.6f, CLI reported $%.6f", snap.TotalCostUSD, result.TotalCostUSD)
	}
}

// TestUsageAccountingIgnoresDuplicateChunks verifies the dedup still holds: the
// CLI emits one assistant event per content block, all carrying identical usage.
func TestUsageAccountingIgnoresDuplicateChunks(t *testing.T) {
	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	acct := newUsageAccounting()

	line := `{"type":"assistant","message":{"id":"msg_dup","model":"claude-opus-5",` +
		`"content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":10,"output_tokens":5,` +
		`"cache_creation_input_tokens":100,"cache_read_input_tokens":1000}}}`

	for i := 0; i < 3; i++ {
		parsed := jsonParser.ParseLine(line)
		added, isNew := acct.recordUsage(jsonParser, parsed, tokenStats)
		if i == 0 {
			if !isNew || added != 1115 {
				t.Fatalf("first chunk: added=%d isNew=%v, want 1115/true", added, isNew)
			}
			continue
		}
		if isNew || added != 0 {
			t.Errorf("chunk %d: added=%d isNew=%v, want 0/false", i, added, isNew)
		}
	}
	if got := tokenStats.Snapshot().TotalTokensCount; got != 1115 {
		t.Errorf("total tokens = %d, want 1115", got)
	}
}

// TestResetIterationClearsAccumulators checks the loop-boundary reset, and that
// lastResultCost survives it — it tracks the CLI's running total across the
// iterations of a resumed session.
func TestResetIterationClearsAccumulators(t *testing.T) {
	acct := newUsageAccounting()
	acct.iterEstimate = 1.5
	acct.subagentCostAccum = 0.25
	acct.lastResultCost = 9.0
	acct.iterMainTokens.Add(1, 2, 3, 4)
	acct.seenMsgIDs["msg_x"] = true

	acct.resetIteration()

	if acct.iterEstimate != 0 || acct.subagentCostAccum != 0 {
		t.Errorf("cost accumulators not cleared: %+v", acct)
	}
	if acct.iterMainTokens != (stats.TokenDelta{}) {
		t.Errorf("iterMainTokens = %+v, want zero", acct.iterMainTokens)
	}
	if len(acct.seenMsgIDs) != 0 {
		t.Errorf("seenMsgIDs = %v, want empty", acct.seenMsgIDs)
	}
	if acct.lastResultCost != 9.0 {
		t.Errorf("lastResultCost = %v, want it preserved at 9.0", acct.lastResultCost)
	}
}

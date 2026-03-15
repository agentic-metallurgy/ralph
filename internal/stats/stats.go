package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// TokenStats tracks token usage and costs.
// All mutating methods are protected by a sync.RWMutex for concurrent access
// from the processLoopOutput goroutine (writer) and the BubbleTea TUI goroutine (reader).
type TokenStats struct {
	mu                  sync.RWMutex `json:"-"`
	InputTokens         int64        `json:"input_tokens"`
	OutputTokens        int64        `json:"output_tokens"`
	CacheCreationTokens int64        `json:"cache_creation_tokens"`
	CacheReadTokens     int64        `json:"cache_read_tokens"`
	TotalCostUSD        float64      `json:"total_cost"`
	TotalTokensCount    int64        `json:"total_tokens"`
	TotalElapsedNs      int64        `json:"elapsed_ns"`
}

// NewTokenStats creates a new empty TokenStats instance
func NewTokenStats() *TokenStats {
	return &TokenStats{
		InputTokens:         0,
		OutputTokens:        0,
		CacheCreationTokens: 0,
		CacheReadTokens:     0,
		TotalCostUSD:        0.0,
		TotalTokensCount:    0,
		TotalElapsedNs:      0,
	}
}

// AddUsage adds token usage counts to the stats
func (t *TokenStats) AddUsage(input, output, cacheCreation, cacheRead int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.InputTokens += input
	t.OutputTokens += output
	t.CacheCreationTokens += cacheCreation
	t.CacheReadTokens += cacheRead
	t.TotalTokensCount = t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

// Pricing constants for Claude Sonnet 4 (per token)
const (
	PriceInputPerToken         = 3.00 / 1_000_000
	PriceOutputPerToken        = 15.00 / 1_000_000
	PriceCacheCreationPerToken = 3.75 / 1_000_000
	PriceCacheReadPerToken     = 0.30 / 1_000_000
)

// EstimateCostFromTokens computes estimated cost from token counts using hardcoded pricing
func EstimateCostFromTokens(input, output, cacheCreation, cacheRead int64) float64 {
	return float64(input)*PriceInputPerToken +
		float64(output)*PriceOutputPerToken +
		float64(cacheCreation)*PriceCacheCreationPerToken +
		float64(cacheRead)*PriceCacheReadPerToken
}

// AddCost adds cost to the total cost
func (t *TokenStats) AddCost(costUSD float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalCostUSD += costUSD
}

// ReconcileCost replaces an estimated cost delta with the actual cost.
// It subtracts the estimated amount and adds the actual amount.
func (t *TokenStats) ReconcileCost(estimatedDelta, actualCost float64) {
	t.TotalCostUSD -= estimatedDelta
	t.TotalCostUSD += actualCost
}

// TotalTokens returns the sum of all token counts
func (t *TokenStats) TotalTokens() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

// SetTotalElapsedNs sets the total elapsed time in nanoseconds (thread-safe)
func (t *TokenStats) SetTotalElapsedNs(ns int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalElapsedNs = ns
}

// Snapshot returns a consistent point-in-time copy of the stats for reading.
// The returned value has a fresh (zero) mutex and can be read without locking.
func (t *TokenStats) Snapshot() TokenStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return TokenStats{
		InputTokens:         t.InputTokens,
		OutputTokens:        t.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens,
		TotalCostUSD:        t.TotalCostUSD,
		TotalTokensCount:    t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens,
		TotalElapsedNs:      t.TotalElapsedNs,
	}
}

// FormatTokens formats a token count into a human-readable string
// e.g., 36870000 → "36.87m", 300000 → "300k", 1500 → "1.5k", 42 → "42"
func FormatTokens(count int64) string {
	switch {
	case count >= 1_000_000:
		return fmt.Sprintf("%.2fm", float64(count)/1_000_000)
	case count >= 1_000:
		val := float64(count) / 1_000
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dk", int64(val))
		}
		return fmt.Sprintf("%.1fk", val)
	default:
		return fmt.Sprintf("%d", count)
	}
}

// Save persists the stats to a JSON file at the given path
func (t *TokenStats) Save(path string) error {
	// Take a consistent snapshot under lock, then do I/O outside the lock
	t.mu.Lock()
	t.TotalTokensCount = t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
	snap := TokenStats{
		InputTokens:         t.InputTokens,
		OutputTokens:        t.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens,
		TotalCostUSD:        t.TotalCostUSD,
		TotalTokensCount:    t.TotalTokensCount,
		TotalElapsedNs:      t.TotalElapsedNs,
	}
	t.mu.Unlock()

	data, err := json.MarshalIndent(&snap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadTokenStats loads stats from a JSON file, returns empty stats if file doesn't exist
func LoadTokenStats(path string) (*TokenStats, error) {
	// If file doesn't exist, return empty stats
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return NewTokenStats(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var stats TokenStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

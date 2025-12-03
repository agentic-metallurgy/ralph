package stats

import (
	"encoding/json"
	"os"
)

// TokenStats tracks token usage and costs
type TokenStats struct {
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalCostUSD        float64 `json:"total_cost"`
	TotalTokensCount    int64   `json:"total_tokens"`
	TotalElapsedNs      int64   `json:"elapsed_ns"`
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
	t.InputTokens += input
	t.OutputTokens += output
	t.CacheCreationTokens += cacheCreation
	t.CacheReadTokens += cacheRead
	t.TotalTokensCount = t.TotalTokens()
}

// AddCost adds cost to the total cost
func (t *TokenStats) AddCost(costUSD float64) {
	t.TotalCostUSD += costUSD
}

// TotalTokens returns the sum of all token counts
func (t *TokenStats) TotalTokens() int64 {
	return t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

// Save persists the stats to a JSON file at the given path
func (t *TokenStats) Save(path string) error {
	// Update total tokens before saving
	t.TotalTokensCount = t.TotalTokens()

	data, err := json.MarshalIndent(t, "", "  ")
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

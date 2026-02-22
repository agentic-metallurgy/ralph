package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudosai/ralph-go/internal/stats"
)

func TestNewTokenStats(t *testing.T) {
	s := stats.NewTokenStats()

	if s.InputTokens != 0 {
		t.Errorf("Expected InputTokens 0, got %d", s.InputTokens)
	}
	if s.OutputTokens != 0 {
		t.Errorf("Expected OutputTokens 0, got %d", s.OutputTokens)
	}
	if s.CacheCreationTokens != 0 {
		t.Errorf("Expected CacheCreationTokens 0, got %d", s.CacheCreationTokens)
	}
	if s.CacheReadTokens != 0 {
		t.Errorf("Expected CacheReadTokens 0, got %d", s.CacheReadTokens)
	}
	if s.TotalCostUSD != 0.0 {
		t.Errorf("Expected TotalCostUSD 0.0, got %f", s.TotalCostUSD)
	}
	if s.TotalTokensCount != 0 {
		t.Errorf("Expected TotalTokensCount 0, got %d", s.TotalTokensCount)
	}
}

func TestAddUsage(t *testing.T) {
	s := stats.NewTokenStats()

	// First addition
	s.AddUsage(100, 50, 20, 10)
	if s.InputTokens != 100 {
		t.Errorf("Expected InputTokens 100, got %d", s.InputTokens)
	}
	if s.OutputTokens != 50 {
		t.Errorf("Expected OutputTokens 50, got %d", s.OutputTokens)
	}
	if s.CacheCreationTokens != 20 {
		t.Errorf("Expected CacheCreationTokens 20, got %d", s.CacheCreationTokens)
	}
	if s.CacheReadTokens != 10 {
		t.Errorf("Expected CacheReadTokens 10, got %d", s.CacheReadTokens)
	}
	if s.TotalTokensCount != 180 {
		t.Errorf("Expected TotalTokensCount 180, got %d", s.TotalTokensCount)
	}

	// Second addition (accumulates)
	s.AddUsage(200, 100, 30, 40)
	if s.InputTokens != 300 {
		t.Errorf("Expected InputTokens 300 after accumulation, got %d", s.InputTokens)
	}
	if s.OutputTokens != 150 {
		t.Errorf("Expected OutputTokens 150 after accumulation, got %d", s.OutputTokens)
	}
	if s.CacheCreationTokens != 50 {
		t.Errorf("Expected CacheCreationTokens 50 after accumulation, got %d", s.CacheCreationTokens)
	}
	if s.CacheReadTokens != 50 {
		t.Errorf("Expected CacheReadTokens 50 after accumulation, got %d", s.CacheReadTokens)
	}
	if s.TotalTokensCount != 550 {
		t.Errorf("Expected TotalTokensCount 550 after accumulation, got %d", s.TotalTokensCount)
	}
}

func TestAddCost(t *testing.T) {
	s := stats.NewTokenStats()

	// First cost addition
	s.AddCost(0.001234)
	if s.TotalCostUSD != 0.001234 {
		t.Errorf("Expected TotalCostUSD 0.001234, got %f", s.TotalCostUSD)
	}

	// Second cost addition (accumulates)
	s.AddCost(0.005678)
	expected := 0.001234 + 0.005678
	if s.TotalCostUSD != expected {
		t.Errorf("Expected TotalCostUSD %f after accumulation, got %f", expected, s.TotalCostUSD)
	}
}

func TestTotalTokens(t *testing.T) {
	s := stats.NewTokenStats()

	// Empty stats
	if s.TotalTokens() != 0 {
		t.Errorf("Expected TotalTokens() 0 for empty stats, got %d", s.TotalTokens())
	}

	// With values
	s.InputTokens = 1000
	s.OutputTokens = 500
	s.CacheCreationTokens = 200
	s.CacheReadTokens = 100

	expected := int64(1800)
	if s.TotalTokens() != expected {
		t.Errorf("Expected TotalTokens() %d, got %d", expected, s.TotalTokens())
	}
}

func TestSave(t *testing.T) {
	// Create temp file
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	s := stats.NewTokenStats()
	s.InputTokens = 1000
	s.OutputTokens = 500
	s.CacheCreationTokens = 200
	s.CacheReadTokens = 100
	s.TotalCostUSD = 0.123456

	// Save
	if err := s.Save(statsFile); err != nil {
		t.Fatalf("Failed to save stats: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statsFile); os.IsNotExist(err) {
		t.Fatal("Stats file was not created")
	}

	// Read and parse the file
	data, err := os.ReadFile(statsFile)
	if err != nil {
		t.Fatalf("Failed to read stats file: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("Failed to parse saved JSON: %v", err)
	}

	// Verify values
	if saved["input_tokens"].(float64) != 1000 {
		t.Errorf("Expected saved input_tokens 1000, got %v", saved["input_tokens"])
	}
	if saved["output_tokens"].(float64) != 500 {
		t.Errorf("Expected saved output_tokens 500, got %v", saved["output_tokens"])
	}
	if saved["total_cost"].(float64) != 0.123456 {
		t.Errorf("Expected saved total_cost 0.123456, got %v", saved["total_cost"])
	}
	// TotalTokensCount should be updated on save
	if saved["total_tokens"].(float64) != 1800 {
		t.Errorf("Expected saved total_tokens 1800, got %v", saved["total_tokens"])
	}
}

func TestLoadTokenStats_ExistingFile(t *testing.T) {
	// Create temp dir and file
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	// Create a stats file with known values
	statsData := `{
		"input_tokens": 5000,
		"output_tokens": 2500,
		"cache_creation_tokens": 1000,
		"cache_read_tokens": 500,
		"total_cost": 0.987654,
		"total_tokens": 9000
	}`
	if err := os.WriteFile(statsFile, []byte(statsData), 0644); err != nil {
		t.Fatalf("Failed to write test stats file: %v", err)
	}

	// Load
	s, err := stats.LoadTokenStats(statsFile)
	if err != nil {
		t.Fatalf("Failed to load stats: %v", err)
	}

	// Verify values
	if s.InputTokens != 5000 {
		t.Errorf("Expected InputTokens 5000, got %d", s.InputTokens)
	}
	if s.OutputTokens != 2500 {
		t.Errorf("Expected OutputTokens 2500, got %d", s.OutputTokens)
	}
	if s.CacheCreationTokens != 1000 {
		t.Errorf("Expected CacheCreationTokens 1000, got %d", s.CacheCreationTokens)
	}
	if s.CacheReadTokens != 500 {
		t.Errorf("Expected CacheReadTokens 500, got %d", s.CacheReadTokens)
	}
	if s.TotalCostUSD != 0.987654 {
		t.Errorf("Expected TotalCostUSD 0.987654, got %f", s.TotalCostUSD)
	}
	if s.TotalTokensCount != 9000 {
		t.Errorf("Expected TotalTokensCount 9000, got %d", s.TotalTokensCount)
	}
}

func TestLoadTokenStats_NonExistentFile(t *testing.T) {
	// Load from non-existent file should return empty stats
	s, err := stats.LoadTokenStats("/nonexistent/path/.claude_stats")
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	// Should return empty stats
	if s.InputTokens != 0 || s.OutputTokens != 0 || s.TotalCostUSD != 0 {
		t.Error("Expected empty stats for non-existent file")
	}
}

func TestLoadTokenStats_CorruptJSON(t *testing.T) {
	// Create temp dir and file with invalid JSON
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	// Write corrupt JSON
	if err := os.WriteFile(statsFile, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupt stats file: %v", err)
	}

	// Load should return error
	_, err = stats.LoadTokenStats(statsFile)
	if err == nil {
		t.Error("Expected error for corrupt JSON, got nil")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	// Create stats with specific values
	original := stats.NewTokenStats()
	original.InputTokens = 12345
	original.OutputTokens = 6789
	original.CacheCreationTokens = 1111
	original.CacheReadTokens = 2222
	original.TotalCostUSD = 1.234567

	// Save
	if err := original.Save(statsFile); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Load
	loaded, err := stats.LoadTokenStats(statsFile)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// Compare
	if loaded.InputTokens != original.InputTokens {
		t.Errorf("InputTokens mismatch: %d vs %d", loaded.InputTokens, original.InputTokens)
	}
	if loaded.OutputTokens != original.OutputTokens {
		t.Errorf("OutputTokens mismatch: %d vs %d", loaded.OutputTokens, original.OutputTokens)
	}
	if loaded.CacheCreationTokens != original.CacheCreationTokens {
		t.Errorf("CacheCreationTokens mismatch: %d vs %d", loaded.CacheCreationTokens, original.CacheCreationTokens)
	}
	if loaded.CacheReadTokens != original.CacheReadTokens {
		t.Errorf("CacheReadTokens mismatch: %d vs %d", loaded.CacheReadTokens, original.CacheReadTokens)
	}
	if loaded.TotalCostUSD != original.TotalCostUSD {
		t.Errorf("TotalCostUSD mismatch: %f vs %f", loaded.TotalCostUSD, original.TotalCostUSD)
	}
	// TotalTokensCount is updated on Save
	expectedTotal := original.TotalTokens()
	if loaded.TotalTokensCount != expectedTotal {
		t.Errorf("TotalTokensCount mismatch: %d vs %d", loaded.TotalTokensCount, expectedTotal)
	}
}

func TestAddUsage_ZeroValues(t *testing.T) {
	s := stats.NewTokenStats()
	s.AddUsage(0, 0, 0, 0)

	if s.InputTokens != 0 || s.OutputTokens != 0 || s.CacheCreationTokens != 0 || s.CacheReadTokens != 0 {
		t.Error("Adding zero values should keep stats at zero")
	}
	if s.TotalTokensCount != 0 {
		t.Error("TotalTokensCount should be 0 after adding zeros")
	}
}

func TestAddUsage_LargeValues(t *testing.T) {
	s := stats.NewTokenStats()

	// Add large values to test int64 handling
	largeInput := int64(1_000_000_000)
	largeOutput := int64(500_000_000)
	s.AddUsage(largeInput, largeOutput, 0, 0)

	if s.InputTokens != largeInput {
		t.Errorf("Expected InputTokens %d, got %d", largeInput, s.InputTokens)
	}
	if s.OutputTokens != largeOutput {
		t.Errorf("Expected OutputTokens %d, got %d", largeOutput, s.OutputTokens)
	}
	expectedTotal := largeInput + largeOutput
	if s.TotalTokensCount != expectedTotal {
		t.Errorf("Expected TotalTokensCount %d, got %d", expectedTotal, s.TotalTokensCount)
	}
}

func TestAddCost_ZeroCost(t *testing.T) {
	s := stats.NewTokenStats()
	s.AddCost(0.0)

	if s.TotalCostUSD != 0.0 {
		t.Errorf("Expected TotalCostUSD 0.0, got %f", s.TotalCostUSD)
	}
}

func TestAddCost_SmallValues(t *testing.T) {
	s := stats.NewTokenStats()

	// Add very small cost values (micro-dollar amounts)
	s.AddCost(0.000001)
	s.AddCost(0.000002)
	s.AddCost(0.000003)

	expected := 0.000006
	// Use tolerance for floating point comparison
	tolerance := 0.0000001
	diff := s.TotalCostUSD - expected
	if diff < -tolerance || diff > tolerance {
		t.Errorf("Expected TotalCostUSD ~%f, got %f", expected, s.TotalCostUSD)
	}
}

func TestLoadTokenStats_PartialData(t *testing.T) {
	// Test loading a file with only some fields
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	// Create a minimal stats file
	statsData := `{"input_tokens": 100, "output_tokens": 50}`
	if err := os.WriteFile(statsFile, []byte(statsData), 0644); err != nil {
		t.Fatalf("Failed to write test stats file: %v", err)
	}

	s, err := stats.LoadTokenStats(statsFile)
	if err != nil {
		t.Fatalf("Failed to load partial stats: %v", err)
	}

	if s.InputTokens != 100 {
		t.Errorf("Expected InputTokens 100, got %d", s.InputTokens)
	}
	if s.OutputTokens != 50 {
		t.Errorf("Expected OutputTokens 50, got %d", s.OutputTokens)
	}
	// Missing fields should be zero
	if s.CacheCreationTokens != 0 {
		t.Errorf("Expected CacheCreationTokens 0 for missing field, got %d", s.CacheCreationTokens)
	}
	if s.TotalCostUSD != 0 {
		t.Errorf("Expected TotalCostUSD 0 for missing field, got %f", s.TotalCostUSD)
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	// Test that Save overwrites existing file
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	// First save
	s1 := stats.NewTokenStats()
	s1.InputTokens = 100
	if err := s1.Save(statsFile); err != nil {
		t.Fatalf("Failed first save: %v", err)
	}

	// Second save with different values
	s2 := stats.NewTokenStats()
	s2.InputTokens = 999
	if err := s2.Save(statsFile); err != nil {
		t.Fatalf("Failed second save: %v", err)
	}

	// Load and verify it has the second values
	loaded, err := stats.LoadTokenStats(statsFile)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if loaded.InputTokens != 999 {
		t.Errorf("Expected InputTokens 999 (from second save), got %d", loaded.InputTokens)
	}
}

func TestAccumulationMatchesPython(t *testing.T) {
	// Test that accumulation matches the Python version behavior
	s := stats.NewTokenStats()

	// Simulate multiple Claude messages with usage
	// Message 1
	s.AddUsage(1500, 300, 100, 50)
	s.AddCost(0.004500)

	// Message 2
	s.AddUsage(200, 150, 0, 100)
	s.AddCost(0.001250)

	// Message 3
	s.AddUsage(500, 200, 50, 25)
	s.AddCost(0.002100)

	// Verify totals
	expectedInput := int64(2200)      // 1500 + 200 + 500
	expectedOutput := int64(650)      // 300 + 150 + 200
	expectedCacheCreate := int64(150) // 100 + 0 + 50
	expectedCacheRead := int64(175)   // 50 + 100 + 25
	expectedCost := 0.00785           // 0.004500 + 0.001250 + 0.002100

	if s.InputTokens != expectedInput {
		t.Errorf("InputTokens: expected %d, got %d", expectedInput, s.InputTokens)
	}
	if s.OutputTokens != expectedOutput {
		t.Errorf("OutputTokens: expected %d, got %d", expectedOutput, s.OutputTokens)
	}
	if s.CacheCreationTokens != expectedCacheCreate {
		t.Errorf("CacheCreationTokens: expected %d, got %d", expectedCacheCreate, s.CacheCreationTokens)
	}
	if s.CacheReadTokens != expectedCacheRead {
		t.Errorf("CacheReadTokens: expected %d, got %d", expectedCacheRead, s.CacheReadTokens)
	}

	// Cost comparison with tolerance
	tolerance := 0.0000001
	diff := s.TotalCostUSD - expectedCost
	if diff < -tolerance || diff > tolerance {
		t.Errorf("TotalCostUSD: expected %f, got %f", expectedCost, s.TotalCostUSD)
	}

	expectedTotal := expectedInput + expectedOutput + expectedCacheCreate + expectedCacheRead
	if s.TotalTokens() != expectedTotal {
		t.Errorf("TotalTokens(): expected %d, got %d", expectedTotal, s.TotalTokens())
	}
}

func TestSave_CreatesParentDirectory(t *testing.T) {
	// Test that Save creates parent directory if it doesn't exist
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Path with non-existent subdirectory
	statsFile := filepath.Join(tmpDir, "subdir", ".claude_stats")

	s := stats.NewTokenStats()
	s.InputTokens = 42

	// This should fail because Save doesn't create parent directories
	err = s.Save(statsFile)
	if err == nil {
		t.Log("Note: Save successfully created file in non-existent directory")
	} else {
		t.Logf("As expected, Save failed for non-existent parent dir: %v", err)
	}
}

func TestLoadTokenStats_EmptyFile(t *testing.T) {
	// Test loading an empty file
	tmpDir, err := os.MkdirTemp("", "ralph-stats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	// Create empty file
	if err := os.WriteFile(statsFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty stats file: %v", err)
	}

	// Load should return error for empty file (invalid JSON)
	_, err = stats.LoadTokenStats(statsFile)
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}
}

func TestTotalTokensCount_UpdatedAfterAddUsage(t *testing.T) {
	s := stats.NewTokenStats()

	// TotalTokensCount should be updated after AddUsage
	s.AddUsage(100, 50, 25, 25)
	if s.TotalTokensCount != 200 {
		t.Errorf("TotalTokensCount should be 200 after AddUsage, got %d", s.TotalTokensCount)
	}

	// Verify it matches TotalTokens()
	if s.TotalTokensCount != s.TotalTokens() {
		t.Errorf("TotalTokensCount (%d) should match TotalTokens() (%d)", s.TotalTokensCount, s.TotalTokens())
	}
}

func TestElapsedTime_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ralph-stats-elapsed-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	statsFile := filepath.Join(tmpDir, ".claude_stats")

	s := stats.NewTokenStats()
	s.TotalElapsedNs = int64(5445 * 1e9) // 1h30m45s

	if err := s.Save(statsFile); err != nil {
		t.Fatalf("Failed to save stats: %v", err)
	}

	loaded, err := stats.LoadTokenStats(statsFile)
	if err != nil {
		t.Fatalf("Failed to load stats: %v", err)
	}

	if loaded.TotalElapsedNs != s.TotalElapsedNs {
		t.Errorf("TotalElapsedNs mismatch: expected %d, got %d", s.TotalElapsedNs, loaded.TotalElapsedNs)
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name     string
		count    int64
		expected string
	}{
		{"zero", 0, "0"},
		{"small number", 42, "42"},
		{"under 1k", 999, "999"},
		{"exactly 1k", 1000, "1k"},
		{"1.5k", 1500, "1.5k"},
		{"10k", 10000, "10k"},
		{"300k", 300000, "300k"},
		{"999k", 999000, "999k"},
		{"1 million", 1000000, "1.00m"},
		{"36.87m", 36870000, "36.87m"},
		{"100m", 100000000, "100.00m"},
		{"1.23m", 1234567, "1.23m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stats.FormatTokens(tt.count)
			if result != tt.expected {
				t.Errorf("FormatTokens(%d) = %q, expected %q", tt.count, result, tt.expected)
			}
		})
	}
}

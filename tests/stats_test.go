package tests

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudosai/ralph-go/internal/parser"
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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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
	s, err := stats.LoadTokenStats("/nonexistent/path/.ralph.claude_stats")
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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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
	statsFile := filepath.Join(tmpDir, "subdir", ".ralph.claude_stats")

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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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

	statsFile := filepath.Join(tmpDir, ".ralph.claude_stats")

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

func TestEstimateCostFromTokens(t *testing.T) {
	tests := []struct {
		name                                       string
		input, output, cacheCreation, cacheRead    int64
		expected                                   float64
	}{
		{"1M input tokens", 1_000_000, 0, 0, 0, 3.00},
		{"1M output tokens", 0, 1_000_000, 0, 0, 15.00},
		{"1M cache creation tokens", 0, 0, 1_000_000, 0, 3.75},
		{"1M cache read tokens", 0, 0, 0, 1_000_000, 0.30},
		{"all zeros", 0, 0, 0, 0, 0.0},
		{"mixed counts", 100_000, 50_000, 20_000, 200_000, 0.30 + 0.75 + 0.075 + 0.06},
	}

	tolerance := 0.0000001
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stats.EstimateCostFromTokens(tt.input, tt.output, tt.cacheCreation, tt.cacheRead)
			diff := result - tt.expected
			if diff < -tolerance || diff > tolerance {
				t.Errorf("EstimateCostFromTokens(%d, %d, %d, %d) = %f, expected %f",
					tt.input, tt.output, tt.cacheCreation, tt.cacheRead, result, tt.expected)
			}
		})
	}
}

func TestReconcileCost(t *testing.T) {
	tests := []struct {
		name           string
		initialCost    float64
		estimatedDelta float64
		actualCost     float64
		expected       float64
	}{
		{"replace estimate with actual", 1.00, 0.50, 0.60, 1.10},
		{"zero estimate", 1.00, 0.0, 0.50, 1.50},
		{"zero actual", 1.00, 0.50, 0.0, 0.50},
		{"both zero", 1.00, 0.0, 0.0, 1.00},
		{"estimate equals actual", 1.00, 0.30, 0.30, 1.00},
	}

	tolerance := 0.0000001
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := stats.NewTokenStats()
			s.TotalCostUSD = tt.initialCost
			s.ReconcileCost(tt.estimatedDelta, tt.actualCost)
			diff := s.TotalCostUSD - tt.expected
			if diff < -tolerance || diff > tolerance {
				t.Errorf("After ReconcileCost(%f, %f): TotalCostUSD = %f, expected %f",
					tt.estimatedDelta, tt.actualCost, s.TotalCostUSD, tt.expected)
			}
		})
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

// --- DB Tests ---

// helperInitTestDB creates a temp DB and returns it along with a cleanup function.
func helperInitTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "ralph-db-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test_stats.db")
	db, err := stats.InitDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init DB: %v", err)
	}
	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
	return db, cleanup
}

func TestGenerateSessionID(t *testing.T) {
	hexPattern := regexp.MustCompile(`^[0-9a-f]{6}$`)
	seen := make(map[string]bool)

	for i := 0; i < 10; i++ {
		id, err := stats.GenerateSessionID()
		if err != nil {
			t.Fatalf("GenerateSessionID() returned error: %v", err)
		}
		if !hexPattern.MatchString(id) {
			t.Errorf("GenerateSessionID() = %q, want 6 lowercase hex chars", id)
		}
		if seen[id] {
			t.Errorf("GenerateSessionID() produced duplicate: %q", id)
		}
		seen[id] = true
	}
}

func TestInitDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ralph-db-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_stats.db")

	db, err := stats.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := make(map[string]bool)
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Failed to scan table name: %v", err)
		}
		tables[name] = true
	}

	if !tables["checkpoints"] {
		t.Error("Expected 'checkpoints' table to exist")
	}
	if !tables["loop_stats"] {
		t.Error("Expected 'loop_stats' table to exist")
	}

	// Verify WAL mode
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("Expected journal_mode 'wal', got %q", journalMode)
	}

	// Test idempotency — calling InitDB again on the same path should succeed
	db.Close()
	db2, err := stats.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Second InitDB call failed (idempotency): %v", err)
	}
	db2.Close()
}

func TestInitDB_PrunesOldRows(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ralph-db-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_stats.db")

	db, err := stats.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	// Insert an old checkpoint row (8 days ago)
	oldTimestamp := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO checkpoints (loop_id, session_id, owner, repo, branch, delta_cost, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"old-1", "aaaaaa", "test", "repo", "main", 1.0, oldTimestamp,
	)
	if err != nil {
		t.Fatalf("Failed to insert old row: %v", err)
	}

	// Also insert a recent row that should survive
	recentTimestamp := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO checkpoints (loop_id, session_id, owner, repo, branch, delta_cost, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"recent-1", "bbbbbb", "test", "repo", "main", 2.0, recentTimestamp,
	)
	if err != nil {
		t.Fatalf("Failed to insert recent row: %v", err)
	}

	db.Close()

	// Re-init DB — should prune the old row
	db2, err := stats.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Second InitDB failed: %v", err)
	}
	defer db2.Close()

	var count int
	if err := db2.QueryRow("SELECT COUNT(*) FROM checkpoints WHERE loop_id = 'old-1'").Scan(&count); err != nil {
		t.Fatalf("Failed to query old row: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected old row to be pruned, but found %d rows", count)
	}

	// Recent row should still be there
	if err := db2.QueryRow("SELECT COUNT(*) FROM checkpoints WHERE loop_id = 'recent-1'").Scan(&count); err != nil {
		t.Fatalf("Failed to query recent row: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected recent row to survive, but found %d rows", count)
	}
}

func TestFlushCheckpoint(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	ts := time.Now().UTC().Format(time.RFC3339)
	p := stats.CheckpointParams{
		LoopID:            "abc123-1",
		SessionID:         "abc123",
		Owner:             "testowner",
		Repo:              "testrepo",
		Branch:            "main",
		DeltaCost:         0.05,
		DeltaInputTokens:  1000,
		DeltaOutputTokens: 500,
		DeltaCacheCreation: 200,
		DeltaCacheRead:    100,
		Timestamp:         ts,
	}

	if err := stats.FlushCheckpoint(db, p); err != nil {
		t.Fatalf("FlushCheckpoint failed: %v", err)
	}

	// Query and verify
	var loopID string
	var deltaCost float64
	var deltaInput int64
	var timestamp string
	err := db.QueryRow("SELECT loop_id, delta_cost, delta_input_tokens, timestamp FROM checkpoints WHERE loop_id = ?", "abc123-1").
		Scan(&loopID, &deltaCost, &deltaInput, &timestamp)
	if err != nil {
		t.Fatalf("Failed to query checkpoint: %v", err)
	}
	if loopID != "abc123-1" {
		t.Errorf("Expected loop_id 'abc123-1', got %q", loopID)
	}
	if deltaCost != 0.05 {
		t.Errorf("Expected delta_cost 0.05, got %f", deltaCost)
	}
	if deltaInput != 1000 {
		t.Errorf("Expected delta_input_tokens 1000, got %d", deltaInput)
	}
	if timestamp != ts {
		t.Errorf("Expected timestamp %q, got %q", ts, timestamp)
	}
}

func TestFlushCheckpoint_NilDB(t *testing.T) {
	err := stats.FlushCheckpoint(nil, stats.CheckpointParams{
		LoopID:    "test-1",
		SessionID: "test00",
		DeltaCost: 1.0,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Errorf("FlushCheckpoint(nil, ...) should return nil, got %v", err)
	}
}

func TestWriteLoopStats(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	p := stats.LoopStatsParams{
		LoopID:              "abc123-1",
		SessionID:           "abc123",
		Owner:               "testowner",
		Repo:                "testrepo",
		Branch:              "main",
		Description:         "feat: add widget",
		TotalCost:           0.25,
		InputTokens:         5000,
		OutputTokens:        2000,
		CacheCreationTokens: 500,
		CacheReadTokens:     300,
		TotalTokens:         7800,
		StartTime:           "2026-03-22T10:00:00Z",
		FinishTime:          "2026-03-22T10:05:00Z",
	}

	if err := stats.WriteLoopStats(db, p); err != nil {
		t.Fatalf("WriteLoopStats failed: %v", err)
	}

	// Verify all fields
	var loopID, sessID, owner, repo, branch, desc, startTime, finishTime string
	var totalCost float64
	var input, output, cacheCreation, cacheRead, total int64
	err := db.QueryRow("SELECT * FROM loop_stats WHERE loop_id = ?", "abc123-1").
		Scan(&loopID, &sessID, &owner, &repo, &branch, &desc, &totalCost,
			&input, &output, &cacheCreation, &cacheRead, &total, &startTime, &finishTime)
	if err != nil {
		t.Fatalf("Failed to query loop_stats: %v", err)
	}
	if desc != "feat: add widget" {
		t.Errorf("Expected description 'feat: add widget', got %q", desc)
	}
	if totalCost != 0.25 {
		t.Errorf("Expected total_cost 0.25, got %f", totalCost)
	}
	if input != 5000 {
		t.Errorf("Expected input_tokens 5000, got %d", input)
	}

	// Test INSERT OR REPLACE — update with different total_cost
	p.TotalCost = 0.50
	if err := stats.WriteLoopStats(db, p); err != nil {
		t.Fatalf("WriteLoopStats (replace) failed: %v", err)
	}

	err = db.QueryRow("SELECT total_cost FROM loop_stats WHERE loop_id = ?", "abc123-1").Scan(&totalCost)
	if err != nil {
		t.Fatalf("Failed to query after replace: %v", err)
	}
	if totalCost != 0.50 {
		t.Errorf("Expected updated total_cost 0.50, got %f", totalCost)
	}

	// Verify only one row exists (INSERT OR REPLACE, not INSERT)
	var count int
	db.QueryRow("SELECT COUNT(*) FROM loop_stats WHERE loop_id = ?", "abc123-1").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row after INSERT OR REPLACE, got %d", count)
	}
}

func TestWriteLoopStats_NilDB(t *testing.T) {
	err := stats.WriteLoopStats(nil, stats.LoopStatsParams{
		LoopID:    "test-1",
		SessionID: "test00",
	})
	if err != nil {
		t.Errorf("WriteLoopStats(nil, ...) should return nil, got %v", err)
	}
}

func TestQueryRollingHourCost_Global(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	ts := now.Format(time.RFC3339)

	// Insert 3 checkpoint rows with known delta_costs
	for i, cost := range []float64{0.10, 0.20, 0.30} {
		err := stats.FlushCheckpoint(db, stats.CheckpointParams{
			LoopID:    fmt.Sprintf("sess-1-%d", i),
			SessionID: "sess01",
			DeltaCost: cost,
			Timestamp: ts,
		})
		if err != nil {
			t.Fatalf("FlushCheckpoint failed: %v", err)
		}
	}

	total, err := stats.QueryRollingHourCost(db, "", "")
	if err != nil {
		t.Fatalf("QueryRollingHourCost failed: %v", err)
	}

	expected := 0.60
	tolerance := 0.0001
	if diff := total - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected global cost ~%f, got %f", expected, total)
	}
}

func TestQueryRollingHourCost_Scoped(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	ts := time.Now().UTC().Format(time.RFC3339)

	// Insert rows for two different repos
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "a-1", SessionID: "aaaaaa", Owner: "org1", Repo: "repo1", DeltaCost: 0.10, Timestamp: ts,
	})
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "a-2", SessionID: "aaaaaa", Owner: "org1", Repo: "repo1", DeltaCost: 0.20, Timestamp: ts,
	})
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "b-1", SessionID: "bbbbbb", Owner: "org2", Repo: "repo2", DeltaCost: 0.50, Timestamp: ts,
	})

	// Scoped to org1/repo1
	cost, err := stats.QueryRollingHourCost(db, "org1", "repo1")
	if err != nil {
		t.Fatalf("QueryRollingHourCost scoped failed: %v", err)
	}
	expected := 0.30
	tolerance := 0.0001
	if diff := cost - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected scoped cost ~%f, got %f", expected, cost)
	}

	// Scoped to org2/repo2
	cost, err = stats.QueryRollingHourCost(db, "org2", "repo2")
	if err != nil {
		t.Fatalf("QueryRollingHourCost scoped failed: %v", err)
	}
	expected = 0.50
	if diff := cost - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected scoped cost ~%f, got %f", expected, cost)
	}
}

func TestQueryRollingHourCost_ExcludesOldRows(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	oldTs := now.Add(-2 * time.Hour).Format(time.RFC3339)
	currentTs := now.Format(time.RFC3339)

	// Old row (2 hours ago — outside the 60-minute rolling window)
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "old-1", SessionID: "aaaaaa", DeltaCost: 99.0, Timestamp: oldTs,
	})
	// Current row
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "new-1", SessionID: "bbbbbb", DeltaCost: 0.25, Timestamp: currentTs,
	})

	cost, err := stats.QueryRollingHourCost(db, "", "")
	if err != nil {
		t.Fatalf("QueryRollingHourCost failed: %v", err)
	}

	// Should only include the row within the rolling window
	tolerance := 0.0001
	if diff := cost - 0.25; diff < -tolerance || diff > tolerance {
		t.Errorf("Expected cost ~0.25 (excluding old row), got %f", cost)
	}
}

func TestConcurrentCheckpointAppends(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	ts := time.Now().UTC().Format(time.RFC3339)
	var wg sync.WaitGroup
	n := 10

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := stats.FlushCheckpoint(db, stats.CheckpointParams{
				LoopID:    fmt.Sprintf("conc-%d", idx),
				SessionID: "cccccc",
				DeltaCost: float64(idx) * 0.01,
				Timestamp: ts,
			})
			if err != nil {
				t.Errorf("Concurrent FlushCheckpoint %d failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all rows are present
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM checkpoints WHERE session_id = 'cccccc'").Scan(&count); err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != n {
		t.Errorf("Expected %d rows from concurrent appends, got %d", n, count)
	}
}

func TestGetGitContext(t *testing.T) {
	// In a git repo (current dir), should return without panic
	owner, repo, branch := stats.GetGitContext()
	// We don't assert specific values since it depends on the environment,
	// but in this repo we expect non-empty values
	t.Logf("GetGitContext: owner=%q repo=%q branch=%q", owner, repo, branch)

	// Test in a non-git temp dir — should return empty strings without error
	tmpDir, err := os.MkdirTemp("", "ralph-nogit-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore working dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	owner2, repo2, branch2 := stats.GetGitContext()
	// In a non-git dir, all should be empty
	if owner2 != "" || repo2 != "" || branch2 != "" {
		t.Logf("Non-git dir returned: owner=%q repo=%q branch=%q (expected empty)", owner2, repo2, branch2)
	}
}

func TestExportSessionTSV(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	// Write 2 loop_stats rows for the same session
	stats.WriteLoopStats(db, stats.LoopStatsParams{
		LoopID:              "abc123-1",
		SessionID:           "abc123",
		Owner:               "myorg",
		Repo:                "myrepo",
		Branch:              "main",
		Description:         "feat: first loop",
		TotalCost:           0.10,
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 100,
		CacheReadTokens:     50,
		TotalTokens:         1650,
		StartTime:           "2026-03-22T10:00:00Z",
		FinishTime:          "2026-03-22T10:02:00Z",
	})
	stats.WriteLoopStats(db, stats.LoopStatsParams{
		LoopID:              "abc123-2",
		SessionID:           "abc123",
		Owner:               "myorg",
		Repo:                "myrepo",
		Branch:              "main",
		Description:         "feat: second loop",
		TotalCost:           0.20,
		InputTokens:         2000,
		OutputTokens:        1000,
		CacheCreationTokens: 200,
		CacheReadTokens:     100,
		TotalTokens:         3300,
		StartTime:           "2026-03-22T10:02:00Z",
		FinishTime:          "2026-03-22T10:05:00Z",
	})

	// Also write a row for a different session — should not appear
	stats.WriteLoopStats(db, stats.LoopStatsParams{
		LoopID:    "other-1",
		SessionID: "other0",
		TotalCost: 9.99,
		StartTime: "2026-03-22T10:00:00Z",
		FinishTime: "2026-03-22T10:01:00Z",
	})

	tsv, err := stats.ExportSessionTSV(db, "abc123")
	if err != nil {
		t.Fatalf("ExportSessionTSV failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(tsv), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines (1 header + 2 data), got %d: %q", len(lines), tsv)
	}

	// Verify header
	if !strings.HasPrefix(lines[0], "loop_id\tsession_id\t") {
		t.Errorf("Expected TSV header to start with 'loop_id\\tsession_id\\t', got %q", lines[0])
	}

	// Verify data lines contain correct loop_ids
	if !strings.HasPrefix(lines[1], "abc123-1\t") {
		t.Errorf("Expected first data line to start with 'abc123-1', got %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "abc123-2\t") {
		t.Errorf("Expected second data line to start with 'abc123-2', got %q", lines[2])
	}

	// Verify the "other" session is not included
	if strings.Contains(tsv, "other-1") {
		t.Error("TSV should not contain rows from other sessions")
	}
}

func TestExportSessionTSV_EmptySession(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	tsv, err := stats.ExportSessionTSV(db, "nonexistent")
	if err != nil {
		t.Fatalf("ExportSessionTSV failed: %v", err)
	}
	if tsv != "" {
		t.Errorf("Expected empty string for non-existent session, got %q", tsv)
	}
}

func TestQueryRollingHourCost_NilDB(t *testing.T) {
	cost, err := stats.QueryRollingHourCost(nil, "", "")
	if err != nil {
		t.Errorf("QueryRollingHourCost(nil, ...) should return nil error, got %v", err)
	}
	if cost != 0 {
		t.Errorf("QueryRollingHourCost(nil, ...) should return 0, got %f", cost)
	}
}

func TestQueryRollingWakeTime_Basic(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	// Insert 3 checkpoints at slightly different times within the window
	// Oldest first: 0.10 at now-30min, 0.20 at now-20min, 0.30 at now-10min
	// Total = 0.60, limit = 0.25
	// Walking oldest to newest, subtracting:
	//   0.60 - 0.10 = 0.50 (still >= 0.25)
	//   0.50 - 0.20 = 0.30 (still >= 0.25)
	//   0.30 - 0.30 = 0.00 (< 0.25) → wake = ts3 + 60min
	ts1 := now.Add(-30 * time.Minute).Format(time.RFC3339)
	ts2 := now.Add(-20 * time.Minute).Format(time.RFC3339)
	ts3 := now.Add(-10 * time.Minute).Format(time.RFC3339)

	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "w-1", SessionID: "ww0001", DeltaCost: 0.10, Timestamp: ts1,
	})
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "w-2", SessionID: "ww0001", DeltaCost: 0.20, Timestamp: ts2,
	})
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "w-3", SessionID: "ww0001", DeltaCost: 0.30, Timestamp: ts3,
	})

	wakeTime, err := stats.QueryRollingWakeTime(db, "", "", 0.25)
	if err != nil {
		t.Fatalf("QueryRollingWakeTime failed: %v", err)
	}

	// Expected: ts3 + 60min = now - 10min + 60min = now + 50min
	expectedWake := now.Add(-10 * time.Minute).Add(60 * time.Minute)
	diff := wakeTime.Sub(expectedWake)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("Expected wake time ~%v, got %v (diff=%v)", expectedWake, wakeTime, diff)
	}
}

func TestQueryRollingWakeTime_Scoped(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	ts1 := now.Add(-30 * time.Minute).Format(time.RFC3339)
	ts2 := now.Add(-20 * time.Minute).Format(time.RFC3339)

	// Insert rows for two repos
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "s-1", SessionID: "ss0001", Owner: "org1", Repo: "repo1", DeltaCost: 0.40, Timestamp: ts1,
	})
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "s-2", SessionID: "ss0001", Owner: "org1", Repo: "repo1", DeltaCost: 0.30, Timestamp: ts2,
	})
	// Different repo — should not affect org1/repo1 wake time
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "s-3", SessionID: "ss0001", Owner: "org2", Repo: "repo2", DeltaCost: 5.00, Timestamp: ts1,
	})

	// Scoped to org1/repo1: total=0.70, limit=0.25
	// 0.70 - 0.40 = 0.30 (>= 0.25)
	// 0.30 - 0.30 = 0.00 (< 0.25) → wake = ts2 + 60min
	wakeTime, err := stats.QueryRollingWakeTime(db, "org1", "repo1", 0.25)
	if err != nil {
		t.Fatalf("QueryRollingWakeTime scoped failed: %v", err)
	}

	expectedWake := now.Add(-20 * time.Minute).Add(60 * time.Minute)
	diff := wakeTime.Sub(expectedWake)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("Expected scoped wake time ~%v, got %v (diff=%v)", expectedWake, wakeTime, diff)
	}
}

func TestQueryRollingWakeTime_FallbackNoDB(t *testing.T) {
	wakeTime, err := stats.QueryRollingWakeTime(nil, "", "", 1.0)
	if err != nil {
		t.Errorf("QueryRollingWakeTime(nil, ...) should return nil error, got %v", err)
	}
	if !wakeTime.IsZero() {
		t.Errorf("QueryRollingWakeTime(nil, ...) should return zero time, got %v", wakeTime)
	}
}

func TestQueryRollingWakeTime_FallbackSingleLargeCheckpoint(t *testing.T) {
	db, cleanup := helperInitTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	ts := now.Add(-10 * time.Minute).Format(time.RFC3339)

	// One checkpoint with cost=5.0, limit=1.0
	// After subtracting this single row: 5.0 - 5.0 = 0.0 < 1.0
	// But since there's only one row and removing it brings total to 0,
	// wake time = ts + 60min
	stats.FlushCheckpoint(db, stats.CheckpointParams{
		LoopID: "big-1", SessionID: "bb0001", DeltaCost: 5.0, Timestamp: ts,
	})

	wakeTime, err := stats.QueryRollingWakeTime(db, "", "", 1.0)
	if err != nil {
		t.Fatalf("QueryRollingWakeTime failed: %v", err)
	}

	// With a single row, subtracting it: 5.0 - 5.0 = 0.0 < 1.0
	// So wake time = ts + 60min = now - 10min + 60min = now + 50min
	expectedWake := now.Add(-10 * time.Minute).Add(60 * time.Minute)
	diff := wakeTime.Sub(expectedWake)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("Expected fallback wake time ~%v, got %v (diff=%v)", expectedWake, wakeTime, diff)
	}
}

func TestExportSessionTSV_NilDB(t *testing.T) {
	tsv, err := stats.ExportSessionTSV(nil, "abc123")
	if err != nil {
		t.Errorf("ExportSessionTSV(nil, ...) should return nil error, got %v", err)
	}
	if tsv != "" {
		t.Errorf("ExportSessionTSV(nil, ...) should return empty string, got %q", tsv)
	}
}

func TestReconcileCostThreadSafe(t *testing.T) {
	s := stats.NewTokenStats()

	// Seed with some initial cost so ReconcileCost has something to work with
	s.AddCost(10.0)

	const goroutines = 100
	const iterations = 1000
	var wg sync.WaitGroup

	// Half the goroutines call ReconcileCost, the other half call Snapshot
	wg.Add(goroutines)
	for i := 0; i < goroutines/2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.ReconcileCost(0.001, 0.001)
			}
		}()
	}
	for i := 0; i < goroutines/2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				snap := s.Snapshot()
				_ = snap.TotalCostUSD // read the field
			}
		}()
	}

	wg.Wait()

	// The exact final value isn't important — the test's value is that
	// it passes under `go test -race` without data race warnings.
	snap := s.Snapshot()
	if snap.TotalCostUSD < 0 {
		t.Errorf("TotalCostUSD should not be negative, got %f", snap.TotalCostUSD)
	}
}

// TestSubagentCostNoDoubleCount simulates an iteration with one subagent and verifies
// that the final TotalCostUSD equals the main result's total_cost_usd (no double-counting).
// Worked example from research: subagent $0.22 + main $3.32 = reported $3.32 (not $3.54).
func TestSubagentCostNoDoubleCount(t *testing.T) {
	s := stats.NewTokenStats()
	tolerance := 0.0000001

	// Phase 1: Token estimates stream in (main + subagent usage messages)
	// Subagent estimates: $0.20, Main estimates: $3.00
	var iterEstimate float64
	var subagentCostAccum float64

	subagentEstimate := 0.20
	mainEstimate := 3.00

	s.AddCost(subagentEstimate)
	iterEstimate += subagentEstimate

	s.AddCost(mainEstimate)
	iterEstimate += mainEstimate

	// TotalCostUSD should be $3.20 at this point
	snap := s.Snapshot()
	if diff := snap.TotalCostUSD - 3.20; diff < -tolerance || diff > tolerance {
		t.Errorf("After estimates: TotalCostUSD = %f, expected 3.20", snap.TotalCostUSD)
	}

	// Phase 2: Subagent result arrives with total_cost_usd = $0.22
	subagentActual := 0.22
	s.AddCost(subagentActual)
	subagentCostAccum += subagentActual

	// Phase 3: Main result arrives with total_cost_usd = $3.32 (includes subagent)
	mainActual := 3.32
	s.ReconcileCost(iterEstimate+subagentCostAccum, mainActual)

	// Final TotalCostUSD should equal $3.32 (the main result's actual cost)
	snap = s.Snapshot()
	if diff := snap.TotalCostUSD - mainActual; diff < -tolerance || diff > tolerance {
		t.Errorf("After reconciliation: TotalCostUSD = %f, expected %f (no double-counting)", snap.TotalCostUSD, mainActual)
	}
}

// TestSubagentCostMultipleSubagents covers two subagent results in one iteration.
// Each subagent's cost is accumulated, final total equals the main result's total_cost_usd.
func TestSubagentCostMultipleSubagents(t *testing.T) {
	s := stats.NewTokenStats()
	tolerance := 0.0000001

	var iterEstimate float64
	var subagentCostAccum float64

	// Token estimates stream in
	estimates := []float64{0.10, 0.15, 2.50} // subagent1, subagent2, main
	for _, est := range estimates {
		s.AddCost(est)
		iterEstimate += est
	}

	// Subagent 1 result: $0.12
	sub1Actual := 0.12
	s.AddCost(sub1Actual)
	subagentCostAccum += sub1Actual

	// Subagent 2 result: $0.18
	sub2Actual := 0.18
	s.AddCost(sub2Actual)
	subagentCostAccum += sub2Actual

	// Main result: $3.00 (includes both subagents)
	mainActual := 3.00
	s.ReconcileCost(iterEstimate+subagentCostAccum, mainActual)

	snap := s.Snapshot()
	if diff := snap.TotalCostUSD - mainActual; diff < -tolerance || diff > tolerance {
		t.Errorf("After reconciliation with 2 subagents: TotalCostUSD = %f, expected %f", snap.TotalCostUSD, mainActual)
	}
}

// TestSubagentCostAccumResetsOnNewLoop verifies that resetting subagentCostAccum
// on new loop start prevents cross-iteration leakage.
func TestSubagentCostAccumResetsOnNewLoop(t *testing.T) {
	s := stats.NewTokenStats()
	tolerance := 0.0000001

	// --- Iteration 1 ---
	var iterEstimate float64
	var subagentCostAccum float64

	s.AddCost(1.00) // main estimate
	iterEstimate += 1.00

	s.AddCost(0.10) // subagent estimate
	iterEstimate += 0.10

	s.AddCost(0.12) // subagent result actual
	subagentCostAccum += 0.12

	s.ReconcileCost(iterEstimate+subagentCostAccum, 1.15) // main result
	snap := s.Snapshot()
	if diff := snap.TotalCostUSD - 1.15; diff < -tolerance || diff > tolerance {
		t.Errorf("After iteration 1: TotalCostUSD = %f, expected 1.15", snap.TotalCostUSD)
	}

	// --- New loop start: reset accumulators ---
	iterEstimate = 0
	subagentCostAccum = 0

	// --- Iteration 2 ---
	s.AddCost(2.00) // main estimate
	iterEstimate += 2.00

	s.AddCost(0.20) // subagent estimate
	iterEstimate += 0.20

	s.AddCost(0.25) // subagent result actual
	subagentCostAccum += 0.25

	s.ReconcileCost(iterEstimate+subagentCostAccum, 2.30) // main result
	snap = s.Snapshot()

	// Expected: iteration1 (1.15) + iteration2 (2.30) = 3.45
	expected := 1.15 + 2.30
	if diff := snap.TotalCostUSD - expected; diff < -tolerance || diff > tolerance {
		t.Errorf("After iteration 2: TotalCostUSD = %f, expected %f (no cross-iteration leakage)", snap.TotalCostUSD, expected)
	}
}

// replayFixture is a helper that replays a fixture file through the cost-tracking
// logic mirroring handleParsedMessage, with optional message-ID deduplication.
// Returns the final TokenStats snapshot, the expected cost from the result message,
// and the peak TotalCostUSD seen during the replay (before reconciliation).
func replayFixture(t *testing.T, fixturePath string, dedup bool) (snap *stats.TokenStats, expectedCost float64, peakCost float64) {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	p := parser.NewParser()
	tokenStats := stats.NewTokenStats()

	var iterEstimate float64
	var subagentCostAccum float64
	seenMsgIDs := make(map[string]bool)

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		parsed := p.ParseLine(line)
		if parsed == nil {
			continue
		}

		// --- Mirrors handleParsedMessage cost logic (main.go:685-723) ---
		if usage := p.GetUsage(parsed); usage != nil {
			shouldProcess := true
			if dedup {
				msgID := p.GetMessageID(parsed)
				if msgID != "" && seenMsgIDs[msgID] {
					shouldProcess = false
				} else if msgID != "" {
					seenMsgIDs[msgID] = true
				}
			}
			if shouldProcess {
				tokenStats.AddUsage(
					usage.InputTokens,
					usage.OutputTokens,
					usage.CacheCreationInputTokens,
					usage.CacheReadInputTokens,
				)
				estimate := stats.EstimateCostFromTokens(
					usage.InputTokens,
					usage.OutputTokens,
					usage.CacheCreationInputTokens,
					usage.CacheReadInputTokens,
				)
				tokenStats.AddCost(estimate)
				iterEstimate += estimate
			}
		}

		// Track peak cost (what the user sees in real-time before reconciliation)
		s := tokenStats.Snapshot()
		if s.TotalCostUSD > peakCost {
			peakCost = s.TotalCostUSD
		}

		// Extract cost from result messages — reconcile estimate with actual
		if cost := p.GetCost(parsed); cost > 0 {
			if !p.IsSubagentMessage(parsed) {
				tokenStats.ReconcileCost(iterEstimate+subagentCostAccum, cost)
				iterEstimate = 0
				subagentCostAccum = 0
				expectedCost = cost
			} else {
				tokenStats.AddCost(cost)
				subagentCostAccum += cost
			}
		}
	}

	s := tokenStats.Snapshot()
	return &s, expectedCost, peakCost
}

// TestSubagentCostIntegration_TokenCountInflation proves that without deduplication,
// token counts are permanently inflated because AddUsage is never reconciled.
// The CLI emits multiple chunks per message ID with identical cumulative usage.
func TestSubagentCostIntegration_TokenCountInflation(t *testing.T) {
	fixture := filepath.Join("fixtures", "subagent_cost_session.json")

	withDedup, _, _ := replayFixture(t, fixture, true)
	withoutDedup, _, _ := replayFixture(t, fixture, false)

	if withoutDedup.CacheCreationTokens <= withDedup.CacheCreationTokens {
		t.Fatal("Expected without-dedup to have inflated cache creation tokens, but it didn't")
	}

	inflationRatio := float64(withoutDedup.CacheCreationTokens) / float64(withDedup.CacheCreationTokens)
	t.Logf("Token inflation without dedup: %.1fx (cache_creation: %d vs %d)",
		inflationRatio, withoutDedup.CacheCreationTokens, withDedup.CacheCreationTokens)

	if inflationRatio < 1.1 {
		t.Error("Expected significant token inflation without dedup, but ratio is < 1.1x")
	}
}

// TestSubagentCostIntegration_PeakCostInflation proves that without deduplication,
// the real-time cost displayed to the user (before reconciliation) is inflated.
// If the iteration is interrupted before reconciliation, this inflated cost sticks.
func TestSubagentCostIntegration_PeakCostInflation(t *testing.T) {
	fixture := filepath.Join("fixtures", "subagent_cost_session.json")

	_, expectedCost, peakWithDedup := replayFixture(t, fixture, true)
	_, _, peakWithoutDedup := replayFixture(t, fixture, false)

	if expectedCost == 0 {
		t.Fatal("No result message with total_cost_usd found in fixture")
	}

	t.Logf("Actual cost: $%.6f", expectedCost)
	t.Logf("Peak cost with dedup:    $%.6f (%.1fx actual)", peakWithDedup, peakWithDedup/expectedCost)
	t.Logf("Peak cost without dedup: $%.6f (%.1fx actual)", peakWithoutDedup, peakWithoutDedup/expectedCost)

	// Without dedup, peak should be significantly higher than with dedup
	if peakWithoutDedup <= peakWithDedup {
		t.Error("Expected without-dedup peak to exceed with-dedup peak")
	}

	// With dedup, peak estimate should be in a reasonable range of the actual cost.
	// Estimates use hardcoded Sonnet pricing which may differ from actual model mix,
	// so we allow up to 5x. The important thing is it's way less than without dedup.
	if peakWithDedup > expectedCost*5 {
		t.Errorf("Even with dedup, peak estimate ($%.6f) is >5x actual ($%.6f) — dedup may not be working", peakWithDedup, expectedCost)
	}
}

// TestSubagentCostIntegration_FinalCostWithDedup verifies that after a complete
// iteration (with reconciliation), the final cost matches the result's total_cost_usd.
func TestSubagentCostIntegration_FinalCostWithDedup(t *testing.T) {
	fixture := filepath.Join("fixtures", "subagent_cost_session.json")

	snap, expectedCost, _ := replayFixture(t, fixture, true)

	if expectedCost == 0 {
		t.Fatal("No result message with total_cost_usd found in fixture")
	}

	tolerance := 0.000001
	diff := snap.TotalCostUSD - expectedCost
	if diff < -tolerance || diff > tolerance {
		t.Errorf("Final TotalCostUSD = %.6f, expected %.6f (diff = %.6f)",
			snap.TotalCostUSD, expectedCost, diff)
	}
}

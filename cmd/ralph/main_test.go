package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudosai/ralph-go/internal/config"
)

func TestParseTaskCountsNoFile(t *testing.T) {
	completed, total := parseTaskCounts("/nonexistent/path/IMPLEMENTATION_PLAN.md")
	if completed != 0 || total != 0 {
		t.Errorf("expected (0, 0) for missing file, got (%d, %d)", completed, total)
	}
}

func TestParseTaskCountsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	os.WriteFile(path, []byte(""), 0644)

	completed, total := parseTaskCounts(path)
	if completed != 0 || total != 0 {
		t.Errorf("expected (0, 0) for empty file, got (%d, %d)", completed, total)
	}
}

func TestParseTaskCountsSingleTaskTodo(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: Add feature X
**Priority: HIGH**
**Status: TODO**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if completed != 0 {
		t.Errorf("expected completed=0, got %d", completed)
	}
}

func TestParseTaskCountsSingleTaskDone(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: Add feature X
**Priority: HIGH**
**Status: DONE**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed=1, got %d", completed)
	}
}

func TestParseTaskCountsNotNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: Deprecated feature
**Status: NOT NEEDED**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed=1, got %d", completed)
	}
}

func TestParseTaskCountsMultipleTasks(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: First feature
**Status: DONE**

## TASK 2: Second feature
**Status: IN PROGRESS**

## TASK 3: Third feature
**Status: TODO**

## TASK 4: Fourth feature
**Status: DONE**

## TASK 5: Fifth feature
**Status: NOT NEEDED**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if completed != 3 {
		t.Errorf("expected completed=3 (2 DONE + 1 NOT NEEDED), got %d", completed)
	}
}

func TestParseTaskCountsNoTaskHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

Some general notes about the project.

**Status: DONE**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 0 {
		t.Errorf("expected total=0 (no ## TASK headers), got %d", total)
	}
	// Status line without a TASK header still counts as completed
	if completed != 1 {
		t.Errorf("expected completed=1 (status line exists), got %d", completed)
	}
}

func TestParseTaskCountsStatusOnSameLineAsHeader(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `## TASK 1: Feature — **Status: DONE**
## TASK 2: Other feature
**Status: TODO**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed=1, got %d", completed)
	}
}

func TestParseTaskCountsReflectsFileChanges(t *testing.T) {
	// Verifies that repeated calls to parseTaskCounts pick up file modifications,
	// which is the basis for live recount at iteration boundaries.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")

	// Initial: 1 done out of 3
	initial := "## TASK 1\n**Status: DONE**\n## TASK 2\n**Status: TODO**\n## TASK 3\n**Status: TODO**\n"
	os.WriteFile(path, []byte(initial), 0644)

	completed, total := parseTaskCounts(path)
	if completed != 1 || total != 3 {
		t.Errorf("initial: expected 1/3, got %d/%d", completed, total)
	}

	// Simulate Claude marking task 2 as DONE during an iteration
	updated := "## TASK 1\n**Status: DONE**\n## TASK 2\n**Status: DONE**\n## TASK 3\n**Status: TODO**\n"
	os.WriteFile(path, []byte(updated), 0644)

	completed, total = parseTaskCounts(path)
	if completed != 2 || total != 3 {
		t.Errorf("after update: expected 2/3, got %d/%d", completed, total)
	}

	// Simulate adding a new task during the session
	expanded := updated + "## TASK 4\n**Status: TODO**\n"
	os.WriteFile(path, []byte(expanded), 0644)

	completed, total = parseTaskCounts(path)
	if completed != 2 || total != 4 {
		t.Errorf("after expansion: expected 2/4, got %d/%d", completed, total)
	}
}

func TestCheckCostPacingDisabled(t *testing.T) {
	// maxCostPerHour=0 means disabled — should be a no-op
	exceeded, hourCost, nextHour := checkCostPacing(&dbContext{}, 0, nil)
	if exceeded {
		t.Error("expected exceeded=false when maxCostPerHour=0")
	}
	if hourCost != 0 {
		t.Errorf("expected hourCost=0, got %f", hourCost)
	}
	if !nextHour.IsZero() {
		t.Errorf("expected zero nextHour, got %v", nextHour)
	}

	// Negative value also means disabled
	exceeded, _, _ = checkCostPacing(&dbContext{}, -1.0, nil)
	if exceeded {
		t.Error("expected exceeded=false when maxCostPerHour<0")
	}
}

func TestCheckCostPacingNilDB(t *testing.T) {
	// dbCtx with nil db — should be a no-op
	exceeded, hourCost, nextHour := checkCostPacing(&dbContext{db: nil}, 1.0, nil)
	if exceeded {
		t.Error("expected exceeded=false when db is nil")
	}
	if hourCost != 0 {
		t.Errorf("expected hourCost=0, got %f", hourCost)
	}
	if !nextHour.IsZero() {
		t.Errorf("expected zero nextHour, got %v", nextHour)
	}

	// Nil dbCtx entirely
	exceeded, _, _ = checkCostPacing(nil, 1.0, nil)
	if exceeded {
		t.Error("expected exceeded=false when dbCtx is nil")
	}
}

func TestMaxCostPerHourFlag(t *testing.T) {
	// Save and restore global state
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	// Reset flag state so ParseFlags can register new flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"ralph", "--max-cost-per-hour=1.50", "--show-prompt"}

	cfg := config.ParseFlags()
	if cfg.MaxCostPerHour != 1.50 {
		t.Errorf("expected MaxCostPerHour=1.50, got %f", cfg.MaxCostPerHour)
	}

	// Test default (0 = no limit)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"ralph", "--show-prompt"}

	cfg = config.ParseFlags()
	if cfg.MaxCostPerHour != 0 {
		t.Errorf("expected MaxCostPerHour=0 (default), got %f", cfg.MaxCostPerHour)
	}
}

func TestHandleLoopMarkerReturnsTrue(t *testing.T) {
	// Verify the isLoopStart detection logic matches what handleLoopMarker uses
	tests := []struct {
		content  string
		expected bool
	}{
		{"======= LOOP 1/5 =======", true},
		{"======= LOOP 2/5 =======", true},
		{"======= STOPPED =======", false},
		{"======= COMPLETED 5 ITERATIONS =======", false},
		{"======= RESUMED =======", false},
	}

	for _, tt := range tests {
		isLoopStart := isNewLoopStart(tt.content)
		if isLoopStart != tt.expected {
			t.Errorf("isNewLoopStart(%q) = %v, want %v", tt.content, isLoopStart, tt.expected)
		}
	}
}

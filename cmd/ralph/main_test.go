package main

import (
	"os"
	"path/filepath"
	"testing"
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

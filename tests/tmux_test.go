package tests

import (
	"os"
	"os/exec"
	"testing"

	"github.com/cloudosai/ralph-go/internal/tmux"
)

func TestIsInsideTmux_WhenTMUXSet(t *testing.T) {
	// Save and restore TMUX env var
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Setenv("TMUX", "/tmp/tmux-501/default,12345,0")
	if !tmux.IsInsideTmux() {
		t.Error("Expected IsInsideTmux() to return true when TMUX is set")
	}
}

func TestIsInsideTmux_WhenTMUXUnset(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Unsetenv("TMUX")
	if tmux.IsInsideTmux() {
		t.Error("Expected IsInsideTmux() to return false when TMUX is unset")
	}
}

func TestIsInsideTmux_WhenTMUXEmpty(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Setenv("TMUX", "")
	if tmux.IsInsideTmux() {
		t.Error("Expected IsInsideTmux() to return false when TMUX is empty")
	}
}

func TestFindBinary_ReturnsPath(t *testing.T) {
	// tmux should be installed on this system
	path := tmux.FindBinary()
	if path == "" {
		t.Skip("tmux not found in PATH, skipping")
	}

	// Verify the returned path is actually a tmux binary
	cmd := exec.Command(path, "-V")
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("FindBinary() returned %q but it's not executable: %v", path, err)
	}
	if len(output) == 0 {
		t.Error("FindBinary() returned a path that produces no version output")
	}
}

func TestShouldWrap_NoTmuxFlag(t *testing.T) {
	// When --no-tmux is set, should never wrap
	if tmux.ShouldWrap(true) {
		t.Error("Expected ShouldWrap(true) to return false")
	}
}

func TestShouldWrap_AlreadyInTmux(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Setenv("TMUX", "/tmp/tmux-501/default,12345,0")
	if tmux.ShouldWrap(false) {
		t.Error("Expected ShouldWrap() to return false when already inside tmux")
	}
}

func TestShouldWrap_NotInTmux(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Unsetenv("TMUX")

	// This should return true if tmux is available
	if tmux.FindBinary() == "" {
		t.Skip("tmux not found in PATH, skipping")
	}

	if !tmux.ShouldWrap(false) {
		t.Error("Expected ShouldWrap(false) to return true when not in tmux and tmux is available")
	}
}

func TestShouldWrap_TmuxNotAvailable(t *testing.T) {
	orig := os.Getenv("TMUX")
	origPath := os.Getenv("PATH")
	defer func() {
		os.Setenv("TMUX", orig)
		os.Setenv("PATH", origPath)
	}()

	os.Unsetenv("TMUX")
	// Set PATH to empty so tmux can't be found
	os.Setenv("PATH", "/nonexistent")

	if tmux.ShouldWrap(false) {
		t.Error("Expected ShouldWrap() to return false when tmux is not available")
	}
}

// TestNewStatusBar_NotInTmux tests that StatusBar is inactive when not inside tmux
func TestNewStatusBar_NotInTmux(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Unsetenv("TMUX")
	sb := tmux.NewStatusBar()
	if sb.IsActive() {
		t.Error("StatusBar should be inactive when not inside tmux")
	}
}

// TestStatusBarUpdate_Inactive tests that Update is a no-op when inactive
func TestStatusBarUpdate_Inactive(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Unsetenv("TMUX")
	sb := tmux.NewStatusBar()
	// Should not panic
	sb.Update("test content")
}

// TestStatusBarRestore_Inactive tests that Restore is a no-op when inactive
func TestStatusBarRestore_Inactive(t *testing.T) {
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Unsetenv("TMUX")
	sb := tmux.NewStatusBar()
	// Should not panic
	sb.Restore()
}

// TestStatusBarNilSafe tests that nil StatusBar methods don't panic
func TestStatusBarNilSafe(t *testing.T) {
	var sb *tmux.StatusBar
	if sb.IsActive() {
		t.Error("nil StatusBar should not be active")
	}
	// These should not panic
	sb.Update("test")
	sb.Restore()
}

// TestFormatStatusRight tests the status bar format string
func TestFormatStatusRight(t *testing.T) {
	result := tmux.FormatStatusRight("#1/5", "128.58m", "07:18:00")
	expected := "[current loop: #1/5   tokens: 128.58m   elapsed: 07:18:00]"
	if result != expected {
		t.Errorf("FormatStatusRight() = %q, want %q", result, expected)
	}
}

// TestFormatStatusRight_ZeroValues tests formatting with zero/default values
func TestFormatStatusRight_ZeroValues(t *testing.T) {
	result := tmux.FormatStatusRight("#0/0", "0", "00:00:00")
	if result == "" {
		t.Error("FormatStatusRight should return non-empty string for zero values")
	}
	expected := "[current loop: #0/0   tokens: 0   elapsed: 00:00:00]"
	if result != expected {
		t.Errorf("FormatStatusRight() = %q, want %q", result, expected)
	}
}

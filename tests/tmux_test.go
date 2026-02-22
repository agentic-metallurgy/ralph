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

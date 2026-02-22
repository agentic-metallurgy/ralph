package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// StatusBar manages the tmux status-right bar for ralph.
// When active (inside tmux), it sets the status-right to show loop/token/elapsed info.
// On Restore, it unsets the session-level override so tmux falls back to the global default.
type StatusBar struct {
	active bool
}

// NewStatusBar creates a StatusBar. If not inside tmux, it returns an inactive no-op bar.
func NewStatusBar() *StatusBar {
	if !IsInsideTmux() {
		return &StatusBar{active: false}
	}
	sb := &StatusBar{active: true}
	// Override the tmux status bar completely: clear the left side and extend
	// status-right to full width so our content takes over the entire bar.
	if path := FindBinary(); path != "" {
		exec.Command(path, "set-option", "status-left", "").Run()
		exec.Command(path, "set-option", "status-left-length", "0").Run()
		exec.Command(path, "set-option", "status-right-length", "200").Run()
	}
	return sb
}

// IsActive returns whether the status bar is managing tmux.
func (s *StatusBar) IsActive() bool {
	return s != nil && s.active
}

// Update sets the tmux status-right to the given content string.
func (s *StatusBar) Update(content string) {
	if s == nil || !s.active {
		return
	}
	path := FindBinary()
	if path == "" {
		return
	}
	exec.Command(path, "set-option", "status-right", content).Run()
}

// Restore unsets the session-level status-right override so tmux falls back to the global default.
func (s *StatusBar) Restore() {
	if s == nil || !s.active {
		return
	}
	path := FindBinary()
	if path == "" {
		return
	}
	exec.Command(path, "set-option", "-u", "status-right").Run()
	exec.Command(path, "set-option", "-u", "status-right-length").Run()
	exec.Command(path, "set-option", "-u", "status-left").Run()
	exec.Command(path, "set-option", "-u", "status-left-length").Run()
}

// FormatStatusRight builds the tmux status bar content string.
func FormatStatusRight(loopDisplay, tokenDisplay, timeDisplay string) string {
	return fmt.Sprintf("[current loop: %s   tokens: %s   elapsed: %s]",
		loopDisplay, tokenDisplay, timeDisplay)
}

// GetCurrentSessionName returns the current tmux session name, or empty if not in tmux.
func GetCurrentSessionName() string {
	if !IsInsideTmux() {
		return ""
	}
	path := FindBinary()
	if path == "" {
		return ""
	}
	out, err := exec.Command(path, "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// IsInsideTmux returns true if the current process is running inside a tmux session.
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// FindBinary returns the path to the tmux binary, or empty string if not found.
func FindBinary() string {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return ""
	}
	return path
}

// ShouldWrap returns true if the process should be wrapped in a tmux session.
func ShouldWrap(noTmux bool) bool {
	if noTmux {
		return false
	}
	if IsInsideTmux() {
		return false
	}
	if FindBinary() == "" {
		return false
	}
	return true
}

// pickSessionName returns a unique tmux session name starting with "ralph".
func pickSessionName(tmuxPath string) string {
	base := "ralph"
	// Check if session already exists
	if err := exec.Command(tmuxPath, "has-session", "-t", base).Run(); err != nil {
		// Session doesn't exist, use the base name
		return base
	}
	// Session exists, find a unique name
	for i := 1; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if err := exec.Command(tmuxPath, "has-session", "-t", candidate).Run(); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", base, os.Getpid())
}

// Wrap re-execs the current process inside a new tmux session.
// It replaces the current process via syscall.Exec, so this function
// does not return on success.
func Wrap() error {
	tmuxPath := FindBinary()
	if tmuxPath == "" {
		return fmt.Errorf("tmux not found in PATH")
	}

	ralphBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	sessionName := pickSessionName(tmuxPath)

	// Reconstruct args with --no-tmux added to prevent recursive wrapping
	ralphArgs := make([]string, 0, len(os.Args))
	ralphArgs = append(ralphArgs, os.Args[1:]...)
	ralphArgs = append(ralphArgs, "--no-tmux")

	// Build: tmux new-session -s <name> -- <ralph> [args...]
	args := []string{"tmux", "new-session", "-s", sessionName, "--"}
	args = append(args, ralphBin)
	args = append(args, ralphArgs...)

	return syscall.Exec(tmuxPath, args, os.Environ())
}

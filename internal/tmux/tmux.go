package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

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

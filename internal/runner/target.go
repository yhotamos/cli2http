package runner

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// Target identifies the CLI selected at startup.
// Executable is an absolute path, so later execution need not search PATH again.
type Target struct {
	Command    string
	Executable string
}

// Resolve finds a command through PATH or an explicitly supplied path.
// It preserves exec.LookPath errors, including exec.ErrDot.
func Resolve(command string) (Target, error) {
	executable, err := exec.LookPath(command)
	if err != nil {
		return Target{}, fmt.Errorf("resolve command %q: %w", command, err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return Target{}, fmt.Errorf("resolve executable path: %w", err)
	}
	return Target{Command: command, Executable: executable}, nil
}

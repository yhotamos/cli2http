package runner

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var shellNames = []string{
	"cmd",
	"powershell",
	"pwsh",
	"sh",
	"bash",
	"dash",
	"ash",
	"zsh",
	"ksh",
	"csh",
	"tcsh",
	"fish",
	"nu",
}

// Target identifies the CLI selected at startup.
// Executable is an absolute path, so later execution need not search PATH again.
type Target struct {
	Command    string
	Executable string
}

// Resolve finds a command through PATH or an explicitly supplied path and rejects shells.
// It preserves exec.LookPath errors, including exec.ErrDot.
func Resolve(command string) (Target, error) {
	if err := rejectShell(command); err != nil {
		return Target{}, err
	}
	executable, err := exec.LookPath(command)
	if err != nil {
		return Target{}, fmt.Errorf("resolve command %q: %w", command, err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return Target{}, fmt.Errorf("resolve executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return Target{}, fmt.Errorf("resolve executable symlinks: %w", err)
	}
	if err := rejectShell(resolved); err != nil {
		return Target{}, err
	}
	return Target{Command: command, Executable: resolved}, nil
}

func rejectShell(command string) error {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(command)), ".exe")
	if slices.Contains(shellNames, name) {
		return fmt.Errorf("shell command %q is not supported", command)
	}
	return nil
}

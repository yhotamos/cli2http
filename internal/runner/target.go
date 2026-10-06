package runner

import (
	"fmt"
	"os"
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

// Resolve finds a command through PATH or an explicitly supplied path and rejects shells and cli2http itself.
// It preserves exec.LookPath errors, including exec.ErrDot.
func Resolve(command string) (Target, error) {
	if err := rejectCommandName(command); err != nil {
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
	if err := rejectCommandName(resolved); err != nil {
		return Target{}, err
	}
	self, err := os.Executable()
	if err != nil {
		return Target{}, fmt.Errorf("resolve current executable: %w", err)
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		return Target{}, fmt.Errorf("stat current executable: %w", err)
	}
	targetInfo, err := os.Stat(executable)
	if err != nil {
		return Target{}, fmt.Errorf("stat target executable: %w", err)
	}
	if os.SameFile(selfInfo, targetInfo) {
		return Target{}, fmt.Errorf("cli2http command %q is not supported", command)
	}
	return Target{Command: command, Executable: executable}, nil
}

func rejectCommandName(command string) error {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(command)), ".exe")
	if slices.Contains(shellNames, name) {
		return fmt.Errorf("shell command %q is not supported", command)
	}
	if name == "cli2http" {
		return fmt.Errorf("cli2http command %q is not supported", command)
	}
	return nil
}

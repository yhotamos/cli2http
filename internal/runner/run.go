package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Result contains the completed CLI process output.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Run executes the resolved target directly, without invoking a shell.
// A nonzero exit code is a CLI result, not a failure to run the process.
func (t Target) Run(ctx context.Context, args []string) (Result, error) {
	cmd := exec.CommandContext(ctx, t.Executable, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return Result{}, fmt.Errorf("execute command %q: %w", t.Command, err)
		}
		result.ExitCode = exitErr.ExitCode()
	}
	return result, nil
}

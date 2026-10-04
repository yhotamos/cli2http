package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
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
	executionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(executionCtx, t.Executable, args...)
	cmd.WaitDelay = time.Second
	stdout := outputBuffer{cancel: cancel}
	stderr := outputBuffer{cancel: cancel}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := runProcess(cmd)
	if stdout.exceeded || stderr.exceeded {
		return Result{}, ErrOutputLimit
	}
	result := Result{Stdout: stdout.buffer.String(), Stderr: stderr.buffer.String()}
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

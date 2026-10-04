package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The test binary also acts as a portable CLI fixture, without a shell dependency.
func TestRunProcess(t *testing.T) {
	if os.Getenv("CLI2HTTP_RUNNER_PROCESS") != "1" {
		return
	}
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) > 0 && args[0] == "sleep" {
		time.Sleep(30 * time.Second)
	}
	fmt.Fprint(os.Stdout, strings.Join(args, "|"))
	fmt.Fprint(os.Stderr, "stderr")
	if len(args) > 0 && args[0] == "fail" {
		os.Exit(7)
	}
	os.Exit(0)
}

func TestRun(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLI2HTTP_RUNNER_PROCESS", "1")
	target := Target{Command: "fixture", Executable: executable}
	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{name: "no arguments"},
		{name: "literal arguments", args: []string{"a b", "&&", "|", ">", ";", "$HOME"}},
		{name: "nonzero exit", args: []string{"fail"}, code: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestRunProcess$", "--"}, test.args...)
			result, err := target.Run(context.Background(), args)
			if err != nil {
				t.Fatal(err)
			}
			if result.ExitCode != test.code || result.Stdout != strings.Join(test.args, "|") || result.Stderr != "stderr" {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := target.Run(ctx, []string{"-test.run=^TestRunProcess$", "--", "sleep"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestRunStartFailure(t *testing.T) {
	target := Target{Command: "missing", Executable: "cli2http-missing-command-5c3497"}
	if _, err := target.Run(context.Background(), nil); err == nil {
		t.Fatal("expected a process startup error")
	}
}

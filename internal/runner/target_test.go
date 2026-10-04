package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(executable))
	for _, command := range []string{filepath.Base(executable), executable} {
		t.Run(command, func(t *testing.T) {
			target, err := Resolve(command)
			if err != nil {
				t.Fatal(err)
			}
			if target.Command != command || target.Executable != executable {
				t.Fatalf("unexpected target: %+v", target)
			}
		})
	}
}

func TestResolveMissingCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	target, err := Resolve("cli2http-missing-command-5c3497")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("error = %v, want exec.ErrNotFound", err)
	}
	if target != (Target{}) {
		t.Fatalf("unexpected target on error: %+v", target)
	}
}

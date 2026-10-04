package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestResolveRejectsShell(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for _, name := range []string{"cmd", "powershell", "pwsh", "sh", "bash", "dash", "ash", "zsh", "ksh", "csh", "tcsh", "fish", "nu"} {
		for _, command := range []string{name, name + ".exe", strings.ToUpper(name), filepath.Join(dir, strings.ToUpper(name)+".EXE")} {
			t.Run(command, func(t *testing.T) {
				target, err := Resolve(command)
				if err == nil || !strings.Contains(err.Error(), "shell command") {
					t.Fatalf("error = %v, want shell rejection", err)
				}
				if target != (Target{}) {
					t.Fatalf("unexpected target on error: %+v", target)
				}
			})
		}
	}
}

func TestResolveSymlink(t *testing.T) {
	for _, name := range []string{"sh.exe", "tool.exe"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			executable := filepath.Join(dir, name)
			if err := os.WriteFile(executable, []byte("fixture"), 0755); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "alias.exe")
			if err := os.Symlink(executable, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			t.Setenv("PATH", dir)
			for _, command := range []string{filepath.Base(link), link} {
				target, err := Resolve(command)
				if name == "sh.exe" {
					if err == nil || !strings.Contains(err.Error(), "shell command") || target != (Target{}) {
						t.Fatalf("target = %+v, error = %v, want shell rejection", target, err)
					}
					continue
				}
				if err != nil || target.Command != command || target.Executable != link {
					t.Fatalf("target = %+v, error = %v, want %q", target, err, link)
				}
			}
		})
	}
}

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
	executable := filepath.Join(t.TempDir(), "tool.exe")
	if err := os.WriteFile(executable, []byte("fixture"), 0755); err != nil {
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
	for _, name := range []string{"sh.exe", "cli2http.exe", "tool.exe"} {
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
				if name != "tool.exe" {
					if err == nil || !strings.Contains(err.Error(), "is not supported") || target != (Target{}) {
						t.Fatalf("target = %+v, error = %v, want command rejection", target, err)
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

func TestResolveRejectsCLI2HTTP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for _, command := range []string{"cli2http", "cli2http.exe", "CLI2HTTP", filepath.Join(dir, "CLI2HTTP.EXE")} {
		t.Run(command, func(t *testing.T) {
			target, err := Resolve(command)
			if err == nil || !strings.Contains(err.Error(), "cli2http command") || target != (Target{}) {
				t.Fatalf("target = %+v, error = %v, want cli2http rejection", target, err)
			}
		})
	}
}

func TestResolveRejectsSelf(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(executable))
	for _, command := range []string{filepath.Base(executable), executable} {
		target, err := Resolve(command)
		if err == nil || !strings.Contains(err.Error(), "cli2http command") || target != (Target{}) {
			t.Fatalf("target = %+v, error = %v, want self rejection", target, err)
		}
	}
	t.Run("symlink", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "alias.exe")
		if err := os.Symlink(executable, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		target, err := Resolve(link)
		if err == nil || !strings.Contains(err.Error(), "cli2http command") || target != (Target{}) {
			t.Fatalf("target = %+v, error = %v, want self rejection", target, err)
		}
	})
}

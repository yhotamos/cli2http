package cmd

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestStartupOutput(t *testing.T) {
	executable := writeTargetFixture(t)
	t.Setenv("PATH", filepath.Dir(executable))
	name := filepath.Base(executable)
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(reservation.Addr().(*net.TCPAddr).Port)
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		args     []string
		wantPort string
	}{
		{"default", []string{name}, `[1-9][0-9]*`},
		{"automatic", []string{"--port", "0", name}, `[1-9][0-9]*`},
		{"specified", []string{"--port", port, name}, port},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs(test.args)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := command.ExecuteContext(ctx); err != nil {
				t.Fatal(err)
			}
			want := `^Command : ` + regexp.QuoteMeta(name) + `\nURL     : http://127\.0\.0\.1:` + test.wantPort + `\nToken   : [0-9a-f]{64}\n$`
			if !regexp.MustCompile(want).MatchString(output.String()) {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func TestInvalidCommand(t *testing.T) {
	for _, args := range [][]string{nil, {"one", "two"}, {"cli2http-missing-command-5c3497"}} {
		t.Run(strings.Join(args, ","), func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(append([]string{}, args...))
			if err := command.Execute(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestRejectedCommandBeforeStartup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	for _, test := range []struct {
		command string
		message string
	}{
		{"pwsh", "shell command"},
		{"cli2http", "cli2http command"},
		{executable, "cli2http command"},
	} {
		t.Run(test.command, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs([]string{test.command})
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want %s rejection", err, test.message)
			}
			if output.Len() != 0 {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func TestInvalidPort(t *testing.T) {
	executable := writeTargetFixture(t)
	for _, port := range []string{"-1", "65536", "not-a-number"} {
		t.Run(port, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs([]string{"--port", port, executable})
			if err := command.Execute(); err == nil {
				t.Fatal("expected an error for an invalid port")
			}
			if output.Len() != 0 {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func writeTargetFixture(t *testing.T) string {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "tool.exe")
	if err := os.WriteFile(executable, []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	return executable
}

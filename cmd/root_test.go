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
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
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
			want := `^Command : ` + regexp.QuoteMeta(name) + `\nAddress : http://127\.0\.0\.1:` + test.wantPort + `\nToken   : [0-9a-f]{64}\n\nPOST /exec\n$`
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

func TestShellRejectedBeforeStartup(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"pwsh"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "shell command") {
		t.Fatalf("error = %v, want shell rejection", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected startup output: %q", output.String())
	}
}

func TestInvalidPort(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
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

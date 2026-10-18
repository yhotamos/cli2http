package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStartupOutput(t *testing.T) {
	executable := writeTargetFixture(t)
	t.Setenv("PATH", filepath.Dir(executable))
	name := filepath.Base(executable)
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const fixedToken = "Abcdef01234567-_"
	port := strconv.Itoa(reservation.Addr().(*net.TCPAddr).Port)
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		args      []string
		wantPort  string
		wantToken string
	}{
		{"custom path", []string{"--path", "/resize", name}, `[1-9][0-9]*`, `[0-9a-f]{64}`},
		{"default", []string{name}, `[1-9][0-9]*`, `[0-9a-f]{64}`},
		{"automatic", []string{"--port", "0", name}, `[1-9][0-9]*`, `[0-9a-f]{64}`},
		{"specified", []string{"--port", port, name}, port, `[0-9a-f]{64}`},
		{"fixed token", []string{"--token", fixedToken, name}, `[1-9][0-9]*`, regexp.QuoteMeta(fixedToken)},
		{"fixed token equals", []string{"--token=" + fixedToken, name}, `[1-9][0-9]*`, regexp.QuoteMeta(fixedToken)},
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
			path := "/exec"
			if test.name == "custom path" {
				path = "/resize"
			}
			want := `^URL     : http://127\.0\.0\.1:` + test.wantPort + `\nToken   : ` + test.wantToken + `\n` + path + ` : ` + regexp.QuoteMeta(name) + `\n$`
			if !regexp.MustCompile(want).MatchString(output.String()) {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func TestInvalidCommand(t *testing.T) {
	for _, args := range [][]string{nil, {"--"}, {"cli2http-missing-command-5c3497"}} {
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

func TestInvalidToken(t *testing.T) {
	executable := writeTargetFixture(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{"missing", []string{"--token"}},
		{"empty", []string{"--token=", executable}},
		{"short", []string{"--token", strings.Repeat("a", 15), executable}},
		{"invalid character", []string{"--token", strings.Repeat("a", 15) + "!", executable}},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(test.args)
			if err := command.Execute(); err == nil {
				t.Fatal("expected an error for an invalid token")
			}
			if output.Len() != 0 {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func TestInvalidPath(t *testing.T) {
	executable := writeTargetFixture(t)
	for _, path := range []string{"", "resize", "/info", "/health", "/resize?size=800", "/{name}", "/resize/", "/a/../resize", "/resize\n"} {
		t.Run(path, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs([]string{"--path", path, executable})
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "invalid path") {
				t.Fatalf("error = %v, want invalid path", err)
			}
			if output.Len() != 0 {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func TestStartupOptions(t *testing.T) {
	for _, test := range []struct {
		name             string
		input, remaining []string
		timeout          time.Duration
		port             int
	}{
		{"default", []string{"tool"}, []string{"tool"}, 0, 0},
		{"zero", []string{"--timeout", "0", "tool"}, []string{"tool"}, 0, 0},
		{"seconds", []string{"--timeout", "30s", "tool fixed"}, []string{"tool fixed"}, 30 * time.Second, 0},
		{"minutes", []string{"--timeout=2m", "tool"}, []string{"tool"}, 2 * time.Minute, 0},
		{"path", []string{"--port", "8080", "--path", "/a", "tool"}, []string{"--path", "/a", "tool"}, 0, 8080},
		{"separator", []string{"--port=8080", "--", "tool"}, []string{"tool"}, 0, 8080},
		{"options after command", []string{"tool", "--timeout", "30s"}, []string{"tool", "--timeout", "30s"}, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := newRootCommand()
			args, err := parseOptions(command, test.input)
			if err != nil {
				t.Fatal(err)
			}
			timeout, _ := command.Flags().GetDuration("timeout")
			port, _ := command.Flags().GetInt("port")
			if timeout != test.timeout || port != test.port || !slices.Equal(args, test.remaining) {
				t.Fatalf("args=%q timeout=%s port=%d", args, timeout, port)
			}
		})
	}
}

func TestInvalidTimeout(t *testing.T) {
	executable := writeTargetFixture(t)
	for _, args := range [][]string{
		{"--timeout", "-1s", executable},
		{"--timeout", "30", executable},
		{"--timeout", "invalid", executable},
		{"--timeout"},
	} {
		t.Run(strings.Join(args, ","), func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatal("expected invalid timeout error")
			}
			if output.Len() != 0 {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

func TestPublicationArguments(t *testing.T) {
	executable := writeTargetFixture(t)
	commandLine := `"` + executable + `" "" "a b" --flag`
	for _, test := range []struct {
		name  string
		args  []string
		paths []string
	}{
		{"default", []string{commandLine}, []string{"/exec"}},
		{"custom", []string{"--path", "/a", commandLine}, []string{"/a"}},
		{"equals", []string{"--path=/a", commandLine}, []string{"/a"}},
		{"multiple", []string{"--path", "/b", commandLine, "--path", "/a", commandLine}, []string{"/b", "/a"}},
		{"default and custom", []string{commandLine, "--path", "/a", commandLine}, []string{"/exec", "/a"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			endpoints, err := parseEndpoints(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if len(endpoints) != len(test.paths) {
				t.Fatalf("endpoints=%+v", endpoints)
			}
			for i, endpoint := range endpoints {
				if endpoint.Path != test.paths[i] || !slices.Equal(endpoint.Target.FixedArgs, []string{"", "a b", "--flag"}) {
					t.Fatalf("endpoint=%+v", endpoint)
				}
			}
		})
	}
}

func TestInvalidPublicationArguments(t *testing.T) {
	executable := writeTargetFixture(t)
	for _, args := range [][]string{
		nil, {"--path"}, {"--path", "/a"}, {"--path=/a"},
		{executable, "fixed"}, {executable, executable},
		{executable, "--port", "8080"}, {"--path", "/a", executable, "--token", "abcdefghijklmnop"},
		{executable, "--path", "/a", "sh"}, {"--path", "/a", "cli2http"},
	} {
		t.Run(strings.Join(args, ","), func(t *testing.T) {
			if _, err := parseEndpoints(args); err == nil {
				t.Fatal("expected publication error")
			}
		})
	}
}

func TestDuplicatePathBeforeStartup(t *testing.T) {
	executable := writeTargetFixture(t)
	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{executable, "--path", "/exec", executable})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "duplicate path") {
		t.Fatalf("error=%v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected startup output: %q", output.String())
	}
}

func TestHelpAndVersion(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs([]string{flag})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if output.Len() == 0 {
				t.Fatal("expected help or version output")
			}
		})
	}
}

func TestMultipleStartupOutput(t *testing.T) {
	executable := writeTargetFixture(t)
	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{executable, "--path", "/second", executable + " fixed", "--path", "/first", executable})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("/exec : %s\n/second : %s\n/first : %s\n", executable, executable, executable)
	if !strings.HasSuffix(output.String(), want) || strings.Count(output.String(), "URL     : ") != 1 || strings.Count(output.String(), "Token   : ") != 1 {
		t.Fatalf("unexpected output: %q", output.String())
	}
}

func TestAllTargetsResolvedBeforeStartup(t *testing.T) {
	executable := writeTargetFixture(t)
	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{executable, "--path", "/other", "cli2http-missing-command-5c3497"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected missing executable error")
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected startup output: %q", output.String())
	}
}

func TestDirectoryOption(t *testing.T) {
	base := t.TempDir()
	directory := filepath.Join(base, "with space")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"default", []string{"tool"}, base},
		{"relative", []string{"--dir", "with space", "tool"}, directory},
		{"absolute", []string{"--dir=" + directory, "tool"}, directory},
		{"before path", []string{"--dir", "with space", "--path", "/image", "tool"}, directory},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := newRootCommand()
			if _, err := parseOptions(command, test.args); err != nil {
				t.Fatal(err)
			}
			value, err := command.Flags().GetString("dir")
			if err != nil {
				t.Fatal(err)
			}
			got, err := resolveDirectory(value)
			if err != nil || got != test.want {
				t.Fatalf("directory=%q error=%v, want %q", got, err, test.want)
			}
		})
	}
}

func TestInvalidDirectoryBeforeStartup(t *testing.T) {
	executable := writeTargetFixture(t)
	for _, args := range [][]string{
		{"--dir"},
		{"--dir=", executable},
		{"--dir", filepath.Join(t.TempDir(), "missing"), executable},
		{"--dir", executable, executable},
		{executable, "--dir", t.TempDir()},
	} {
		t.Run(strings.Join(args, ","), func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatal("expected directory option error")
			}
			if output.Len() != 0 {
				t.Fatalf("unexpected startup output: %q", output.String())
			}
		})
	}
}

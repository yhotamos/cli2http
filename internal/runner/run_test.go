package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	if len(args) > 0 {
		switch args[0] {
		case "output":
			stream := os.Stdout
			if args[1] == "stderr" {
				stream = os.Stderr
			}
			count, _ := strconv.Atoi(args[2])
			fmt.Fprint(stream, strings.Repeat("x", count))
			os.Exit(0)
		case "child":
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				os.Exit(1)
			}
			marker := fmt.Sprintf("%d %s", os.Getpid(), listener.Addr())
			if err := os.WriteFile(args[1], []byte(marker), 0600); err != nil {
				os.Exit(1)
			}
			go func() {
				for {
					connection, err := listener.Accept()
					if err != nil {
						return
					}
					connection.Close()
				}
			}()
			time.Sleep(30 * time.Second)
			os.Exit(0)
		case "tree", "orphan", "tree-output":
			child := exec.Command(os.Args[0], "-test.run=^TestRunProcess$", "--", "child", args[1])
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := child.Start(); err != nil {
				os.Exit(1)
			}
			if args[0] == "orphan" {
				os.Exit(0)
			}
			if args[0] == "tree-output" {
				for i := 0; i < 300; i++ {
					if _, err := os.Stat(args[1]); err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				fmt.Fprint(os.Stdout, strings.Repeat("x", maxOutputBytes+1))
			}
			_ = child.Wait()
			os.Exit(0)
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

func TestOutputLimit(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLI2HTTP_RUNNER_PROCESS", "1")
	target := Target{Command: "fixture", Executable: executable}
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := []string{"-test.run=^TestRunProcess$", "--", "output", stream, strconv.Itoa(maxOutputBytes)}
			result, err := target.Run(ctx, args)
			if err != nil || len(result.Stdout)+len(result.Stderr) != maxOutputBytes {
				t.Fatalf("output at limit: error=%v bytes=%d", err, len(result.Stdout)+len(result.Stderr))
			}
			args[len(args)-1] = strconv.Itoa(maxOutputBytes + 1)
			result, err = target.Run(ctx, args)
			if !errors.Is(err, ErrOutputLimit) || result != (Result{}) {
				t.Fatalf("output overflow: error=%v", err)
			}
		})
	}
}

func TestProcessTreeCleanup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLI2HTTP_RUNNER_PROCESS", "1")
	for _, mode := range []string{"tree", "orphan", "tree-output"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			marker := filepath.Join(t.TempDir(), "child")
			target := Target{Command: "fixture", Executable: executable}
			result := make(chan error, 1)
			go func() {
				_, err := target.Run(ctx, []string{"-test.run=^TestRunProcess$", "--", mode, marker})
				result <- err
			}()
			var child []string
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(marker)
				child = strings.Fields(string(data))
				if len(child) == 2 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if len(child) != 2 {
				t.Fatal("descendant did not start")
			}
			pid, err := strconv.Atoi(child[0])
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				connection, err := net.DialTimeout("tcp", child[1], time.Second)
				if err == nil {
					connection.Close()
					process, err := os.FindProcess(pid)
					if err == nil {
						_ = process.Kill()
						_ = process.Release()
					}
				}
			})
			if mode == "tree" {
				cancel()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("expected cancellation, pipe timeout or overflow error")
				}
				if mode == "tree" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation: %v", err)
				}
				if mode == "orphan" && !errors.Is(err, exec.ErrWaitDelay) {
					t.Fatalf("pipe wait: %v", err)
				}
				if mode == "tree-output" && !errors.Is(err, ErrOutputLimit) {
					t.Fatalf("output overflow: %v", err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("execution remained blocked by a descendant")
			}
			deadline = time.Now().Add(2 * time.Second)
			for {
				connection, err := net.DialTimeout("tcp", child[1], 100*time.Millisecond)
				if err != nil {
					break
				}
				connection.Close()
				if time.Now().After(deadline) {
					t.Fatal("descendant is still running")
				}
				time.Sleep(20 * time.Millisecond)
			}
		})
	}
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yhotamos/cli2http/internal/runner"
)

func TestAPIProcess(t *testing.T) {
	if os.Getenv("CLI2HTTP_API_PROCESS") != "1" {
		return
	}
	args := []string{}
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 1 && args[0] == "output" {
		fmt.Fprint(os.Stdout, strings.Repeat("x", 11*1024*1024))
		os.Exit(0)
	}
	if len(args) == 2 && args[0] == "wait" {
		if err := os.WriteFile(args[1], []byte("started"), 0600); err != nil {
			os.Exit(1)
		}
		time.Sleep(30 * time.Second)
	}
	_ = json.NewEncoder(os.Stdout).Encode(args)
	fmt.Fprint(os.Stderr, "stderr")
	if len(args) > 0 && args[0] == "fail" {
		os.Exit(7)
	}
	os.Exit(0)
}

func requestAPI(srv *Server, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(response, request)
	return response
}

func TestHealthInfoAndAuthentication(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	response := requestAPI(srv, http.MethodGet, "/health", "", "")
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health: %d %s", response.Code, response.Body.String())
	}
	response = requestAPI(srv, http.MethodGet, "/info", srv.Token(), "")
	var info struct {
		Command string `json:"command"`
		PID     int    `json:"pid"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || info.Command != "example" || info.PID != os.Getpid() {
		t.Fatalf("info: %d %+v", response.Code, info)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("expected a JSON response")
	}
	for _, path := range []string{"/info", "/exec"} {
		method := http.MethodGet
		if path == "/exec" {
			method = http.MethodPost
		}
		for _, token := range []string{"", "wrong-token"} {
			response := requestAPI(srv, method, path, token, "invalid JSON")
			if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("authentication %s: %d", path, response.Code)
			}
		}
	}
	for _, header := range []string{"Basic " + srv.Token(), srv.Token(), "Bearer " + srv.Token() + " extra"} {
		request := httptest.NewRequest(http.MethodGet, "/info", nil)
		request.Header.Set("Authorization", header)
		response := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid authorization accepted")
		}
	}
	for _, test := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/", http.StatusNotFound},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodGet, "/exec", http.StatusMethodNotAllowed},
		{http.MethodPost, "/info", http.StatusMethodNotAllowed},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
	} {
		if response := requestAPI(srv, test.method, test.path, srv.Token(), ""); response.Code != test.status {
			t.Fatalf("%s %s: %d, want %d", test.method, test.path, response.Code, test.status)
		}
	}
}

func TestExecResults(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLI2HTTP_API_PROCESS", "1")
	srv, err := Listen(runner.Target{Command: "fixture", Executable: executable}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{"success", []string{"a b", "&&", "|", ">", ";", "$HOME"}, 0},
		{"nonzero exit", []string{"fail"}, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestAPIProcess$", "--"}, test.args...)
			body, _ := json.Marshal(map[string]any{"args": args})
			response := requestAPI(srv, http.MethodPost, "/exec", srv.Token(), string(body))
			var result map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(test.args)
			if response.Code != http.StatusOK || len(result) != 3 || result["exitCode"] != float64(test.code) {
				t.Fatalf("exec: %d %+v", response.Code, result)
			}
			if result["stdout"] != string(want)+"\n" || result["stderr"] != "stderr" {
				t.Fatalf("exec: %d %+v", response.Code, result)
			}
		})
	}
	body, _ := json.Marshal(map[string]any{
		"args": []string{"-test.run=^TestAPIProcess$", "--", "output"},
	})
	response := requestAPI(srv, http.MethodPost, "/exec", srv.Token(), string(body))
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "output exceeds 10 MiB") {
		t.Fatalf("output overflow: %d %s", response.Code, response.Body.String())
	}
}

func TestInvalidExecRequest(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "missing", Executable: "cli2http-missing-command-5c3497"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	for _, body := range []string{"", "null", "[]", "{}", `{"args":null}`, `{"args":"wrong"}`, `{"args":[1]}`, `{"args":[null]}`, `{"args":[]`, `{"args":[]} {}`, `{"args":[],"command":"other"}`, strings.Repeat(" ", (1<<20)+1)} {
		response := requestAPI(srv, http.MethodPost, "/exec", srv.Token(), body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid JSON: status %d", response.Code)
		}
	}
	for _, contentType := range []string{"", "text/plain"} {
		request := httptest.NewRequest(http.MethodPost, "/exec", strings.NewReader(`{"args":[]}`))
		request.Header.Set("Authorization", "Bearer "+srv.Token())
		request.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("content type: %d", response.Code)
		}
	}
	response := requestAPI(srv, http.MethodPost, "/exec", srv.Token(), `{"args":[]}`)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("startup failure: %d", response.Code)
	}
}

func TestCancellationWaitsForExecution(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLI2HTTP_API_PROCESS", "1")
	srv, err := Listen(runner.Target{Command: "fixture", Executable: executable}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	executionDone := make(chan struct{})
	handler := srv.http.Handler
	srv.http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
		close(executionDone)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverResult := make(chan error, 1)
	go func() { serverResult <- srv.Run(ctx) }()

	marker := filepath.Join(t.TempDir(), "started")
	body, err := json.Marshal(map[string]any{
		"args": []string{"-test.run=^TestAPIProcess$", "--", "wait", marker},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, srv.URL()+"/exec", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+srv.Token())
	requestResult := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 7 * time.Second}
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusInternalServerError {
				err = fmt.Errorf("canceled execution status: %d", response.StatusCode)
			}
		}
		requestResult <- err
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("CLI did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-serverResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("server did not stop after cancellation")
	}
	select {
	case <-executionDone:
	default:
		t.Fatal("Run returned before execution cleanup completed")
	}
	select {
	case err := <-requestResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("execution response did not finish")
	}
}

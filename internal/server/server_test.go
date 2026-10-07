package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yhotamos/cli2http/internal/runner"
)

func TestStartupAndShutdown(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	address := srv.listener.Addr().(*net.TCPAddr)
	if address.IP.String() != "127.0.0.1" || address.Port == 0 {
		t.Fatalf("unexpected address: %v", address)
	}
	token, err := hex.DecodeString(srv.Token())
	if err != nil || len(token) != 32 {
		t.Fatalf("unexpected token format")
	}
	other, err := Listen(runner.Target{Command: "other"}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if srv.Token() == other.Token() || srv.URL() == other.URL() {
		t.Fatal("servers must have independent tokens and ports")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	var output bytes.Buffer
	go func() { result <- srv.Run(ctx, &output) }()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(srv.URL() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	for _, request := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/info", http.StatusUnauthorized},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
	} {
		req, err := http.NewRequest(request.method, srv.URL()+request.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != request.status {
			t.Fatalf("%s: status = %d, want %d", request.path, response.StatusCode, request.status)
		}
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("server did not stop after cancellation")
	}
	wantLogs := `^GET /health 200 in [0-9]+ms
GET /info 401 in [0-9]+ms
GET /missing 404 in [0-9]+ms
POST /health 405 in [0-9]+ms
$`
	if !regexp.MustCompile(wantLogs).MatchString(output.String()) {
		t.Fatalf("unexpected access logs: %q", output.String())
	}
	connection, err := net.DialTimeout("tcp", address.String(), time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("listener remained open after shutdown")
	}
}

func TestCloseBeforeRun(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	address := srv.listener.Addr().String()
	for i := 0; i < 2; i++ {
		if err := srv.Close(); err != nil {
			t.Fatal(err)
		}
	}
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("listener remained open after Close")
	}
	if err := srv.Run(context.Background(), io.Discard); err != nil {
		t.Fatalf("Run after Close: %v", err)
	}
}

func TestCloseWhileServing(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	started := make(chan struct{})
	finished := make(chan struct{})
	srv.http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(finished)
	})
	serverResult := make(chan error, 1)
	go func() { serverResult <- srv.Run(context.Background(), io.Discard) }()
	requestResult := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get(srv.URL())
		if response != nil {
			response.Body.Close()
		}
		requestResult <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	for i := 0; i < 2; i++ {
		if err := srv.Close(); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not cancel the request")
	}
	select {
	case err := <-serverResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after Close")
	}
	select {
	case err := <-requestResult:
		if err == nil {
			t.Fatal("request unexpectedly completed normally")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("connection remained open after Close")
	}
}

func TestListenPortInUse(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	port := srv.listener.Addr().(*net.TCPAddr).Port
	other, err := Listen(runner.Target{Command: "example"}, port, "")
	if err == nil {
		other.Close()
		t.Fatal("expected an error for a port already in use")
	}
}

func TestListenFixedToken(t *testing.T) {
	for _, token := range []string{"Abcdef01234567-_", strings.Repeat("a", 17)} {
		t.Run(token, func(t *testing.T) {
			for range 2 {
				srv, err := Listen(runner.Target{Command: "example"}, 0, token)
				if err != nil {
					t.Fatal(err)
				}
				if srv.Token() != token {
					t.Errorf("specified token was not preserved")
				}
				if err := srv.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestListenInvalidToken(t *testing.T) {
	for _, token := range []string{
		strings.Repeat("a", 15),
		strings.Repeat("a", 15) + " ",
		strings.Repeat("a", 15) + "!",
		strings.Repeat("a", 15) + "\n",
		strings.Repeat("a", 15) + "あ",
	} {
		t.Run(token, func(t *testing.T) {
			srv, err := Listen(runner.Target{Command: "example"}, 0, token)
			if srv != nil {
				srv.Close()
				t.Fatal("invalid token started a server")
			}
			if err == nil || !strings.Contains(err.Error(), "invalid token") {
				t.Fatalf("error = %v, want token rejection", err)
			}
			if strings.Contains(err.Error(), token) {
				t.Fatal("error contains the supplied token")
			}
		})
	}
}

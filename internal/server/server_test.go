package server

import (
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"testing"
	"time"

	"cli2http/internal/runner"
)

func TestStartupAndShutdown(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"})
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
	other, err := Listen(runner.Target{Command: "other"})
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
	go func() { result <- srv.Run(ctx) }()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(srv.URL() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
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
	connection, err := net.DialTimeout("tcp", address.String(), time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("listener remained open after shutdown")
	}
}

func TestCloseBeforeRun(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"})
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
	if err := srv.Run(context.Background()); err != nil {
		t.Fatalf("Run after Close: %v", err)
	}
}

func TestCloseWhileServing(t *testing.T) {
	srv, err := Listen(runner.Target{Command: "example"})
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
	go func() { serverResult <- srv.Run(context.Background()) }()
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

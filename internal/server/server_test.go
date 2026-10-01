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
	srv, err := New(runner.Target{Command: "example"})
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
	other, err := New(runner.Target{Command: "other"})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if srv.Token() == other.Token() || srv.Address() == other.Address() {
		t.Fatal("servers must have independent tokens and ports")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- srv.Run(ctx) }()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(srv.Address() + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.StatusCode)
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

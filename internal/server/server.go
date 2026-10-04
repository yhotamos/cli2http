package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"cli2http/internal/runner"
)

// Server owns the listener and the startup information for one target CLI.
type Server struct {
	target   runner.Target
	token    string
	listener net.Listener
	http     *http.Server
}

// New reserves an available loopback port and creates a fresh token.
func New(target runner.Target) (*Server, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, fmt.Errorf("generate authentication token: %w", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen on localhost: %w", err)
	}
	s := &Server{
		target:   target,
		token:    hex.EncodeToString(token[:]),
		listener: listener,
		http: &http.Server{
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
	s.http.Handler = s.routes()
	return s, nil
}

func (s *Server) Command() string { return s.target.Command }
func (s *Server) Address() string { return "http://" + s.listener.Addr().String() }
func (s *Server) Token() string   { return s.token }

// Close releases the listener, including when startup output fails.
func (s *Server) Close() error {
	return s.listener.Close()
}

// Run serves HTTP until cancellation, then waits for graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	s.http.BaseContext = func(net.Listener) context.Context { return ctx }
	done := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.http.Shutdown(shutdownCtx); err != nil {
				_ = s.http.Close()
			}
		case <-done:
		}
	}()
	err := s.http.Serve(s.listener)
	close(done)
	<-shutdownDone
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}

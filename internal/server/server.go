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

	"github.com/yhotamos/cli2http/internal/runner"
)

// Server owns the listener and the startup information for one target CLI.
type Server struct {
	target   runner.Target
	token    string
	listener net.Listener
	http     *http.Server
}

// Listen reserves a loopback port and creates a fresh token.
// Port 0 selects an available port.
func Listen(target runner.Target, port int) (*Server, error) {
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 0 and 65535", port)
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, fmt.Errorf("generate authentication token: %w", err)
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
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
func (s *Server) URL() string     { return "http://" + s.listener.Addr().String() }
func (s *Server) Token() string   { return s.token }

// Close stops HTTP connections and releases the listener, even before Run.
// It is safe to call more than once. Closed connections cancel their requests.
func (s *Server) Close() error {
	err := s.http.Close()
	listenerErr := s.listener.Close()
	if err != nil {
		return err
	}
	if errors.Is(listenerErr, net.ErrClosed) {
		return nil
	}
	return listenerErr
}

// Run serves HTTP until cancellation, then waits for request cleanup.
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
				_ = s.Close()
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

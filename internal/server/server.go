package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/yhotamos/cli2http/internal/accesslog"
	"github.com/yhotamos/cli2http/internal/runner"
)

const minTokenLength = 16

// Server owns the listener and the startup information for one target CLI.
type Server struct {
	target   runner.Target
	token    string
	listener net.Listener
	http     *http.Server
}

// Listen reserves a loopback port and uses the specified token.
// An empty token generates a fresh token. Port 0 selects an available port.
func Listen(target runner.Target, port int, token string) (*Server, error) {
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 0 and 65535", port)
	}
	if token == "" {
		var randomToken [32]byte
		if _, err := rand.Read(randomToken[:]); err != nil {
			return nil, fmt.Errorf("generate authentication token: %w", err)
		}
		token = hex.EncodeToString(randomToken[:])
	} else if err := validateToken(token); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("listen on localhost: %w", err)
	}
	s := &Server{
		target:   target,
		token:    token,
		listener: listener,
		http: &http.Server{
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
	s.http.Handler = s.routes()
	return s, nil
}

func validateToken(token string) error {
	if len(token) < minTokenLength {
		return fmt.Errorf("invalid token: must contain at least %d characters", minTokenLength)
	}
	for _, character := range token {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '-' || character == '_':
		default:
			return errors.New("invalid token: only ASCII letters, digits, - and _ are allowed")
		}
	}
	return nil
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

// Run serves HTTP with access logging until cancellation, then waits for request cleanup.
func (s *Server) Run(ctx context.Context, output io.Writer) error {
	s.http.Handler = accesslog.Wrap(s.http.Handler, output)
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

package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/yhotamos/cli2http/internal/runner"
)

const maxRequestBodyBytes = 1 << 20

type execResponse struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	for _, endpoint := range s.endpoints {
		mux.HandleFunc("POST "+endpoint.Path, s.authenticate(func(w http.ResponseWriter, r *http.Request) {
			s.handleExec(w, r, endpoint.Target)
		}))
	}
	mux.HandleFunc("GET /info", s.authenticate(s.handleInfo))
	mux.HandleFunc("GET /health", s.handleHealth)
	return mux
}

func (s *Server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("Authorization")
		if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte("Bearer "+s.token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, "invalid authentication token")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request, target runner.Target) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var request struct {
		Args []*string `json:"args"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeRequestError(w, err, "failed to read request body")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Args == nil {
		writeError(w, http.StatusBadRequest, "expected a JSON object with args as a string array")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "expected a single JSON object")
		return
	}
	args := make([]string, len(request.Args))
	for i, arg := range request.Args {
		if arg == nil {
			writeError(w, http.StatusBadRequest, "args must contain only strings")
			return
		}
		if strings.ContainsRune(*arg, 0) {
			writeError(w, http.StatusBadRequest, "args must not contain NUL")
			return
		}
		args[i] = *arg
	}
	ctx := r.Context()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	result, err := target.Run(ctx, args)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusUnprocessableEntity, "command execution timed out")
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, runner.ErrOutputLimit) {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, execResponse{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	type endpointInfo struct {
		Path    string `json:"path"`
		Command string `json:"command"`
	}
	endpoints := make([]endpointInfo, len(s.endpoints))
	for i, endpoint := range s.endpoints {
		endpoints[i] = endpointInfo{Path: endpoint.Path, Command: endpoint.Target.Command}
	}
	writeJSON(w, http.StatusOK, struct {
		PID       int            `json:"pid"`
		Endpoints []endpointInfo `json:"endpoints"`
	}{PID: os.Getpid(), Endpoints: endpoints})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}

func writeRequestError(w http.ResponseWriter, err error, fallbackMessage string) {
	var limitError *http.MaxBytesError
	if errors.As(err, &limitError) {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds the %g MiB limit", float64(limitError.Limit)/(1<<20)))
		return
	}
	writeError(w, http.StatusBadRequest, fallbackMessage)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

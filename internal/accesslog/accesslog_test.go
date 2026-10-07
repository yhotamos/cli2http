package accesslog

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWrapResponse(t *testing.T) {
	for _, test := range []struct {
		name    string
		headers []int
		body    string
		status  int
	}{
		{"explicit", []int{201}, "response", 201},
		{"implicit", nil, "response", 200},
		{"empty", nil, "", 200},
		{"error", []int{500}, "response", 500},
		{"repeated", []int{400, 500}, "response", 400},
		{"switching protocols", []int{101}, "", 101},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if output.Len() != 0 {
					t.Fatal("request was logged before completion")
				}
				w.Header().Set("X-Example", "preserved")
				for _, status := range test.headers {
					w.WriteHeader(status)
				}
				if test.body != "" {
					_, _ = io.WriteString(w, test.body)
				}
			}), &output)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/info", nil))
			if response.Code != test.status || response.Body.String() != test.body || response.Header().Get("X-Example") != "preserved" {
				t.Fatalf("response changed: status=%d body=%q headers=%v", response.Code, response.Body.String(), response.Header())
			}
			if test.name == "implicit" && response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Fatal("implicit content type was not preserved")
			}
			want := `^GET /info ` + strconv.Itoa(test.status) + ` in [0-9]+ms\n$`
			if !regexp.MustCompile(want).MatchString(output.String()) {
				t.Fatalf("unexpected log: %q", output.String())
			}
		})
	}
}

func TestWrapDurationAndPrivacy(t *testing.T) {
	var output bytes.Buffer
	handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		_, _ = io.WriteString(w, "private command output")
	}), &output)
	request := httptest.NewRequest(http.MethodPost, "/a%0Ab?token=private-query", strings.NewReader("private arguments"))
	request.Header.Set("Authorization", "Bearer private-token")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	match := regexp.MustCompile(`^POST /a%0Ab 200 in ([0-9]+)ms\n$`).FindStringSubmatch(output.String())
	if match == nil {
		t.Fatalf("unexpected log: %q", output.String())
	}
	elapsed, err := strconv.Atoi(match[1])
	if err != nil || elapsed < 10 {
		t.Fatalf("handler duration was not included: %q", match[1])
	}
}

func TestWrapInformationalResponse(t *testing.T) {
	var output bytes.Buffer
	server := httptest.NewServer(Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "response")
	}), &output))
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(server.URL + "/info")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	server.Close()
	if err != nil || response.StatusCode != http.StatusCreated || string(body) != "response" {
		t.Fatalf("response changed: status=%d body=%q error=%v", response.StatusCode, body, err)
	}
	if !regexp.MustCompile(`^GET /info 201 in [0-9]+ms\n$`).MatchString(output.String()) {
		t.Fatalf("unexpected log: %q", output.String())
	}
}

func TestWrapConcurrentRequests(t *testing.T) {
	var output bytes.Buffer
	handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), &output)
	var requests sync.WaitGroup
	for i := 0; i < 20; i++ {
		requests.Add(1)
		go func() {
			defer requests.Done()
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
		}()
	}
	requests.Wait()
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 20 {
		t.Fatalf("log lines = %d, want 20", len(lines))
	}
	format := regexp.MustCompile(`^GET /health 200 in [0-9]+ms$`)
	for _, line := range lines {
		if !format.MatchString(line) {
			t.Fatalf("invalid log line: %q", line)
		}
	}
}

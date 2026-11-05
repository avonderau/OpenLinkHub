package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyRequestBodyLimits(t *testing.T) {
	if got := legacyRequestBodyLimit("/api/speed"); got != 4<<20 {
		t.Fatalf("unexpected JSON request limit: %d", got)
	}
	for _, route := range []string{"/api/restore", "/api/lcd/upload"} {
		if got := legacyRequestBodyLimit(route); got != 6<<20 {
			t.Fatalf("%s request limit = %d, want 6 MiB", route, got)
		}
	}
}

// The shared router must reject an oversized Content-Length before any
// inherited device handler is allowed to change hardware state.
func TestLegacyOversizedRequestRejectedBeforeHandler(t *testing.T) {
	for _, tc := range []struct {
		route  string
		method string
	}{
		{"/api/speed", http.MethodPost},
		{"/api/macro/new", http.MethodPut},
		{"/api/macro/profile", http.MethodDelete},
		{"/api/restore", http.MethodPost},
		{"/api/lcd/upload", http.MethodPost},
	} {
		t.Run(tc.route, func(t *testing.T) {
			called := false
			mux := http.NewServeMux()
			handleFunc(mux, tc.route, tc.method, func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			r := httptest.NewRequest(tc.method, "http://localhost:27003"+tc.route, nil)
			r.ContentLength = legacyRequestBodyLimit(tc.route) + 1
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusRequestEntityTooLarge || called {
				t.Fatalf("oversized %s returned %d, handler called %v", tc.route, w.Code, called)
			}
		})
	}
}

// ContentLength can be unknown for chunked requests. MaxBytesReader must
// enforce the cap as the inherited JSON parser reads the stream.
func TestLegacyChunkedRequestBodyIsBounded(t *testing.T) {
	mux := http.NewServeMux()
	var readErr error
	handleFunc(mux, "/api/speed", http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.Copy(io.Discard, r.Body)
		if readErr != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodPost, "http://localhost:27003/api/speed",
		strings.NewReader(strings.Repeat("x", int(maxLegacyRequestBodyBytes)+1)))
	r.ContentLength = -1
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) || tooLarge.Limit != maxLegacyRequestBodyBytes {
		t.Fatalf("unknown-length oversized body not bounded: %v", readErr)
	}
	if w.Code == http.StatusNoContent {
		t.Fatal("oversized streaming body unexpectedly accepted")
	}
}

func TestLegacyRequestBodyUnderLimitStillAvailable(t *testing.T) {
	mux := http.NewServeMux()
	handleFunc(mux, "/api/speed", http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		if err != nil || string(got) != `{"speed":40}` {
			t.Errorf("valid JSON body modified or lost: %q %v", got, err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodPost, "http://localhost:27003/api/speed", strings.NewReader(`{"speed":40}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("valid request failed: %d", w.Code)
	}
}

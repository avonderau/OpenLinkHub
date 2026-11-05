package server

import (
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

// ContentLength can be unknown for chunked requests. An oversized body
// must be rejected BEFORE the handler could act on a valid JSON prefix.
func TestLegacyChunkedRequestBodyIsBounded(t *testing.T) {
	for _, prefix := range []string{"", `{"speed":40}`} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			mux := http.NewServeMux()
			called := false
			handleFunc(mux, "/api/speed", http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			body := prefix + strings.Repeat("x", int(maxLegacyRequestBodyBytes)+1)
			r := httptest.NewRequest(http.MethodPost, "http://localhost:27003/api/speed", strings.NewReader(body))
			r.ContentLength = -1
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusRequestEntityTooLarge || called {
				t.Fatalf("unknown-length oversized body returned %d, handler called %v", w.Code, called)
			}
		})
	}
}

// Existing LCD and backup forms permit a 5 MiB file. The shared guard
// must not silently reduce those endpoints to the JSON control quota.
func TestLegacyUploadRoutesKeepFiveMiBCapacity(t *testing.T) {
	for _, route := range []string{"/api/lcd/upload", "/api/restore"} {
		t.Run(route, func(t *testing.T) {
			mux := http.NewServeMux()
			var consumed int64
			handleFunc(mux, route, http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
				var err error
				consumed, err = io.Copy(io.Discard, r.Body)
				if err != nil {
					t.Errorf("valid upload unexpectedly truncated: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost,
				"http://localhost:27003"+route,
				strings.NewReader(strings.Repeat("x", 5<<20)))
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent || consumed != 5<<20 {
				t.Fatalf("upload body truncated: HTTP %d, bytes %d", response.Code, consumed)
			}
		})
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

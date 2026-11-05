package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The shared route guard must reject browser-originated cross-site hardware
// writes before upstream request parsing or any device action is reached.
func TestLegacyAPIWriteGuard(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		origin    string
		fetchSite string
		want      int
	}{
		{"same-origin browser", http.MethodPost, "http://localhost:27003", "same-origin", http.StatusNoContent},
		{"local CLI without browser metadata", http.MethodPost, "", "", http.StatusNoContent},
		{"cross-site with explicit origin", http.MethodPost, "http://attacker.example", "cross-site", http.StatusForbidden},
		{"cross-site without origin", http.MethodPost, "", "cross-site", http.StatusForbidden},
		{"same-site sibling origin", http.MethodPost, "http://localhost:27003", "same-site", http.StatusForbidden},
		{"fake origin with cross-site fetch metadata", http.MethodPost, "http://localhost:27003", "cross-site", http.StatusForbidden},
		{"untrusted origin despite same-origin fetch metadata", http.MethodPost, "http://attacker.example", "same-origin", http.StatusForbidden},
		{"null browser origin", http.MethodPost, "null", "same-origin", http.StatusForbidden},
		{"GET cannot trigger POST handler", http.MethodGet, "", "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			calls := 0
			handleFunc(mux, "/api/test-write", http.MethodPost, func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusNoContent)
			})
			r := httptest.NewRequest(tt.method, "http://localhost:27003/api/test-write", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if tt.fetchSite != "" {
				r.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, r)
			if response.Code != tt.want {
				t.Fatalf("HTTP status = %d; want %d", response.Code, tt.want)
			}
			wantCalls := 0
			if tt.want == http.StatusNoContent {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("handler invoked %d times; want %d", calls, wantCalls)
			}
			if tt.method == http.MethodGet && response.Header().Get("Allow") != http.MethodPost {
				t.Fatal("wrong-method response must advertise supported method")
			}
		})
	}
}

func TestLegacyReadRouteRemainsAccessible(t *testing.T) {
	mux := http.NewServeMux()
	called := false
	handleFunc(mux, "/api/test-read", http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodGet, "http://localhost:27003/api/test-read", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, r)
	if !called || response.Code != http.StatusNoContent {
		t.Fatalf("read-only API unexpectedly blocked: called=%t code=%d", called, response.Code)
	}
}

func TestLoopbackHostPolicyRejectsDNSRebinding(t *testing.T) {
	for _, host := range []string{"attacker.example:27003", "localhost.attacker.example:27003", "127.0.0.1.attacker.example:27003"} {
		if nexusHostAllowed("127.0.0.1", host, "") {
			t.Errorf("loopback WebUI accepted rebinding Host %q", host)
		}
	}
	for _, host := range []string{"localhost:27003", "127.0.0.1:27003", "[::1]:27003"} {
		if !nexusHostAllowed("127.0.0.1", host, "") {
			t.Errorf("loopback WebUI rejected local Host %q", host)
		}
	}
}

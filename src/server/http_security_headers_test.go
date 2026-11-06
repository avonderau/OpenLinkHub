package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A local control dashboard must not be frameable by unrelated websites.
// Using SAMEORIGIN preserves any in-app same-origin embedded views.
func TestHTTPResponseHeadersPreventClickjacking(t *testing.T) {
	for _, target := range []string{
		"/static/nonexistent.css",
		"/api/nexus/telemetry?serial=not-a-device",
		"/api/nexus/widget-library",
	} {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://localhost:27003"+target, nil)
			w := httptest.NewRecorder()
			setRoutes().ServeHTTP(w, req)
			for key, want := range map[string]string{
				"X-Frame-Options":       "SAMEORIGIN",
				"Content-Security-Policy": "frame-ancestors 'self'",
				"X-Content-Type-Options": "nosniff",
				"Referrer-Policy":        "same-origin",
			} {
				if got := w.Header().Get(key); got != want {
					t.Errorf("GET %s: %s = %q, want %q", target, key, got, want)
				}
			}
		})
	}
}

func TestHTTPResponseSecurityHeadersAlsoCoverRejectedRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://localhost:27003/api/nexus/widget-library", nil)
	req.Header.Set("Origin", "http://attacker.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	setRoutes().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unexpected cross-origin denial status %d", w.Code)
	}
	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatal("denied requests must also receive anti-framing headers")
	}
}

func TestHTTPResponseSecurityHeadersCoverAuthShortCircuit(t *testing.T) {
	// The auth middleware can reject an unauthenticated request without
	// calling setRoutes, so its outer wrapper must still set the headers.
	unauthorized := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
	})
	req := httptest.NewRequest(http.MethodGet, "https://example.org/api/nexus/screens", nil)
	w := httptest.NewRecorder()
	withHTTPResponseSecurityHeaders(unauthorized).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected auth rejection status: %d", w.Code)
	}
	for header, want := range map[string]string{
		"X-Frame-Options": "SAMEORIGIN",
		"Content-Security-Policy": "frame-ancestors 'self'",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy": "same-origin",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("auth short-circuit missing %s: got %q, want %q", header, got, want)
		}
	}
}

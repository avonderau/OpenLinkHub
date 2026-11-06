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
	req := httptest.NewRequest(http.MethodPost, "http://localhost:27003/api/deck/layout", nil)
	req.Header.Set("Origin", "http://attacker.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	setRoutes().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
		t.Fatalf("unexpected Deck denial status %d", w.Code)
	}
	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatal("denied requests must also receive anti-framing headers")
	}
}

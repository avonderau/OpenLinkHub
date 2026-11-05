package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The invalid action avoids emitting host keyboard events while exercising
// the transport and origin checks that protect every real media action.
func TestMediaPlaybackControlRejectsCrossSiteAndLegacyGET(t *testing.T) {
	cases := []struct {
		name        string
		method      string
		origin      string
		fetchSite   string
		contentType string
		wantStatus  int
	}{
		{"legacy GET", http.MethodGet, "", "", "", http.StatusMethodNotAllowed},
		{"cross-site image-style GET", http.MethodGet, "https://malicious.example", "cross-site", "", http.StatusMethodNotAllowed},
		{"cross-site JSON POST", http.MethodPost, "https://malicious.example", "cross-site", "application/json", http.StatusForbidden},
		{"spoofed Origin POST", http.MethodPost, "https://malicious.example", "same-origin", "application/json", http.StatusForbidden},
		{"cross-site form POST", http.MethodPost, "https://malicious.example", "cross-site", "application/x-www-form-urlencoded", http.StatusForbidden},
		{"same-origin simple form", http.MethodPost, "http://localhost:27003", "same-origin", "text/plain", http.StatusUnsupportedMediaType},
		{"same-origin JSON", http.MethodPost, "http://localhost:27003", "same-origin", "application/json; charset=utf-8", http.StatusOK},
		{"local API JSON POST", http.MethodPost, "", "", "application/json", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://localhost:27003/api/media/unsupported", strings.NewReader("{}"))
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			}
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			recorder := httptest.NewRecorder()
			mediaPlaybackControl(recorder, req)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; response: %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.method == http.MethodGet && recorder.Header().Get("Allow") != http.MethodPost {
				t.Fatalf("GET did not advertise POST-only media actions")
			}
		})
	}
}

func TestMediaPlaybackRouteRejectsGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:27003/api/media/next", nil)
	recorder := httptest.NewRecorder()
	setRoutes().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET media action returned %d, want 405", recorder.Code)
	}
}

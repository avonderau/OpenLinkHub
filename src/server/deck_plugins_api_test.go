package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"OpenLinkHub/src/devices/deck"
)

func TestDeckPluginInstallOnlyFromThisComputer(t *testing.T) {
	cases := []struct {
		remote string
		header string
		want   bool
	}{
		{"127.0.0.1:5000", "", true},
		{"[::1]:5000", "", true},
		{"192.168.1.20:5000", "", false},
		{"127.0.0.1:5000", "X-Forwarded-For", false},
		{"127.0.0.1:5000", "Forwarded", false},
		{"garbage", "", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/api/deck/plugins", nil)
		r.RemoteAddr = c.remote
		if c.header != "" {
			r.Header.Set(c.header, "203.0.113.9")
		}
		if got := deckPluginInstallAllowed(r); got != c.want {
			t.Errorf("%s %s: got %v", c.remote, c.header, got)
		}
	}
}

func TestDeckInspectorShimAndSandbox(t *testing.T) {
	ticket := &deck.InspectorTicket{ID: "0123456789abcdef0123456789abcdef", Context: "0123456789abcdef01234567",
		Settings: json.RawMessage(`{"x":"</script><b>"}`)}
	page := string(injectInspectorShim([]byte(`<!doctype html><HTML><Head lang="en"><title>t</title></head><body></body></html>`), ticket))
	head := strings.Index(page, `<Head lang="en">`)
	shim := strings.Index(page, "<script>")
	if head < 0 || shim != head+len(`<Head lang="en">`) {
		t.Fatalf("shim not placed right after <head>: %s", page)
	}
	if strings.Contains(page, `</script><b>`) {
		t.Fatal("settings can break out of the shim script")
	}
	if !strings.Contains(page, deckPluginSocketPrefix) || !strings.Contains(page, "registerPropertyInspector") {
		t.Fatal("shim does not connect the page")
	}
	if bare := string(injectInspectorShim([]byte("<p>hi</p>"), ticket)); !strings.HasPrefix(bare, "<script>") {
		t.Fatal("pages without <head> get the shim first")
	}

	// Unknown tickets get nothing, and every response is sandboxed.
	for _, path := range []string{deckPluginUIPrefix + "ffffffffffffffffffffffffffffffff/ui/pi.html", deckPluginUIPrefix + "../../api/deck/layout"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		r.Host = "127.0.0.1:27003"
		deckPluginUIFile(w, r)
		if w.Code != 404 || !strings.Contains(strings.Join(w.Header().Values("Content-Security-Policy"), ";"), "sandbox") ||
			w.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s: %d %v", path, w.Code, w.Header())
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", deckPluginSocketPrefix+"ffffffffffffffffffffffffffffffff", nil)
	r.Host = "127.0.0.1:27003"
	deckPluginSocket(w, r)
	if w.Code != 404 {
		t.Fatalf("socket with unknown ticket: %d", w.Code)
	}
	for origin, want := range map[string]bool{"null": true, "": true, "http://127.0.0.1:27003": true, "https://evil.example": false} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = "127.0.0.1:27003"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if inspectorUpgrader.CheckOrigin(r) != want {
			t.Errorf("origin %q", origin)
		}
	}
}

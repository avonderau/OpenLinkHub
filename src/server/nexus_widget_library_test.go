package server

import (
	"OpenLinkHub/src/devices/nexus"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleSavedWidget() nexusLibraryEntry {
	return nexusLibraryEntry{ID: "cpuMeter", Name: "CPU meter", Widget: nexus.Widget{
		ID: "cpu", Type: "metric", Metric: "cpu.utilization", X: 20, Y: 0, Width: 180, Height: 48,
		Settings: map[string]string{"label": "CPU", "style": "outline", "textColor": "#00aaff"},
	}}
}

func widgetRequest(t *testing.T, root, method, uri string, body any, origin string) *httptest.ResponseRecorder {
	t.Helper()
	payload := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = string(encoded)
	}
	req := httptest.NewRequest(method, "http://localhost:27003"+uri, strings.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
	}
	out := httptest.NewRecorder()
	serveNexusWidgetLibrary(out, req, root)
	return out
}

func TestNexusWidgetLibraryPersistedAcrossHandlerCalls(t *testing.T) {
	root := t.TempDir()
	first := widgetRequest(t, root, http.MethodGet, "/api/nexus/widget-library", nil, "")
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"entries":[]`) {
		t.Fatalf("initial list=%d %q", first.Code, first.Body.String())
	}
	entry := sampleSavedWidget()
	saved := widgetRequest(t, root, http.MethodPost, "/api/nexus/widget-library", entry, "")
	if saved.Code != http.StatusCreated {
		t.Fatalf("save status=%d: %s", saved.Code, saved.Body.String())
	}
	file := nexusWidgetLibraryPath(root)
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe widget file mode %o", info.Mode().Perm())
	}
	loaded, err := readNexusWidgetLibrary(root)
	if err != nil || len(loaded) != 1 || loaded[0].Widget.ID != "cpu" {
		t.Fatalf("stored data incorrect: %+v %v", loaded, err)
	}
	reloaded := widgetRequest(t, root, http.MethodGet, "/api/nexus/widget-library", nil, "")
	if reloaded.Code != http.StatusOK || !strings.Contains(reloaded.Body.String(), `"CPU meter"`) {
		t.Fatalf("library not served after reload: %d %q", reloaded.Code, reloaded.Body.String())
	}
	duplicate := widgetRequest(t, root, http.MethodPost, "/api/nexus/widget-library", entry, "")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate accepted: %d", duplicate.Code)
	}
	gone := widgetRequest(t, root, http.MethodDelete, "/api/nexus/widget-library?id=cpuMeter", nil, "")
	if gone.Code != http.StatusNoContent {
		t.Fatalf("delete status %d: %s", gone.Code, gone.Body.String())
	}
	after, err := readNexusWidgetLibrary(root)
	if err != nil || len(after) != 0 {
		t.Fatalf("deleted widget returned: %+v %v", after, err)
	}
}

func TestNexusWidgetLibraryRejectsUnsafeEntries(t *testing.T) {
	root := t.TempDir()
	e := sampleSavedWidget()
	for _, test := range []struct {
		name   string
		change func(*nexusLibraryEntry)
	}{
		{"path traversal", func(e *nexusLibraryEntry) { e.ID = "../../outside" }},
		{"oversized name", func(e *nexusLibraryEntry) { e.Name = strings.Repeat("x", 81) }},
		{"unsupported widget", func(e *nexusLibraryEntry) { e.Widget.Type = "iframe" }},
		{"invalid dimensions", func(e *nexusLibraryEntry) { e.Widget.Width = 1000 }},
		{"executable macro", func(e *nexusLibraryEntry) { e.Widget.Type = "macro" }},
		{"unsafe action", func(e *nexusLibraryEntry) {
			e.Widget.Type = "button"
			e.Widget.Metric = ""
			e.Widget.Action = "shell"
			e.Widget.Target = "rm -rf"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := e
			test.change(&candidate)
			result := widgetRequest(t, root, http.MethodPost, "/api/nexus/widget-library", candidate, "")
			if result.Code != http.StatusBadRequest {
				t.Fatalf("unsafe entry accepted (%d): %s", result.Code, result.Body.String())
			}
		})
	}
	e.Widget.Type = "button"
	e.Widget.Metric = ""
	e.Widget.Action = "screen"
	e.Widget.Target = "another-screen"
	if err := validateNexusWidgetLibraryEntry(e); err != nil {
		t.Fatalf("reusable navigation should accept a valid non-current screen: %v", err)
	}
}

func TestNexusWidgetLibraryMethodOriginAndContentType(t *testing.T) {
	root := t.TempDir()
	valid := sampleSavedWidget()
	foreign := widgetRequest(t, root, http.MethodPost, "/api/nexus/widget-library", valid, "http://evil.example")
	if foreign.Code != http.StatusForbidden {
		t.Fatalf("cross-origin save accepted: %d", foreign.Code)
	}
	wrongMethod := widgetRequest(t, root, http.MethodPut, "/api/nexus/widget-library", valid, "")
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unsupported method: %d", wrongMethod.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost:27003/api/nexus/widget-library", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	output := httptest.NewRecorder()
	serveNexusWidgetLibrary(output, req, root)
	if output.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form POST accepted: %d", output.Code)
	}
	if got := widgetRequest(t, root, http.MethodDelete, "/api/nexus/widget-library?id=../bad", nil, ""); got.Code != http.StatusBadRequest {
		t.Fatalf("unsafe deletion ID accepted: %d", got.Code)
	}
	if _, err := os.Stat(filepath.Join(root, "database", "nexus", "widget-library.json")); !os.IsNotExist(err) {
		t.Fatalf("rejected requests wrote library file: %v", err)
	}
}

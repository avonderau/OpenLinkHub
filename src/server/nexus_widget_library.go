package server

import (
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices/nexus"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const maxNexusWidgetLibraryBytes = 256 * 1024
const maxNexusWidgetLibraryEntries = 50

var widgetLibrarySafeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var widgetLibraryMutex sync.Mutex

type nexusLibraryEntry struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Widget nexus.Widget `json:"widget"`
}

func nexusWidgetLibraryPath(root string) string {
	return filepath.Join(root, "database", "nexus", "widget-library.json")
}

func validateNexusWidgetLibraryEntry(entry nexusLibraryEntry) error {
	if !widgetLibrarySafeID.MatchString(entry.ID) {
		return errors.New("invalid widget library entry ID")
	}
	if strings.TrimSpace(entry.Name) == "" || len(entry.Name) > 80 || strings.ContainsAny(entry.Name, "\x00\r\n") {
		return errors.New("invalid widget library entry name")
	}
	if !widgetLibrarySafeID.MatchString(entry.Widget.ID) {
		return errors.New("invalid saved widget ID")
	}
	if entry.Widget.Type == "macro" {
		return errors.New("legacy inline macro widgets are not supported; save a button with a shared macro action")
	}
	// A reusable screen-navigation widget may refer to a different Nexus
	// screen. Validate its target syntax and then substitute a known test
	// screen during shape and action validation; keep the original target
	// in the persisted reusable widget for editor-time reassignment.
	w := entry.Widget
	if w.Type == "button" && w.Action == "screen" {
		if !widgetLibrarySafeID.MatchString(w.Target) {
			return errors.New("invalid widget target screen")
		}
		w.Target = "library"
	}
	layout := nexus.ScreenConfiguration{
		Version:        1,
		ActiveScreenID: "library",
		Screens:        []nexus.Screen{{ID: "library", Name: "Saved widget", Widgets: []nexus.Widget{w}}},
	}
	if err := layout.Validate(); err != nil {
		return fmt.Errorf("invalid reusable widget: %w", err)
	}
	return nil
}

func readNexusWidgetLibrary(root string) ([]nexusLibraryEntry, error) {
	path := nexusWidgetLibraryPath(root)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []nexusLibraryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxNexusWidgetLibraryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxNexusWidgetLibraryBytes {
		return nil, errors.New("widget library exceeds 256 KiB")
	}
	var entries []nexusLibraryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("invalid stored widget library: %w", err)
	}
	if entries == nil || len(entries) > maxNexusWidgetLibraryEntries {
		return nil, errors.New("invalid stored widget library size")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if err := validateNexusWidgetLibraryEntry(e); err != nil {
			return nil, err
		}
		if seen[e.ID] {
			return nil, errors.New("duplicate stored widget library ID")
		}
		seen[e.ID] = true
	}
	return entries, nil
}

func writeNexusWidgetLibrary(root string, entries []nexusLibraryEntry) error {
	if len(entries) > maxNexusWidgetLibraryEntries {
		return errors.New("widget library is full")
	}
	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if len(encoded) > maxNexusWidgetLibraryBytes {
		return errors.New("widget library exceeds 256 KiB")
	}
	path := nexusWidgetLibraryPath(root)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".widget-library-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func nexusWidgetLibraryAPI(w http.ResponseWriter, r *http.Request) {
	serveNexusWidgetLibrary(w, r, config.GetConfig().ConfigPath)
}

// Extract the storage root so tests can exercise the actual HTTP handler
// without modifying user configuration or requiring a physical Nexus.
func serveNexusWidgetLibrary(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !nexusRequestHostAllowed(r) {
		http.Error(w, "host forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && !nexusWriteAllowed(r) {
		http.Error(w, "cross-origin widget write forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	widgetLibraryMutex.Lock()
	defer widgetLibraryMutex.Unlock()
	entries, err := readNexusWidgetLibrary(root)
	if err != nil {
		http.Error(w, "unable to read widget library", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries})
	case http.MethodPost:
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || contentType != "application/json" {
			http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var entry nexusLibraryEntry
		if err := dec.Decode(&entry); err != nil {
			http.Error(w, "invalid widget entry", http.StatusBadRequest)
			return
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			http.Error(w, "unexpected trailing JSON", http.StatusBadRequest)
			return
		}
		if err := validateNexusWidgetLibraryEntry(entry); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(entries) >= maxNexusWidgetLibraryEntries {
			http.Error(w, "widget library is full", http.StatusConflict)
			return
		}
		for _, existing := range entries {
			if existing.ID == entry.ID {
				http.Error(w, "widget ID already exists", http.StatusConflict)
				return
			}
		}
		if err := writeNexusWidgetLibrary(root, append(entries, entry)); err != nil {
			http.Error(w, "unable to save widget library", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(entry)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if !widgetLibrarySafeID.MatchString(id) {
			http.Error(w, "invalid widget ID", http.StatusBadRequest)
			return
		}
		retained := make([]nexusLibraryEntry, 0, len(entries))
		removed := false
		for _, entry := range entries {
			if entry.ID == id {
				removed = true
				continue
			}
			retained = append(retained, entry)
		}
		if !removed {
			http.Error(w, "saved widget not found", http.StatusNotFound)
			return
		}
		if err := writeNexusWidgetLibrary(root, retained); err != nil {
			http.Error(w, "unable to update widget library", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

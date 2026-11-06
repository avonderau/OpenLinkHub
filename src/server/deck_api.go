package server

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"OpenLinkHub/src/deckplugin"
	"OpenLinkHub/src/devices"
	"OpenLinkHub/src/devices/deck"
)

func deckDevice(r *http.Request) (*deck.Device, bool) {
	if !nexusRequestHostAllowed(r) {
		return nil, false
	}
	id := r.URL.Query().Get("serial")
	if id == "" {
		return nil, false
	}
	d, ok := devices.GetDevice(id).(*deck.Device)
	return d, ok
}

func deckStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, ok := deckDevice(r)
	if !ok {
		http.Error(w, "Deck device not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(d.GetStatus())
}

func deckLayoutAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, ok := deckDevice(r)
	if !ok {
		http.Error(w, "Deck device not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(d.GetLayout())
		return
	}
	if !nexusWriteAllowed(r) {
		http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, deck.MaxDeckLayoutBytes)
	defer r.Body.Close()
	var layout deck.Layout
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&layout); err != nil {
		http.Error(w, "invalid layout: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		http.Error(w, "unexpected trailing JSON", http.StatusBadRequest)
		return
	}
	result, err := d.UpdateLayout(layout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(result)
}

func deckIconAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, ok := deckDevice(r)
	if !ok {
		http.Error(w, "Deck device not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodGet {
		content, err := d.ReadIcon(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "Deck icon not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(content)
		return
	}
	if !nexusWriteAllowed(r) {
		http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (mediaType != "image/png" && mediaType != "image/jpeg") {
		http.Error(w, "PNG or JPEG required", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, deck.MaxDeckIconBytes)
	defer r.Body.Close()
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid image upload", http.StatusBadRequest)
		return
	}
	id, err := d.SaveIcon(content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// deckPluginsAPI lists the plugin actions the desktop agent reported.
func deckPluginsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !nexusRequestHostAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	connected, catalog := deck.PluginCatalog()
	if catalog == nil {
		catalog = []deckplugin.CatalogPlugin{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected, "plugins": catalog})
}

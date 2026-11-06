package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
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

// deckPluginsAPI is the Deck plugin manager: GET lists the plugins the
// desktop agent reported, POST installs an uploaded archive and DELETE
// removes one (?plugin=<uuid>).
func deckPluginsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !nexusRequestHostAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch r.Method {
	case http.MethodGet:
		connected, catalog := deck.PluginCatalog()
		if catalog == nil {
			catalog = []deckplugin.CatalogPlugin{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected, "plugins": catalog,
			"canInstall": deckPluginInstallAllowed(r), "maxUploadBytes": deckplugin.MaxUploadBytes})
		return
	case http.MethodPost:
		if !nexusWriteAllowed(r) {
			http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
			return
		}
		// A plugin is a program that runs as the desktop user, so it can
		// only be installed from a browser on this computer.
		if !deckPluginInstallAllowed(r) {
			http.Error(w, "plugins can only be installed from a browser on this computer", http.StatusForbidden)
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || (mediaType != "application/zip" && mediaType != "application/octet-stream" &&
			mediaType != "application/x-zip-compressed") {
			http.Error(w, "plugin archive (.streamDeckPlugin or .zip) required", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, deckplugin.MaxUploadBytes)
		defer r.Body.Close()
		archive, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "plugin archive is too large or the upload failed", http.StatusRequestEntityTooLarge)
			return
		}
		uuid, err := deck.InstallPlugin(archive)
		if err != nil {
			deckPluginError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"plugin": uuid})
	case http.MethodDelete:
		if !nexusWriteAllowed(r) {
			http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
			return
		}
		if err := deck.RemovePlugin(r.URL.Query().Get("plugin")); err != nil {
			deckPluginError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deckPluginError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, deck.ErrPluginHostOffline) {
		status = http.StatusServiceUnavailable
	}
	http.Error(w, err.Error(), status)
}

// deckPluginInstallAllowed accepts direct loopback requests only. Requests
// relayed by a reverse proxy carry forwarding headers and are refused,
// because the proxy itself connects from loopback.
func deckPluginInstallAllowed(r *http.Request) bool {
	for _, h := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip"} {
		if r.Header.Get(h) != "" {
			return false
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

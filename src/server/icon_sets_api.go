package server

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices/deck"
	"OpenLinkHub/src/devices/nexus"
)

const (
	maxIconSetZipBytes        = 32 << 20
	maxIconSetTotalImageBytes = 64 << 20
	maxIconSetImages          = 4096
	maxIconSets               = 24
	maxIconSetArchiveEntries  = 8192
)

var iconSetIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)
var iconSetImportMu sync.Mutex

type iconSetEntry struct {
	Name    string   `json:"name"`
	Tags    []string `json:"tags,omitempty"`
	DeckID  string   `json:"deckId"`
	NexusID string   `json:"nexusId"`
}
type iconSetManifest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Source      string         `json:"source,omitempty"`
	Author      string         `json:"author,omitempty"`
	Version     string         `json:"version,omitempty"`
	Description string         `json:"description,omitempty"`
	Icons       []iconSetEntry `json:"icons"`
}

func iconSetsDir(root string) string { return filepath.Join(root, "database", "icon-sets") }
func iconSetManifestPath(root, id string) (string, error) {
	if !iconSetIDPattern.MatchString(id) {
		return "", errors.New("invalid icon set ID")
	}
	return filepath.Join(iconSetsDir(root), id+".json"), nil
}
func readIconSetManifest(root, id string) (iconSetManifest, error) {
	file, err := iconSetManifestPath(root, id)
	if err != nil {
		return iconSetManifest{}, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return iconSetManifest{}, err
	}
	if len(data) > 512*1024 {
		return iconSetManifest{}, errors.New("icon set manifest too large")
	}
	var result iconSetManifest
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.ID != id || len(result.Icons) > maxIconSetImages {
		return iconSetManifest{}, errors.New("invalid icon set manifest")
	}
	return result, nil
}
func listIconSets(root string) ([]iconSetManifest, error) {
	entries, err := os.ReadDir(iconSetsDir(root))
	if os.IsNotExist(err) {
		return []iconSetManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > maxIconSets+8 {
		return nil, errors.New("icon set library directory too large")
	}
	sets := []iconSetManifest{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !iconSetIDPattern.MatchString(id) {
			continue
		}
		manifest, err := readIconSetManifest(root, id)
		if err != nil {
			continue
		}
		sets = append(sets, manifest)
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Name < sets[j].Name })
	return sets, nil
}
func cleanIconSetName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 60 {
		return "", errors.New("set name must be 1-60 characters")
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return "", errors.New("invalid icon set name")
		}
	}
	return name, nil
}

// nextDefaultIconSetName is called while iconSetImportMu is held, so two
// simultaneous imports cannot choose the same generated name. Reuse an
// available number if a previously imported default-named pack was removed.
func nextDefaultIconSetName(root string) (string, error) {
	sets, err := listIconSets(root)
	if err != nil {
		return "", err
	}
	used := make(map[string]bool, len(sets))
	for _, set := range sets {
		used[strings.ToLower(set.Name)] = true
	}
	for number := 1; number <= maxIconSets+1; number++ {
		name := fmt.Sprintf("NewPack%d", number)
		if !used[strings.ToLower(name)] {
			return name, nil
		}
	}
	return "", errors.New("no available automatic icon pack names")
}

type pendingIcon struct {
	Name string
	Tags []string
	Data []byte
}

// A ZIP is never unpacked to paths. All member names are validated and the
// decoded raster images go through the existing per-device storage pipelines.
func importIconSet(root, name string, data []byte) (iconSetManifest, error) {
	iconSetImportMu.Lock()
	defer iconSetImportMu.Unlock()
	var empty iconSetManifest
	var err error
	if len(data) == 0 || len(data) > maxIconSetZipBytes {
		return empty, fmt.Errorf("ZIP must be 1-%d MiB", maxIconSetZipBytes>>20)
	}
	digest := sha256.Sum256(data)
	id := hex.EncodeToString(digest[:12])
	if existing, err := readIconSetManifest(root, id); err == nil {
		return existing, nil
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return empty, fmt.Errorf("invalid ZIP: %w", err)
	}
	if len(archive.File) > maxIconSetArchiveEntries {
		return empty, errors.New("too many files in icon set archive")
	}
	pending := []pendingIcon{}
	var importedStreamDeck *streamDeckPack
	if len(archive.File) >= 2 {
		hasManifest, hasIndex := false, false
		for _, member := range archive.File {
			if member.Name == "manifest.json" {
				hasManifest = true
			}
			if member.Name == "icons.json" {
				hasIndex = true
			}
		}
		if hasManifest || hasIndex {
			pack, parseErr := parseStreamDeckPack(archive)
			if parseErr != nil {
				return empty, parseErr
			}
			importedStreamDeck = &pack
			pending = pack.Icons
			if strings.TrimSpace(name) == "" {
				name = pack.Manifest.Name
			}
		}
	}
	// Stream Deck packs supply their name in manifest.json. Legacy image-only
	// ZIPs without a supplied name receive a unique NewPackN after validation.
	if name != "" {
		name, err = cleanIconSetName(name)
		if err != nil {
			return empty, err
		}
	}
	countBytes := 0
	names := map[string]bool{}
	if importedStreamDeck == nil {
		for _, file := range archive.File {
			if file.FileInfo().IsDir() {
				continue
			}
			filename := file.Name
			if strings.Contains(filename, "\\") || strings.HasPrefix(filename, "/") {
				return empty, errors.New("invalid ZIP path")
			}
			for _, segment := range strings.Split(filename, "/") {
				if segment == ".." || segment == "." {
					return empty, errors.New("ZIP path traversal is not permitted")
				}
			}
			if !file.Mode().IsRegular() {
				return empty, errors.New("ZIP must contain regular files")
			}
			base := path.Base(filename)
			if strings.HasPrefix(base, ".") || strings.HasPrefix(filename, "__MACOSX/") {
				continue
			}
			extension := strings.ToLower(path.Ext(base))
			if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".svg" {
				return empty, fmt.Errorf("unsupported icon file %q: use PNG, JPEG or SVG", base)
			}
			if len(pending) >= maxIconSetImages {
				return empty, fmt.Errorf("icon set limited to %d icons", maxIconSetImages)
			}
			if file.UncompressedSize64 > deck.MaxDeckIconBytes || countBytes > maxIconSetTotalImageBytes {
				return empty, errors.New("icon set images exceed quota")
			}
			reader, err := file.Open()
			if err != nil {
				return empty, err
			}
			contents, readErr := io.ReadAll(io.LimitReader(reader, deck.MaxDeckIconBytes+1))
			closeErr := reader.Close()
			if readErr != nil {
				return empty, readErr
			}
			if closeErr != nil {
				return empty, closeErr
			}
			if len(contents) == 0 || len(contents) > deck.MaxDeckIconBytes {
				return empty, errors.New("icon exceeds 512 KiB")
			}
			countBytes += len(contents)
			if countBytes > maxIconSetTotalImageBytes {
				return empty, errors.New("icon set extracted files exceed quota")
			}
			if extension == ".svg" {
				contents, err = rasterizeIconSVG(contents)
				if err != nil {
					return empty, fmt.Errorf("invalid SVG %q: %w", base, err)
				}
			} else {
				info, format, err := image.DecodeConfig(bytes.NewReader(contents))
				if err != nil || !((extension == ".png" && format == "png") || (extension != ".png" && format == "jpeg")) || info.Width < 1 || info.Height < 1 || info.Width > 512 || info.Height > 512 {
					return empty, fmt.Errorf("invalid icon image %q", base)
				}
			}
			title := strings.TrimSpace(strings.TrimSuffix(base, path.Ext(base)))
			if title == "" || utf8.RuneCountInString(title) > 50 || !utf8.ValidString(title) {
				return empty, errors.New("invalid icon title")
			}
			for _, char := range title {
				if unicode.IsControl(char) {
					return empty, errors.New("icon title contains control characters")
				}
			}
			if names[title] {
				return empty, fmt.Errorf("duplicate icon title %q", title)
			}
			names[title] = true
			pending = append(pending, pendingIcon{Name: title, Data: contents})
		}
	}
	if len(pending) == 0 {
		return empty, errors.New("ZIP has no valid icon images")
	}
	dir := iconSetsDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return empty, err
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	if count >= maxIconSets {
		return empty, errors.New("icon set library limited to 24 sets")
	}
	if name == "" {
		name, err = nextDefaultIconSetName(root)
		if err != nil {
			return empty, err
		}
	}
	manifest := iconSetManifest{ID: id, Name: name, Icons: []iconSetEntry{}}
	if importedStreamDeck != nil {
		manifest.Source = "streamdeck"
		manifest.Author = importedStreamDeck.Manifest.Author
		manifest.Version = importedStreamDeck.Manifest.Version
		manifest.Description = importedStreamDeck.Manifest.Description
	}
	for _, icon := range pending {
		deckID, err := deck.SaveDeckIcon(root, icon.Data)
		if err != nil {
			return empty, err
		}
		hash := sha256.Sum256(icon.Data)
		nexusID := "set_" + hex.EncodeToString(hash[:12])
		if _, err := nexus.SaveCustomIcon(filepath.Join(root, "database", "nexus", "icons"), nexusID, icon.Data); err != nil {
			return empty, err
		}
		manifest.Icons = append(manifest.Icons, iconSetEntry{Name: icon.Name, Tags: icon.Tags, DeckID: deckID, NexusID: nexusID})
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return empty, err
	}
	blob, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return empty, err
	}
	temporary, err := os.CreateTemp(dir, ".icon-set-*")
	if err != nil {
		return empty, err
	}
	defer os.Remove(temporary.Name())
	if err = temporary.Chmod(0600); err != nil {
		temporary.Close()
		return empty, err
	}
	if _, err = temporary.Write(blob); err != nil {
		temporary.Close()
		return empty, err
	}
	if err = temporary.Sync(); err != nil {
		temporary.Close()
		return empty, err
	}
	if err = temporary.Close(); err != nil {
		return empty, err
	}
	destination, _ := iconSetManifestPath(root, id)
	if err = os.Rename(temporary.Name(), destination); err != nil {
		return empty, err
	}
	return manifest, nil
}

func renameIconSet(root, id, newName string) (iconSetManifest, error) {
	iconSetImportMu.Lock()
	defer iconSetImportMu.Unlock()
	name, err := cleanIconSetName(newName)
	if err != nil {
		return iconSetManifest{}, err
	}
	current, err := readIconSetManifest(root, id)
	if err != nil {
		return iconSetManifest{}, err
	}
	current.Name = name
	blob, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return iconSetManifest{}, err
	}
	directory := iconSetsDir(root)
	temp, err := os.CreateTemp(directory, ".icon-set-rename-*")
	if err != nil {
		return iconSetManifest{}, err
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(0600); err != nil {
		temp.Close()
		return iconSetManifest{}, err
	}
	if _, err = temp.Write(blob); err != nil {
		temp.Close()
		return iconSetManifest{}, err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return iconSetManifest{}, err
	}
	if err = temp.Close(); err != nil {
		return iconSetManifest{}, err
	}
	target, _ := iconSetManifestPath(root, id)
	if err = os.Rename(temp.Name(), target); err != nil {
		return iconSetManifest{}, err
	}
	return current, nil
}
func iconSetPreviewAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !nexusRequestHostAllowed(r) {
		http.Error(w, "host not allowed", http.StatusForbidden)
		return
	}
	root := config.GetConfig().ConfigPath
	if root == "" {
		http.Error(w, "config unavailable", http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	manifest, err := readIconSetManifest(root, id)
	if err != nil {
		http.Error(w, "icon set unavailable", http.StatusNotFound)
		return
	}
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil || index < 0 || index >= len(manifest.Icons) {
		http.Error(w, "icon unavailable", http.StatusNotFound)
		return
	}
	icon, err := deck.ReadDeckIcon(root, manifest.Icons[index].DeckID)
	if err != nil {
		http.Error(w, "icon unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(icon)
}

func iconSetsAPI(w http.ResponseWriter, r *http.Request) {
	if !nexusRequestHostAllowed(r) {
		http.Error(w, "host not allowed", http.StatusForbidden)
		return
	}
	root := config.GetConfig().ConfigPath
	if root == "" {
		http.Error(w, "config unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch r.Method {
	case http.MethodGet:
		sets, err := listIconSets(root)
		if err != nil {
			http.Error(w, "icon sets unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"sets": sets, "svgAvailable": svgConverterAvailable()})
	case http.MethodPost:
		if !nexusWriteAllowed(r) {
			http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/zip" {
			http.Error(w, "ZIP required", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxIconSetZipBytes)
		defer r.Body.Close()
		contents, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid or oversized ZIP", http.StatusBadRequest)
			return
		}
		sets, err := importIconSetBundle(root, r.URL.Query().Get("name"), contents)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"sets": sets})
	case http.MethodPut:
		if !nexusWriteAllowed(r) {
			http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()
		var request struct {
			Name string `json:"name"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&request); err != nil {
			http.Error(w, "invalid pack name", http.StatusBadRequest)
			return
		}
		renamed, err := renameIconSet(root, r.URL.Query().Get("id"), request.Name)
		if err != nil {
			http.Error(w, "unable to rename pack: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"set": renamed})
	case http.MethodDelete:
		if !nexusWriteAllowed(r) {
			http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
			return
		}
		id := r.URL.Query().Get("id")
		dest, err := iconSetManifestPath(root, id)
		if err != nil {
			http.Error(w, "invalid icon set ID", http.StatusBadRequest)
			return
		}
		iconSetImportMu.Lock()
		err = os.Remove(dest)
		iconSetImportMu.Unlock()
		if err != nil {
			http.Error(w, "icon set not found", http.StatusNotFound)
			return
		}
		// Imported content-addressed images remain for existing saved layouts.
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

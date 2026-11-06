// Package pluginhost runs Stream Deck SDK and OpenAction action plugins for
// Deck keys. It runs inside the desktop agent, as the logged-in user, and
// never inside the OpenLinkHub service.
package pluginhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"OpenLinkHub/src/deckplugin"
)

const maxManifestBytes = 1 << 20

type manifestState struct {
	Image string `json:"Image"`
	Title string `json:"Title"`
}

type manifestAction struct {
	UUID        string          `json:"UUID"`
	Name        string          `json:"Name"`
	Tooltip     string          `json:"Tooltip"`
	Icon        string          `json:"Icon"`
	States      []manifestState `json:"States"`
	Controllers []string        `json:"Controllers"`
	Inspector   string          `json:"PropertyInspectorPath"`
}

// manifest holds the fields we use from a Stream Deck SDK manifest and the
// OpenAction extensions (CodePathLin, CodePaths, DeviceNamespace).
type manifest struct {
	UUID            string            `json:"UUID"`
	PluginUUID      string            `json:"PluginUUID"`
	Name            string            `json:"Name"`
	Version         string            `json:"Version"`
	Author          string            `json:"Author"`
	CodePath        string            `json:"CodePath"`
	CodePathLin     string            `json:"CodePathLin"`
	CodePaths       map[string]string `json:"CodePaths"`
	DeviceNamespace string            `json:"DeviceNamespace"`
	Inspector       string            `json:"PropertyInspectorPath"`
	Actions         []manifestAction  `json:"Actions"`
}

// Action is a key action a plugin offers.
type Action struct {
	UUID    string
	Name    string
	Tooltip string
	States  []manifestState
	// Inspector is the action's settings page (property inspector),
	// relative to the plugin folder in slash form, or "" for none.
	Inspector string
}

// Plugin is one installed plugin folder.
type Plugin struct {
	UUID    string
	Name    string
	Version string
	Author  string
	Dir     string
	// Command is the program and arguments to start, before the
	// Stream Deck launch arguments.
	Command []string
	Actions []Action
	// Err explains why an installed plugin cannot run.
	Err error
	// stamp changes when the manifest is rewritten (reinstall or edit), so
	// a running copy can be restarted.
	stamp string
}

func (p *Plugin) action(uuid string) (Action, bool) {
	for _, a := range p.Actions {
		if a.UUID == uuid {
			return a, true
		}
	}
	return Action{}, false
}

// ErrDevicePlugin marks hardware plugins, which OpenLinkHub refuses because
// it drives Deck hardware itself and two drivers would fight over a device.
var ErrDevicePlugin = errors.New("hardware (device) plugins are not supported; OpenLinkHub drives Deck hardware itself")

func targetTriple() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64-unknown-linux-gnu"
	case "arm64":
		return "aarch64-unknown-linux-gnu"
	}
	return runtime.GOARCH + "-unknown-linux-gnu"
}

// pluginPath resolves a manifest-relative path and refuses anything that
// leaves the plugin folder, including through symlinks.
func pluginPath(dir, rel string) (string, error) {
	rel = filepath.FromSlash(strings.TrimSpace(rel))
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid plugin path %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("plugin path %q leaves the plugin folder", rel)
	}
	full := filepath.Join(dir, clean)
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return "", fmt.Errorf("plugin path %q leaves the plugin folder", rel)
	}
	return resolved, nil
}

// inspectorPath keeps a settings page only if it is an HTML file inside
// the plugin folder.
func inspectorPath(dir, rel string) string {
	clean := deckplugin.CleanInspectorPath(rel)
	ext := strings.ToLower(path.Ext(clean))
	if clean == "" || (ext != ".html" && ext != ".htm") {
		return ""
	}
	full, err := pluginPath(dir, clean)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(full); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return clean
}

// ReadPluginFile reads one file of a plugin's settings page.
func ReadPluginFile(dir, rel string) ([]byte, error) {
	clean := deckplugin.CleanInspectorPath(rel)
	if clean == "" {
		return nil, errors.New("invalid file path")
	}
	full, err := pluginPath(dir, clean)
	if err != nil {
		return nil, errors.New("file not found")
	}
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("file not found")
	}
	if info.Size() > deckplugin.MaxFileBytes {
		return nil, errors.New("file too large")
	}
	data, err := os.ReadFile(full)
	if err != nil || len(data) > deckplugin.MaxFileBytes {
		return nil, errors.New("file not found")
	}
	return data, nil
}

// resolveCommand picks how to start the plugin on Linux: an OpenAction
// native binary first, then a Node.js script.
func resolveCommand(dir string, m manifest, lookPath func(string) (string, error)) ([]string, error) {
	code := m.CodePaths[targetTriple()]
	if code == "" {
		code = m.CodePathLin
	}
	if code == "" {
		code = m.CodePath
	}
	if code == "" {
		return nil, errors.New("manifest has no CodePath")
	}
	path, err := pluginPath(dir, code)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("plugin code %q not found", code)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs":
		node, err := lookPath("node")
		if err != nil {
			return nil, errors.New("this plugin needs Node.js; install the nodejs package")
		}
		return []string{node, path}, nil
	case ".html", ".htm":
		return nil, errors.New("HTML-page plugins are not supported yet")
	case ".exe":
		return nil, errors.New("Windows-only plugin; there is no Linux build in this package")
	}
	if info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("plugin binary %q is not executable", code)
	}
	if !linuxExecutable(path) {
		return nil, errors.New("plugin has no Linux build (its program is for another system)")
	}
	return []string{path}, nil
}

// LoadPlugin reads one *.sdPlugin folder. A plugin that is installed but
// cannot run returns a Plugin with Err set, so the editor can say why.
func LoadPlugin(dir string) (*Plugin, error) {
	return loadPlugin(dir, exec.LookPath)
}

func loadPlugin(dir string, lookPath func(string) (string, error)) (*Plugin, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if len(data) > maxManifestBytes {
		return nil, errors.New("manifest too large")
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	uuid := m.UUID
	if uuid == "" {
		uuid = m.PluginUUID
	}
	if uuid == "" {
		uuid = strings.TrimSuffix(filepath.Base(dir), ".sdPlugin")
	}
	if !deckplugin.ValidID(uuid) {
		return nil, fmt.Errorf("invalid plugin UUID %q", uuid)
	}
	p := &Plugin{UUID: uuid, Name: limitText(m.Name, 64), Version: limitText(m.Version, 32),
		Author: limitText(m.Author, 64), Dir: dir}
	if info, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
		p.stamp = fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	}
	if p.Name == "" {
		p.Name = uuid
	}
	if m.DeviceNamespace != "" {
		p.Err = ErrDevicePlugin
		return p, nil
	}
	for _, a := range m.Actions {
		if !deckplugin.ValidID(a.UUID) || !keypadAction(a.Controllers) {
			continue
		}
		name := limitText(a.Name, 64)
		if name == "" {
			name = a.UUID
		}
		inspector := a.Inspector
		if inspector == "" {
			inspector = m.Inspector
		}
		p.Actions = append(p.Actions, Action{UUID: a.UUID, Name: name, Tooltip: limitText(a.Tooltip, 160), States: a.States,
			Inspector: inspectorPath(dir, inspector)})
		if len(p.Actions) == deckplugin.MaxCatalogItems {
			break
		}
	}
	if len(p.Actions) == 0 {
		p.Err = errors.New("plugin has no key actions")
		return p, nil
	}
	p.Command, p.Err = resolveCommand(dir, m, lookPath)
	return p, nil
}

// linuxExecutable accepts ELF binaries and scripts with a #! line.
func linuxExecutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 4)
	n, _ := f.Read(head)
	return n >= 2 && (string(head[:2]) == "#!" || (n == 4 && string(head) == "\x7fELF"))
}

func keypadAction(controllers []string) bool {
	if len(controllers) == 0 {
		return true
	}
	for _, c := range controllers {
		if c == "Keypad" {
			return true
		}
	}
	return false
}

func limitText(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// ScanPlugins loads every *.sdPlugin folder in dir, sorted by name.
func ScanPlugins(dir string) []*Plugin {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []*Plugin
	seen := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), ".sdPlugin") {
			continue
		}
		p, err := LoadPlugin(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Deck plugin %s skipped: %v\n", e.Name(), err)
			continue
		}
		if seen[p.UUID] {
			continue
		}
		seen[p.UUID] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func catalog(plugins []*Plugin) []deckplugin.CatalogPlugin {
	out := make([]deckplugin.CatalogPlugin, 0, len(plugins))
	for _, p := range plugins {
		entry := deckplugin.CatalogPlugin{UUID: p.UUID, Name: p.Name, Version: p.Version, Author: p.Author,
			Actions: []deckplugin.CatalogAction{}}
		if p.Err != nil {
			entry.Error = p.Err.Error()
		}
		for _, a := range p.Actions {
			entry.Actions = append(entry.Actions, deckplugin.CatalogAction{UUID: a.UUID, Name: a.Name, Tooltip: a.Tooltip,
				Inspector: a.Inspector})
		}
		out = append(out, entry)
		if len(out) == deckplugin.MaxCatalogItems {
			break
		}
	}
	return out
}

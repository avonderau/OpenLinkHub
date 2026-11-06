package pluginhost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"OpenLinkHub/src/deckplugin"

	"github.com/gorilla/websocket"
)

const (
	registerEvent      = "registerPlugin"
	maxQueuedEvents    = 512
	maxPluginFrame     = 4 << 20
	maxGlobalSettings  = 64 << 10
	registrationWindow = 15 * time.Second
	maxRestarts        = 5
	restartWindow      = 2 * time.Minute
)

// Host runs plugin processes and speaks the Stream Deck SDK WebSocket
// protocol to them on 127.0.0.1. Each launch gets a random registration
// token, and connections carrying a browser Origin header are refused, so
// only processes we started can attach.
type Host struct {
	mu        sync.Mutex
	plugins   map[string]*Plugin
	devices   []deckplugin.Device
	instances map[string]*instance
	running   map[string]*process
	byToken   map[string]*process
	send      func(deckplugin.Message)
	listener  net.Listener
	server    *http.Server
	port      int
	stateDir  string
	stopped   bool
	images    *imageLoader
	launch    func(*process) error
}

type instance struct {
	deckplugin.Instance
	customImage bool
	customTitle bool
}

type process struct {
	plugin   *Plugin
	token    string
	cmd      *exec.Cmd
	connMu   sync.Mutex
	conn     *websocket.Conn
	queue    [][]byte
	restarts []time.Time
	failed   bool
}

// NewHost prepares a host. send delivers key updates to the service; it
// must not block for long.
func NewHost(stateDir string, send func(deckplugin.Message)) *Host {
	h := &Host{plugins: map[string]*Plugin{}, instances: map[string]*instance{},
		running: map[string]*process{}, byToken: map[string]*process{},
		send: send, stateDir: stateDir, images: newImageLoader()}
	h.launch = h.startProcess
	return h
}

// Start opens the plugin WebSocket listener.
func (h *Host) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("plugin WebSocket listener: %w", err)
	}
	h.listener = listener
	h.port = listener.Addr().(*net.TCPAddr).Port
	h.server = &http.Server{Handler: http.HandlerFunc(h.serveWebSocket), ReadHeaderTimeout: 5 * time.Second}
	go h.server.Serve(listener)
	return nil
}

// Stop ends every plugin process and closes the listener.
func (h *Host) Stop() {
	h.mu.Lock()
	h.stopped = true
	procs := make([]*process, 0, len(h.running))
	for _, p := range h.running {
		procs = append(procs, p)
	}
	h.running = map[string]*process{}
	h.byToken = map[string]*process{}
	h.mu.Unlock()
	for _, p := range procs {
		p.kill()
	}
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		h.server.Shutdown(ctx)
		cancel()
	}
}

// SetPlugins replaces the installed plugin list after a rescan.
func (h *Host) SetPlugins(plugins []*Plugin) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.plugins = map[string]*Plugin{}
	for _, p := range plugins {
		h.plugins[p.UUID] = p
	}
}

// Reset forgets the service's devices and keys, for example when the
// service restarts. Running plugins are told the keys disappeared.
func (h *Host) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ctx, inst := range h.instances {
		h.notifyLocked(inst, "willDisappear", nil)
		delete(h.instances, ctx)
	}
	for _, d := range h.devices {
		for _, p := range h.running {
			p.sendJSON(map[string]any{"event": "deviceDidDisconnect", "device": d.ID})
		}
	}
	h.devices = nil
}

// HandleServiceMessage applies one message from the OpenLinkHub service.
func (h *Host) HandleServiceMessage(msg deckplugin.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	switch msg.Type {
	case deckplugin.TypeDevices:
		old := map[string]bool{}
		for _, d := range h.devices {
			old[d.ID] = true
		}
		h.devices = msg.Devices
		for _, d := range msg.Devices {
			if !old[d.ID] {
				for _, p := range h.running {
					p.sendJSON(deviceDidConnect(d))
				}
			}
			delete(old, d.ID)
		}
		for id := range old {
			for _, p := range h.running {
				p.sendJSON(map[string]any{"event": "deviceDidDisconnect", "device": id})
			}
		}
	case deckplugin.TypeAppear:
		for _, in := range msg.Instances {
			h.appearLocked(in)
		}
	case deckplugin.TypeDisappear:
		if inst := h.instances[msg.Context]; inst != nil {
			h.notifyLocked(inst, "willDisappear", nil)
			delete(h.instances, msg.Context)
		}
	case deckplugin.TypeKeyDown, deckplugin.TypeKeyUp:
		if inst := h.instances[msg.Context]; inst != nil {
			h.notifyLocked(inst, msg.Type, nil)
		}
	}
}

func (h *Host) appearLocked(in deckplugin.Instance) {
	if !deckplugin.ValidContext(in.Context) || deckplugin.ValidSettings(in.Settings) != nil {
		return
	}
	plugin := h.plugins[in.Plugin]
	if plugin == nil {
		h.sendTitle(in.Context, "Plugin missing")
		return
	}
	action, ok := plugin.action(in.Action)
	if !ok || plugin.Err != nil {
		h.sendTitle(in.Context, "Unavailable")
		return
	}
	if existing := h.instances[in.Context]; existing != nil {
		h.notifyLocked(existing, "willDisappear", nil)
	}
	if in.State < 0 || in.State >= max(1, len(action.States)) {
		in.State = 0
	}
	inst := &instance{Instance: in}
	h.instances[in.Context] = inst
	h.sendDefaultsLocked(plugin, action, inst)
	if p := h.running[plugin.UUID]; p != nil {
		h.notifyLocked(inst, "willAppear", nil)
		return
	}
	h.startLocked(plugin)
}

// startLocked launches a plugin and queues willAppear for its keys, which
// are delivered as soon as it registers.
func (h *Host) startLocked(plugin *Plugin) {
	if plugin.Command == nil {
		return
	}
	token, err := randomToken()
	if err != nil {
		return
	}
	p := &process{plugin: plugin, token: token}
	if old := h.running[plugin.UUID]; old != nil {
		p.restarts = old.restarts
	}
	h.running[plugin.UUID] = p
	h.byToken[token] = p
	for _, inst := range h.instances {
		if inst.Plugin == plugin.UUID {
			p.sendJSON(instanceEvent(inst, "willAppear", nil))
		}
	}
	if err := h.launch(p); err != nil {
		fmt.Fprintf(os.Stderr, "Deck plugin %s failed to start: %v\n", plugin.UUID, err)
		delete(h.running, plugin.UUID)
		delete(h.byToken, token)
		for _, inst := range h.instances {
			if inst.Plugin == plugin.UUID {
				h.sendTitle(inst.Context, "Plugin error")
			}
		}
	}
}

func (h *Host) info(plugin *Plugin) string {
	devices := make([]map[string]any, 0, len(h.devices))
	for _, d := range h.devices {
		devices = append(devices, map[string]any{"id": d.ID, "name": d.Name, "type": d.Type,
			"size": map[string]int{"columns": d.Columns, "rows": d.Rows}})
	}
	data, _ := json.Marshal(map[string]any{
		"application": map[string]any{"font": "", "language": "en", "platform": "linux",
			"platformVersion": "", "version": "6.9.0"},
		"plugin":           map[string]string{"uuid": plugin.UUID, "version": plugin.Version},
		"devicePixelRatio": 1,
		"colors":           map[string]string{},
		"devices":          devices,
	})
	return string(data)
}

func (h *Host) startProcess(p *process) error {
	logDir := filepath.Join(h.stateDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, p.plugin.UUID+".log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	args := append([]string{}, p.plugin.Command[1:]...)
	args = append(args, "-port", strconv.Itoa(h.port), "-pluginUUID", p.token,
		"-registerEvent", registerEvent, "-info", h.info(p.plugin))
	cmd := exec.Command(p.plugin.Command[0], args...)
	cmd.Dir = p.plugin.Dir
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	p.cmd = cmd
	go func() {
		err := cmd.Wait()
		logFile.Close()
		h.processExited(p, err)
	}()
	return nil
}

func (h *Host) processExited(p *process, err error) {
	p.closeConn()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running[p.plugin.UUID] != p || h.stopped {
		return
	}
	delete(h.running, p.plugin.UUID)
	delete(h.byToken, p.token)
	fmt.Fprintf(os.Stderr, "Deck plugin %s exited: %v\n", p.plugin.UUID, err)
	now := time.Now()
	recent := p.restarts[:0]
	for _, t := range p.restarts {
		if now.Sub(t) < restartWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= maxRestarts {
		for _, inst := range h.instances {
			if inst.Plugin == p.plugin.UUID {
				h.sendTitle(inst.Context, "Plugin crashed")
			}
		}
		return
	}
	p.restarts = append(recent, now)
	delay := time.Duration(1<<len(recent)) * time.Second
	h.running[p.plugin.UUID] = &process{plugin: p.plugin, restarts: p.restarts, failed: true}
	time.AfterFunc(delay, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		placeholder := h.running[p.plugin.UUID]
		if h.stopped || placeholder == nil || !placeholder.failed {
			return
		}
		delete(h.running, p.plugin.UUID)
		for _, inst := range h.instances {
			if inst.Plugin == p.plugin.UUID {
				h.startLocked(p.plugin)
				return
			}
		}
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 << 10,
	WriteBufferSize: 64 << 10,
	// Plugins are local processes, not web pages. A browser always sends
	// an Origin header, so refusing it keeps web pages off this port.
	CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" },
}

func (h *Host) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxPluginFrame)
	conn.SetReadDeadline(time.Now().Add(registrationWindow))
	var reg struct {
		Event string `json:"event"`
		UUID  string `json:"uuid"`
	}
	if err := conn.ReadJSON(&reg); err != nil || reg.Event != registerEvent {
		conn.Close()
		return
	}
	h.mu.Lock()
	p := h.byToken[reg.UUID]
	h.mu.Unlock()
	if p == nil || !p.attach(conn) {
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			p.closeConn()
			return
		}
		h.handlePluginEvent(p, data)
	}
}

type pluginEvent struct {
	Event   string          `json:"event"`
	Context string          `json:"context"`
	Payload json.RawMessage `json:"payload"`
}

func (h *Host) handlePluginEvent(p *process, data []byte) {
	var ev pluginEvent
	if json.Unmarshal(data, &ev) != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running[p.plugin.UUID] != p {
		return
	}
	switch ev.Event {
	case "setGlobalSettings":
		h.saveGlobalSettings(p.plugin, ev.Payload)
		return
	case "getGlobalSettings":
		p.sendJSON(map[string]any{"event": "didReceiveGlobalSettings",
			"payload": map[string]json.RawMessage{"settings": h.loadGlobalSettings(p.plugin)}})
		return
	case "openUrl":
		var payload struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(ev.Payload, &payload) == nil {
			go openURL(payload.URL)
		}
		return
	case "logMessage":
		var payload struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(ev.Payload, &payload) == nil {
			fmt.Fprintf(os.Stderr, "Deck plugin %s: %s\n", p.plugin.UUID, limitText(payload.Message, 512))
		}
		return
	}
	// Everything else targets one key, which must belong to this plugin.
	inst := h.instances[ev.Context]
	if inst == nil || inst.Plugin != p.plugin.UUID {
		return
	}
	action, _ := p.plugin.action(inst.Action)
	switch ev.Event {
	case "setTitle":
		var payload struct {
			Title *string `json:"title"`
			State *int    `json:"state"`
		}
		if json.Unmarshal(ev.Payload, &payload) != nil || (payload.State != nil && *payload.State != inst.State) {
			return
		}
		if payload.Title == nil || *payload.Title == "" {
			inst.customTitle = false
			h.sendTitle(inst.Context, stateTitle(action, inst.State))
			return
		}
		inst.customTitle = true
		h.sendTitle(inst.Context, limitText(*payload.Title, deckplugin.MaxTitleRunes))
	case "setImage":
		var payload struct {
			Image string `json:"image"`
			State *int   `json:"state"`
		}
		if json.Unmarshal(ev.Payload, &payload) != nil || (payload.State != nil && *payload.State != inst.State) {
			return
		}
		if payload.Image == "" {
			inst.customImage = false
			h.sendDefaultImageLocked(p.plugin, action, inst)
			return
		}
		image, err := decodeDataURL(payload.Image, h.images.svg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Deck plugin %s setImage: %v\n", p.plugin.UUID, err)
			return
		}
		inst.customImage = true
		h.send(deckplugin.Message{Type: deckplugin.TypeImage, Context: inst.Context, Image: image})
	case "setState":
		var payload struct {
			State int `json:"state"`
		}
		if json.Unmarshal(ev.Payload, &payload) != nil || payload.State < 0 || payload.State >= max(1, len(action.States)) {
			return
		}
		inst.State = payload.State
		h.send(deckplugin.Message{Type: deckplugin.TypeState, Context: inst.Context, State: inst.State})
		h.sendDefaultsLocked(p.plugin, action, inst)
	case "setSettings":
		if deckplugin.ValidSettings(ev.Payload) != nil {
			return
		}
		inst.Settings = append(json.RawMessage(nil), ev.Payload...)
		h.send(deckplugin.Message{Type: deckplugin.TypeSettings, Context: inst.Context, Settings: inst.Settings})
	case "getSettings":
		p.sendJSON(instanceEvent(inst, "didReceiveSettings", nil))
	case "showAlert":
		h.send(deckplugin.Message{Type: deckplugin.TypeAlert, Context: inst.Context})
	case "showOk":
		h.send(deckplugin.Message{Type: deckplugin.TypeOK, Context: inst.Context})
	}
}

func (h *Host) sendTitle(ctx, title string) {
	h.send(deckplugin.Message{Type: deckplugin.TypeTitle, Context: ctx, Title: &title})
}

func stateTitle(action Action, state int) string {
	if state >= 0 && state < len(action.States) {
		return limitText(action.States[state].Title, deckplugin.MaxTitleRunes)
	}
	return ""
}

// sendDefaultsLocked shows the manifest's image and title for the key's
// current state, unless the plugin has set its own.
func (h *Host) sendDefaultsLocked(plugin *Plugin, action Action, inst *instance) {
	if !inst.customTitle {
		h.sendTitle(inst.Context, stateTitle(action, inst.State))
	}
	if !inst.customImage {
		h.sendDefaultImageLocked(plugin, action, inst)
	}
}

func (h *Host) sendDefaultImageLocked(plugin *Plugin, action Action, inst *instance) {
	var image []byte
	if inst.State < len(action.States) {
		image = h.images.manifestImage(plugin.Dir, action.States[inst.State].Image)
	}
	h.send(deckplugin.Message{Type: deckplugin.TypeImage, Context: inst.Context, Image: image})
}

func (h *Host) notifyLocked(inst *instance, event string, _ any) {
	if p := h.running[inst.Plugin]; p != nil && !p.failed {
		p.sendJSON(instanceEvent(inst, event, nil))
	}
}

func instanceEvent(inst *instance, event string, _ any) map[string]any {
	settings := inst.Settings
	if len(settings) == 0 {
		settings = json.RawMessage("{}")
	}
	return map[string]any{
		"event":   event,
		"action":  inst.Action,
		"context": inst.Context,
		"device":  inst.Device,
		"payload": map[string]any{
			"settings":        settings,
			"coordinates":     map[string]int{"column": inst.Column, "row": inst.Row},
			"state":           inst.State,
			"controller":      "Keypad",
			"isInMultiAction": false,
		},
	}
}

func deviceDidConnect(d deckplugin.Device) map[string]any {
	return map[string]any{"event": "deviceDidConnect", "device": d.ID, "deviceInfo": map[string]any{
		"name": d.Name, "type": d.Type, "size": map[string]int{"columns": d.Columns, "rows": d.Rows}}}
}

func (h *Host) globalSettingsPath(plugin *Plugin) string {
	return filepath.Join(h.stateDir, "global-settings", plugin.UUID+".json")
}

func (h *Host) loadGlobalSettings(plugin *Plugin) json.RawMessage {
	data, err := os.ReadFile(h.globalSettingsPath(plugin))
	if err != nil || len(data) > maxGlobalSettings || !json.Valid(data) {
		return json.RawMessage("{}")
	}
	return data
}

func (h *Host) saveGlobalSettings(plugin *Plugin, payload json.RawMessage) {
	var object map[string]json.RawMessage
	if len(payload) > maxGlobalSettings || json.Unmarshal(payload, &object) != nil {
		return
	}
	path := h.globalSettingsPath(plugin)
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, payload, 0o600) == nil {
		os.Rename(tmp, path)
	}
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (p *process) attach(conn *websocket.Conn) bool {
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.conn != nil {
		return false
	}
	p.conn = conn
	for _, data := range p.queue {
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if conn.WriteMessage(websocket.TextMessage, data) != nil {
			break
		}
	}
	p.queue = nil
	return true
}

// sendJSON writes to the plugin, or queues until it registers.
func (p *process) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.conn == nil {
		if len(p.queue) < maxQueuedEvents {
			p.queue = append(p.queue, data)
		}
		return
	}
	p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if p.conn.WriteMessage(websocket.TextMessage, data) != nil {
		p.conn.Close()
	}
}

func (p *process) closeConn() {
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.conn != nil {
		p.conn.Close()
	}
}

func (p *process) kill() {
	p.closeConn()
	if p.cmd != nil && p.cmd.Process != nil {
		// Signal the whole process group so helpers the plugin spawned exit.
		syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
		time.AfterFunc(2*time.Second, func() { syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL) })
	}
}

var errUnsupportedImage = errors.New("unsupported image")

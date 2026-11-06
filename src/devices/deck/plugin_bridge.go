package deck

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"OpenLinkHub/src/deckplugin"
	"OpenLinkHub/src/devices/nexus"
	"OpenLinkHub/src/logger"
)

// pluginBridge is the service end of the deck-plugins socket. One desktop
// agent may attach at a time. The service only sends key visibility and
// presses; everything the agent sends back is validated before use.
type pluginBridge struct {
	mu       sync.Mutex
	listener net.Listener
	client   net.Conn
	out      chan deckplugin.Message
	catalog  []deckplugin.CatalogPlugin
	// contexts maps each visible plugin key to its device.
	contexts map[string]*Device
	devices  map[*Device]bool
}

var plugins = &pluginBridge{contexts: map[string]*Device{}, devices: map[*Device]bool{}}
var pluginBridgeOnce sync.Once

// StartPluginBridge opens the plugin socket beside the Nexus launch socket.
func StartPluginBridge() {
	pluginBridgeOnce.Do(func() {
		root, err := os.Getwd()
		if err != nil {
			return
		}
		path := filepath.Join(nexus.DesktopAgentSocketDir(root), deckplugin.SocketName)
		listener, err := nexus.ListenDesktopAgentSocket(path)
		if err != nil {
			logger.Log(logger.Fields{"error": err.Error()}).Warn("Deck plugin bridge unavailable")
			return
		}
		plugins.mu.Lock()
		plugins.listener = listener
		plugins.mu.Unlock()
		go plugins.accept()
	})
}

func (b *pluginBridge) accept() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		if b.client != nil {
			b.mu.Unlock()
			conn.Close()
			continue
		}
		b.client = conn
		b.out = make(chan deckplugin.Message, 256)
		out := b.out
		devices := make([]*Device, 0, len(b.devices))
		for d := range b.devices {
			devices = append(devices, d)
		}
		b.mu.Unlock()
		go b.write(conn, out)
		b.enqueue(deckplugin.Message{Type: deckplugin.TypeDevices, Devices: b.deviceList()})
		for _, d := range devices {
			d.resendPluginKeys()
		}
		go b.read(conn)
	}
}

func (b *pluginBridge) write(conn net.Conn, out chan deckplugin.Message) {
	enc := json.NewEncoder(conn)
	for msg := range out {
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if enc.Encode(msg) != nil {
			conn.Close()
			for range out {
			}
			return
		}
	}
}

func (b *pluginBridge) read(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64<<10), deckplugin.MaxMessageBytes)
	for scanner.Scan() {
		var msg deckplugin.Message
		if json.Unmarshal(scanner.Bytes(), &msg) == nil {
			b.handle(msg)
		}
	}
	conn.Close()
	b.mu.Lock()
	if b.client == conn {
		b.client = nil
		close(b.out)
		b.out = nil
		b.catalog = nil
	}
	devices := make([]*Device, 0, len(b.devices))
	for d := range b.devices {
		devices = append(devices, d)
	}
	b.mu.Unlock()
	for _, d := range devices {
		d.clearPluginState("Plugin host offline")
	}
}

// enqueue never blocks a HID reader; when the agent is slow, it drops.
func (b *pluginBridge) enqueue(msg deckplugin.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.out == nil {
		return
	}
	select {
	case b.out <- msg:
	default:
	}
}

func (b *pluginBridge) connected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.client != nil
}

func (b *pluginBridge) handle(msg deckplugin.Message) {
	if msg.Type == deckplugin.TypeCatalog {
		if len(msg.Catalog) > deckplugin.MaxCatalogItems {
			return
		}
		valid := make([]deckplugin.CatalogPlugin, 0, len(msg.Catalog))
		for _, p := range msg.Catalog {
			if !deckplugin.ValidID(p.UUID) || len(p.Actions) > deckplugin.MaxCatalogItems {
				continue
			}
			actions := p.Actions[:0:0]
			for _, a := range p.Actions {
				if deckplugin.ValidID(a.UUID) {
					a.Name, a.Tooltip = trimRunes(a.Name, 64), trimRunes(a.Tooltip, 160)
					actions = append(actions, a)
				}
			}
			p.Actions = actions
			p.Name, p.Version, p.Author, p.Error = trimRunes(p.Name, 64), trimRunes(p.Version, 32), trimRunes(p.Author, 64), trimRunes(p.Error, 200)
			valid = append(valid, p)
		}
		b.mu.Lock()
		b.catalog = valid
		b.mu.Unlock()
		return
	}
	if !deckplugin.ValidContext(msg.Context) {
		return
	}
	b.mu.Lock()
	d := b.contexts[msg.Context]
	b.mu.Unlock()
	if d != nil {
		d.applyPluginMessage(msg)
	}
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// PluginCatalog lists the plugins the connected agent reported.
func PluginCatalog() (bool, []deckplugin.CatalogPlugin) {
	plugins.mu.Lock()
	defer plugins.mu.Unlock()
	return plugins.client != nil, append([]deckplugin.CatalogPlugin(nil), plugins.catalog...)
}

func (b *pluginBridge) deviceList() []deckplugin.Device {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []deckplugin.Device{}
	for d := range b.devices {
		out = append(out, deckplugin.Device{ID: d.Serial, Name: d.Model.Name,
			Rows: d.Model.Rows, Columns: d.Model.Columns, Type: streamDeckType(d.Model)})
	}
	return out
}

// streamDeckType is the Stream Deck SDK device type plugins expect.
func streamDeckType(m Model) int {
	if m.Family == "elgato" {
		switch {
		case m.Rows == 2:
			return 1 // Mini
		case m.Rows == 4:
			return 2 // XL
		}
	}
	return 0 // Stream Deck
}

func (b *pluginBridge) registerDevice(d *Device, present bool) {
	b.mu.Lock()
	if present {
		b.devices[d] = true
	} else {
		delete(b.devices, d)
	}
	b.mu.Unlock()
	b.enqueue(deckplugin.Message{Type: deckplugin.TypeDevices, Devices: b.deviceList()})
}

// pluginKey is what a plugin last told us about one visible key.
type pluginKey struct {
	index    int
	title    string
	hasTitle bool
	image    []byte
	state    int
}

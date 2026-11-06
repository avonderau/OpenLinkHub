package deck

import (
	"bytes"
	"encoding/json"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"unicode/utf8"

	"OpenLinkHub/src/deckplugin"

	xdraw "golang.org/x/image/draw"
)

const maxPluginImagePixels = 1024

// pluginContext is the stable plugin instance ID for a key on a page.
func (d *Device) pluginContext(pageID string, index int) string {
	return deckplugin.ContextID(d.Serial, pageID, index)
}

// visiblePluginInstancesLocked lists the plugin keys on the active page.
// Keys are only visible while the hardware is connected.
func (d *Device) visiblePluginInstancesLocked() map[string]deckplugin.Instance {
	out := map[string]deckplugin.Instance{}
	if !d.connected || d.hidDevice == nil {
		return out
	}
	for _, page := range d.layout.Pages {
		if page.ID != d.layout.ActivePage {
			continue
		}
		for _, key := range page.Keys {
			if key.Action != "plugin" || !d.Model.CanPress(key.Index) {
				continue
			}
			ctx := d.pluginContext(page.ID, key.Index)
			out[ctx] = deckplugin.Instance{Context: ctx, Plugin: key.Plugin, Action: key.PluginAction,
				Device: d.Serial, Row: key.Index / d.Model.Columns, Column: key.Index % d.Model.Columns,
				Settings: compactJSON(key.PluginSettings)}
		}
	}
	return out
}

// compactJSON strips the indentation saved layouts add to settings, so
// unchanged settings compare equal after a reload.
func compactJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return raw
	}
	return buf.Bytes()
}

func sameInstance(a, b deckplugin.Instance) bool {
	return a.Plugin == b.Plugin && a.Action == b.Action && bytes.Equal(a.Settings, b.Settings)
}

// syncPluginKeysLocked tells the plugin host which plugin keys appeared or
// disappeared after a page change, layout save, connect or disconnect.
func (d *Device) syncPluginKeysLocked() {
	next := d.visiblePluginInstancesLocked()
	if d.pluginInstances == nil {
		d.pluginInstances = map[string]deckplugin.Instance{}
		d.pluginKeys = map[string]*pluginKey{}
	}
	var appear []deckplugin.Instance
	for ctx, old := range d.pluginInstances {
		if in, ok := next[ctx]; ok && sameInstance(old, in) {
			continue
		}
		plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeDisappear, Context: ctx})
		delete(d.pluginInstances, ctx)
		delete(d.pluginKeys, ctx)
		plugins.mu.Lock()
		delete(plugins.contexts, ctx)
		plugins.mu.Unlock()
	}
	for ctx, in := range next {
		if _, ok := d.pluginInstances[ctx]; ok {
			continue
		}
		d.pluginInstances[ctx] = in
		d.pluginKeys[ctx] = &pluginKey{index: in.Column + in.Row*d.Model.Columns}
		plugins.mu.Lock()
		plugins.contexts[ctx] = d
		plugins.mu.Unlock()
		appear = append(appear, in)
	}
	if len(appear) > 0 {
		plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeAppear, Instances: appear})
	}
}

// resendPluginKeys replays every visible plugin key to a newly attached
// plugin host.
func (d *Device) resendPluginKeys() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pluginInstances) == 0 {
		return
	}
	list := make([]deckplugin.Instance, 0, len(d.pluginInstances))
	for _, in := range d.pluginInstances {
		list = append(list, in)
	}
	plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeAppear, Instances: list})
}

// clearPluginState drops plugin-provided titles and images, for example
// when the desktop agent disconnects, and redraws those keys.
func (d *Device) clearPluginState(status string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pluginKeys) == 0 {
		return
	}
	for _, key := range d.pluginKeys {
		*key = pluginKey{index: key.index, title: status, hasTitle: true}
	}
	d.redrawPluginKeysLocked()
}

func (d *Device) dropPluginKeysLocked() {
	for ctx := range d.pluginInstances {
		plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeDisappear, Context: ctx})
		plugins.mu.Lock()
		delete(plugins.contexts, ctx)
		plugins.mu.Unlock()
	}
	d.pluginInstances = nil
	d.pluginKeys = nil
}

func (d *Device) redrawPluginKeysLocked() {
	if d.hidDevice == nil || !d.connected {
		return
	}
	drew := false
	for _, page := range d.layout.Pages {
		if page.ID != d.layout.ActivePage {
			continue
		}
		for _, key := range page.Keys {
			if key.Action != "plugin" {
				continue
			}
			if d.drawKeyLocked(key.Index, key) == nil {
				drew = true
			}
		}
	}
	if drew {
		finishHardwareFrame(d.hidDevice, d.Model)
	}
}

func (d *Device) redrawPluginKeyLocked(ctx string) {
	pk := d.pluginKeys[ctx]
	if pk == nil || d.hidDevice == nil || !d.connected {
		return
	}
	for _, page := range d.layout.Pages {
		if page.ID != d.layout.ActivePage {
			continue
		}
		for _, key := range page.Keys {
			if key.Index == pk.index && key.Action == "plugin" {
				if err := d.drawKeyLocked(key.Index, key); err == nil {
					finishHardwareFrame(d.hidDevice, d.Model)
				}
				return
			}
		}
	}
}

// decodePluginImage accepts only bounded PNG or JPEG from the agent.
func decodePluginImage(data []byte) (image.Image, bool) {
	if len(data) == 0 || len(data) > deckplugin.MaxImageBytes {
		return nil, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 ||
		cfg.Width > maxPluginImagePixels || cfg.Height > maxPluginImagePixels {
		return nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err == nil
}

// applyPluginMessage handles one validated update from the plugin host.
func (d *Device) applyPluginMessage(msg deckplugin.Message) {
	d.mu.Lock()
	defer d.mu.Unlock()
	pk := d.pluginKeys[msg.Context]
	if pk == nil {
		return
	}
	switch msg.Type {
	case deckplugin.TypeTitle:
		if msg.Title == nil || utf8.RuneCountInString(*msg.Title) > deckplugin.MaxTitleRunes {
			return
		}
		pk.title, pk.hasTitle = *msg.Title, true
	case deckplugin.TypeImage:
		if len(msg.Image) == 0 {
			pk.image = nil
		} else if _, ok := decodePluginImage(msg.Image); ok {
			pk.image = msg.Image
		} else {
			return
		}
	case deckplugin.TypeState:
		if msg.State < 0 || msg.State > 31 {
			return
		}
		pk.state = msg.State
		return
	case deckplugin.TypeSettings:
		d.savePluginSettingsLocked(msg.Context, msg.Settings)
		return
	case deckplugin.TypeAlert:
		d.status = "A plugin key reported an error"
		return
	case deckplugin.TypeOK:
		return
	default:
		return
	}
	d.redrawPluginKeyLocked(msg.Context)
}

// savePluginSettingsLocked stores settings a plugin set on its key.
func (d *Device) savePluginSettingsLocked(ctx string, settings json.RawMessage) {
	if deckplugin.ValidSettings(settings) != nil {
		return
	}
	in, ok := d.pluginInstances[ctx]
	if !ok {
		return
	}
	updated := cloneLayout(d.layout)
	for pi := range updated.Pages {
		if updated.Pages[pi].ID != updated.ActivePage {
			continue
		}
		for ki := range updated.Pages[pi].Keys {
			key := &updated.Pages[pi].Keys[ki]
			if key.Action == "plugin" && key.Index == in.Row*d.Model.Columns+in.Column {
				key.PluginSettings = append(json.RawMessage(nil), settings...)
				if err := SaveLayout(d.root, d.Serial, d.Model, updated); err != nil {
					d.status = "Plugin settings save failed: " + err.Error()
					return
				}
				d.layout = updated
				in.Settings = compactJSON(key.PluginSettings)
				d.pluginInstances[ctx] = in
				return
			}
		}
	}
}

// renderPluginKeyLocked draws a plugin key: the plugin's image full-bleed
// with the title as a caption, or a plain label until the plugin draws.
func (d *Device) renderPluginKeyLocked(size int, key KeyConfig, artwork image.Image) image.Image {
	pk := d.pluginKeys[d.pluginContext(d.layout.ActivePage, key.Index)]
	title := key.Label
	if title == "" && pk != nil && pk.hasTitle {
		title = pk.title
	}
	var img image.Image
	if pk != nil && pk.image != nil {
		img, _ = decodePluginImage(pk.image)
	}
	if img == nil && artwork != nil {
		return renderDeckArtwork(size, artwork, title)
	}
	if img == nil {
		if title == "" && pk == nil {
			title = "Plugin"
		}
		return renderKeyLabel(size, title)
	}
	canvas := renderKeyLabel(size, "").(*image.RGBA)
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), img, img.Bounds(), draw.Over, nil)
	if title == "" {
		return canvas
	}
	return drawDeckKeyCaption(canvas, title)
}

// keepPluginSettings carries settings a plugin or its settings page saved
// into an editor save. The editor has no settings of its own for plugin
// keys and may hold an older copy; a key whose plugin action changed
// starts with what the editor sent (normally nothing).
func keepPluginSettings(next *Layout, current Layout) {
	saved := map[string]KeyConfig{}
	for _, page := range current.Pages {
		for _, key := range page.Keys {
			if key.Action == "plugin" {
				saved[page.ID+"\x00"+strconv.Itoa(key.Index)] = key
			}
		}
	}
	for pi := range next.Pages {
		for ki := range next.Pages[pi].Keys {
			key := &next.Pages[pi].Keys[ki]
			old, ok := saved[next.Pages[pi].ID+"\x00"+strconv.Itoa(key.Index)]
			if ok && key.Action == "plugin" && key.Plugin == old.Plugin && key.PluginAction == old.PluginAction {
				key.PluginSettings = append(json.RawMessage(nil), old.PluginSettings...)
			}
		}
	}
}

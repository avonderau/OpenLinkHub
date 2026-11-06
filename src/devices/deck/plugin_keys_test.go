package deck

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net"
	"testing"
	"time"

	"OpenLinkHub/src/deckplugin"
)

// capturePluginMessages swaps in a bridge queue for one test.
func capturePluginMessages(t *testing.T) chan deckplugin.Message {
	t.Helper()
	out := make(chan deckplugin.Message, 64)
	plugins.mu.Lock()
	plugins.out = out
	plugins.mu.Unlock()
	t.Cleanup(func() {
		plugins.mu.Lock()
		plugins.out = nil
		plugins.contexts = map[string]*Device{}
		plugins.mu.Unlock()
	})
	return out
}

func nextPluginMessage(t *testing.T, out chan deckplugin.Message, kind string) deckplugin.Message {
	t.Helper()
	for {
		select {
		case m := <-out:
			if m.Type == kind {
				return m
			}
		default:
			t.Fatalf("no %s message", kind)
		}
	}
}

func pluginLayout() Layout {
	return Layout{Version: 1, ActivePage: "home", Pages: []Page{
		{ID: "home", Name: "Home", Keys: []KeyConfig{
			{Index: 7, Action: "plugin", Plugin: "com.example.counter", PluginAction: "com.example.counter.increment",
				PluginSettings: json.RawMessage(`{"count":1}`)},
			{Index: 0, Action: "page-next"},
		}},
		{ID: "two", Name: "Two", Keys: []KeyConfig{}},
	}}
}

func TestPluginKeyValidation(t *testing.T) {
	d := testDevice(t)
	if err := pluginLayout().Validate(d.Model); err != nil {
		t.Fatal(err)
	}
	bad := []KeyConfig{
		{Index: 1, Action: "plugin", Plugin: "bad id", PluginAction: "a.b"},
		{Index: 1, Action: "plugin", Plugin: "a.b", PluginAction: "a.b.c", PluginSettings: json.RawMessage(`[1]`)},
		{Index: 1, Action: "plugin", Plugin: "a.b", PluginAction: "a.b.c", Target: "x"},
		{Index: 1, Action: "hotkey", Target: "Ctrl+A", Plugin: "a.b"},
	}
	for _, key := range bad {
		l := DefaultLayout(d.Model)
		l.Pages[0].Keys = []KeyConfig{key}
		if l.Validate(d.Model) == nil {
			t.Errorf("accepted %+v", key)
		}
	}
}

func TestPluginKeysAppearUpdateAndDisappear(t *testing.T) {
	out := capturePluginMessages(t)
	d := testDevice(t)
	d.layout = pluginLayout()
	h := new(fakeDeckSession)
	if err := d.attachSession(h); err != nil {
		t.Fatal(err)
	}
	appear := nextPluginMessage(t, out, deckplugin.TypeAppear)
	if len(appear.Instances) != 1 {
		t.Fatalf("instances %+v", appear.Instances)
	}
	in := appear.Instances[0]
	ctx := d.pluginContext("home", 7)
	if in.Context != ctx || in.Row != 1 || in.Column != 1 || string(in.Settings) != `{"count":1}` {
		t.Fatalf("instance %+v", in)
	}

	// The plugin draws: one key image is sent and committed.
	img := image.NewRGBA(image.Rect(0, 0, 72, 72))
	img.Set(3, 3, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	before := countMiraboxOperation(h.reports, "BAT")
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeImage, Context: ctx, Image: buf.Bytes()})
	if got := countMiraboxOperation(h.reports, "BAT") - before; got != 1 {
		t.Fatalf("expected one key redraw, got %d", got)
	}
	// Images that are not bounded PNG/JPEG are ignored.
	before = countMiraboxOperation(h.reports, "BAT")
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeImage, Context: ctx, Image: []byte("<svg/>")})
	if countMiraboxOperation(h.reports, "BAT") != before {
		t.Fatal("invalid image redrew the key")
	}

	plugins.handle(deckplugin.Message{Type: deckplugin.TypeSettings, Context: ctx, Settings: json.RawMessage(`{"count":2}`)})
	saved, err := LoadLayout(d.root, d.Serial, d.Model)
	if err != nil || string(compactJSON(saved.Pages[0].Keys[0].PluginSettings)) != `{"count":2}` {
		t.Fatalf("settings not saved: %v %+v", err, saved.Pages[0].Keys)
	}

	d.handleKeyPress(7)
	if nextPluginMessage(t, out, deckplugin.TypeKeyDown).Context != ctx || nextPluginMessage(t, out, deckplugin.TypeKeyUp).Context != ctx {
		t.Fatal("press not forwarded")
	}

	// Leaving the page makes the key disappear.
	d.handleKeyPress(0)
	if nextPluginMessage(t, out, deckplugin.TypeDisappear).Context != ctx {
		t.Fatal("page change did not hide the plugin key")
	}
	plugins.mu.Lock()
	_, still := plugins.contexts[ctx]
	plugins.mu.Unlock()
	if still {
		t.Fatal("hidden key still routable")
	}
}

func TestPluginMessagesForUnknownContextsAreIgnored(t *testing.T) {
	capturePluginMessages(t)
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeTitle, Context: "not-a-context"})
	title := "x"
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeTitle, Context: "0123456789abcdef01234567", Title: &title})
}

func TestPluginCatalogIsSanitized(t *testing.T) {
	capturePluginMessages(t)
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeCatalog, Catalog: []deckplugin.CatalogPlugin{
		{UUID: "com.example.ok", Name: "OK", Actions: []deckplugin.CatalogAction{{UUID: "com.example.ok.a", Name: "A"}, {UUID: "../bad"}}},
		{UUID: "bad id", Name: "Bad"},
	}})
	plugins.mu.Lock()
	catalog := plugins.catalog
	plugins.catalog = nil
	plugins.mu.Unlock()
	if len(catalog) != 1 || len(catalog[0].Actions) != 1 {
		t.Fatalf("catalog %+v", catalog)
	}
}

func TestPluginManagerRequests(t *testing.T) {
	if _, err := InstallPlugin([]byte("zip")); err != ErrPluginHostOffline {
		t.Fatalf("offline install: %v", err)
	}
	out := capturePluginMessages(t)
	go func() {
		msg := <-out
		if msg.Type != deckplugin.TypeInstall || string(msg.Archive) != "zip" || !deckplugin.ValidRequest(msg.Request) {
			plugins.handle(deckplugin.Message{Type: deckplugin.TypeResult, Request: msg.Request, Error: "bad message"})
			return
		}
		// A result for another request is ignored.
		plugins.handle(deckplugin.Message{Type: deckplugin.TypeResult, Request: "ffffffffffffffffffffffffffffffff", Plugin: "x.y"})
		plugins.handle(deckplugin.Message{Type: deckplugin.TypeResult, Request: msg.Request, Plugin: "com.example.counter"})
	}()
	uuid, err := InstallPlugin([]byte("zip"))
	if err != nil || uuid != "com.example.counter" {
		t.Fatalf("install: %q %v", uuid, err)
	}
	go func() {
		msg := <-out
		plugins.handle(deckplugin.Message{Type: deckplugin.TypeResult, Request: msg.Request, Error: "plugin is not installed"})
	}()
	if err := RemovePlugin("com.example.counter"); err == nil || err.Error() != "plugin is not installed" {
		t.Fatalf("remove error not passed through: %v", err)
	}
	if err := RemovePlugin("../bad"); err == nil {
		t.Fatal("invalid ID sent")
	}
}

func TestPluginInspectorTicketsAndRelay(t *testing.T) {
	out := capturePluginMessages(t)
	d := testDevice(t)
	d.layout = pluginLayout()
	if err := d.attachSession(new(fakeDeckSession)); err != nil {
		t.Fatal(err)
	}
	ctx := d.pluginContext("home", 7)
	plugins.mu.Lock()
	plugins.client = &net.UnixConn{}
	plugins.catalog = []deckplugin.CatalogPlugin{{UUID: "com.example.counter", Name: "Counter",
		Actions: []deckplugin.CatalogAction{{UUID: "com.example.counter.increment", Name: "Inc", Inspector: "ui/pi.html"}}}}
	plugins.mu.Unlock()
	t.Cleanup(func() {
		plugins.mu.Lock()
		plugins.client, plugins.catalog = nil, nil
		plugins.mu.Unlock()
	})
	if _, err := d.OpenPluginInspector(0); err == nil {
		t.Fatal("ticket issued for a key without a plugin")
	}
	ticket, err := d.OpenPluginInspector(7)
	if err != nil || ticket.Inspector != "ui/pi.html" || ticket.Context != ctx || string(ticket.Settings) != `{"count":1}` {
		t.Fatalf("ticket %+v %v", ticket, err)
	}
	if _, ok := LookupInspectorTicket("not-a-ticket"); ok {
		t.Fatal("bad ticket accepted")
	}
	got, ok := LookupInspectorTicket(ticket.ID)
	if !ok {
		t.Fatal("ticket not found")
	}

	// Files come from the agent for the ticket's plugin only.
	go func() {
		msg := nextBlocking(out, deckplugin.TypeFile)
		if msg.Plugin != "com.example.counter" || msg.Path != "ui/app.js" {
			plugins.handle(deckplugin.Message{Type: deckplugin.TypeResult, Request: msg.Request, Error: "wrong file"})
			return
		}
		plugins.handle(deckplugin.Message{Type: deckplugin.TypeResult, Request: msg.Request, Data: []byte("js")})
	}()
	data, err := PluginInspectorFile(got, "ui/./app.js")
	if err != nil || string(data) != "js" {
		t.Fatalf("file %q %v", data, err)
	}
	if _, err := PluginInspectorFile(got, "../secret"); err == nil {
		t.Fatal("escaping path requested")
	}

	delivered := make(chan string, 4)
	closed := make(chan bool, 1)
	session, err := AttachPluginInspector(got, func(f json.RawMessage) { delivered <- string(f) }, func() { closed <- true })
	if err != nil {
		t.Fatal(err)
	}
	open := nextBlocking(out, deckplugin.TypeInspectorOpen)
	if open.Context != ctx || !deckplugin.ValidSession(open.Session) {
		t.Fatalf("open %+v", open)
	}
	session.Send(json.RawMessage(`{"event":"getSettings"}`))
	session.Send(json.RawMessage(`not json`))
	if m := nextBlocking(out, deckplugin.TypeInspector); m.Session != open.Session || string(m.Payload) != `{"event":"getSettings"}` {
		t.Fatalf("relay %+v", m)
	}
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeInspector, Session: open.Session, Payload: json.RawMessage(`{"event":"didReceiveSettings"}`)})
	plugins.handle(deckplugin.Message{Type: deckplugin.TypeInspector, Session: "ffffffffffffffffffffffffffffffff", Payload: json.RawMessage(`{}`)})
	if f := <-delivered; f != `{"event":"didReceiveSettings"}` {
		t.Fatalf("delivered %s", f)
	}
	session.Close()
	if nextBlocking(out, deckplugin.TypeInspectorClose).Session != open.Session || !<-closed {
		t.Fatal("close not relayed")
	}
}

func nextBlocking(out chan deckplugin.Message, kind string) deckplugin.Message {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case m := <-out:
			if m.Type == kind {
				return m
			}
		case <-timeout:
			return deckplugin.Message{}
		}
	}
}

func TestEditorSaveKeepsPluginSettings(t *testing.T) {
	current := pluginLayout()
	next := cloneLayout(current)
	next.Pages[0].Keys[0].PluginSettings = nil // the editor's older copy
	keepPluginSettings(&next, current)
	if string(next.Pages[0].Keys[0].PluginSettings) != `{"count":1}` {
		t.Fatalf("settings lost: %s", next.Pages[0].Keys[0].PluginSettings)
	}
	changed := cloneLayout(current)
	changed.Pages[0].Keys[0].PluginAction = "com.example.counter.reset"
	changed.Pages[0].Keys[0].PluginSettings = nil
	keepPluginSettings(&changed, current)
	if changed.Pages[0].Keys[0].PluginSettings != nil {
		t.Fatal("settings carried to a different action")
	}
}

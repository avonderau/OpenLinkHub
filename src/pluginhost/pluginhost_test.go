package pluginhost

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"OpenLinkHub/src/deckplugin"

	"github.com/gorilla/websocket"
)

func writePlugin(t *testing.T, root, folder, manifest string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, folder)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o700)
		mode := os.FileMode(0o600)
		if strings.HasPrefix(body, "#!") {
			mode = 0o700
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const nodeManifest = `{"UUID":"com.example.counter","Name":"Counter","Version":"1.0","CodePath":"bin/plugin.js",
 "Actions":[{"UUID":"com.example.counter.increment","Name":"Increment","States":[{"Image":"imgs/key","Title":"0"}]},
            {"UUID":"com.example.counter.dial","Name":"Dial","Controllers":["Encoder"]}]}`

func noNode(string) (string, error)   { return "", errors.New("missing") }
func fakeNode(string) (string, error) { return "/usr/bin/node", nil }

func TestLoadPluginResolvesNodeAndNativeBuilds(t *testing.T) {
	root := t.TempDir()
	dir := writePlugin(t, root, "com.example.counter.sdPlugin", nodeManifest, map[string]string{"bin/plugin.js": "//"})
	p, err := loadPlugin(dir, fakeNode)
	if err != nil || p.Err != nil {
		t.Fatalf("load: %v / %v", err, p.Err)
	}
	if p.UUID != "com.example.counter" || len(p.Actions) != 1 || p.Actions[0].Name != "Increment" {
		t.Fatalf("unexpected plugin %+v", p)
	}
	if strings.Join(p.Command, " ") != "/usr/bin/node "+filepath.Join(dir, "bin/plugin.js") {
		t.Fatalf("command %v", p.Command)
	}
	if p, _ := loadPlugin(dir, noNode); p.Err == nil || !strings.Contains(p.Err.Error(), "Node.js") {
		t.Fatalf("missing node not reported: %v", p.Err)
	}
	native := `{"PluginUUID":"st.example.native","Name":"Native","CodePathLin":"native","CodePath":"plugin.exe",
	 "CodePaths":{"` + targetTriple() + `":"bin/native"},"Actions":[{"UUID":"st.example.native.go","Name":"Go"}]}`
	dir = writePlugin(t, root, "st.example.native.sdPlugin", native, map[string]string{"bin/native": "#!/bin/sh\n"})
	if p, err := loadPlugin(dir, noNode); err != nil || p.Err != nil || p.Command[0] != filepath.Join(dir, "bin/native") {
		t.Fatalf("native: %v %+v", err, p)
	}
}

func TestLoadPluginRefusesUnsafeOrUnsupportedPlugins(t *testing.T) {
	root := t.TempDir()
	cases := map[string]struct {
		manifest string
		files    map[string]string
		want     string
	}{
		"device": {`{"UUID":"st.lynx.dev","Name":"Hardware","CodePathLin":"x","DeviceNamespace":"99","Actions":[]}`, nil, "hardware"},
		"escape": {`{"UUID":"a.escape","CodePath":"../outside.js","Actions":[{"UUID":"a.escape.x"}]}`, nil, "leaves the plugin folder"},
		"html":   {`{"UUID":"a.html","CodePath":"index.html","Actions":[{"UUID":"a.html.x"}]}`, map[string]string{"index.html": "<html>"}, "HTML"},
		"exe":    {`{"UUID":"a.exe","CodePath":"plugin.exe","Actions":[{"UUID":"a.exe.x"}]}`, map[string]string{"plugin.exe": "MZ"}, "Windows"},
		"macho":  {`{"UUID":"a.mac","CodePath":"plugin","Actions":[{"UUID":"a.mac.x"}]}`, map[string]string{"plugin": "#!"}, ""},
		"none":   {`{"UUID":"a.none","CodePath":"p.js","Actions":[{"UUID":"a.none.x","Controllers":["Encoder"]}]}`, map[string]string{"p.js": ""}, "no key actions"},
	}
	for name, c := range cases {
		dir := writePlugin(t, root, name+".sdPlugin", c.manifest, c.files)
		p, err := loadPlugin(dir, fakeNode)
		if c.want == "" {
			continue
		}
		if err == nil {
			err = p.Err
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", name, err, c.want)
		}
	}
	os.WriteFile(filepath.Join(root, "outside.js"), nil, 0o600)
	if err := os.Symlink(filepath.Join(root, "outside.js"), filepath.Join(root, "escape.sdPlugin", "link.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := pluginPath(filepath.Join(root, "escape.sdPlugin"), "link.js"); err == nil {
		t.Fatal("symlink out of the plugin folder accepted")
	}
}

func zipFile(t *testing.T, entries map[string]string, link string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.streamDeckPlugin")
	f, _ := os.Create(path)
	w := zip.NewWriter(f)
	for name, body := range entries {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			h.SetMode(0o755)
		}
		out, _ := w.CreateHeader(h)
		out.Write([]byte(body))
	}
	if link != "" {
		h := &zip.FileHeader{Name: link}
		h.SetMode(os.ModeSymlink | 0o777)
		out, _ := w.CreateHeader(h)
		out.Write([]byte("/etc/passwd"))
	}
	w.Close()
	f.Close()
	return path
}

func TestInstallArchive(t *testing.T) {
	plugins := t.TempDir()
	good := zipFile(t, map[string]string{
		"com.example.counter.sdPlugin/manifest.json":  nodeManifest,
		"com.example.counter.sdPlugin/bin/plugin.js":  "//",
		"com.example.counter.sdPlugin/imgs/key.png":   "png",
		"__MACOSX/com.example.counter.sdPlugin/._x":   "",
		"com.example.counter.sdPlugin/scripts/run.sh": "#!/bin/sh",
	}, "")
	p, err := Install(good, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if p.UUID != "com.example.counter" {
		t.Fatalf("installed %+v", p)
	}
	info, err := os.Stat(filepath.Join(plugins, "com.example.counter.sdPlugin", "scripts", "run.sh"))
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("executable bit lost: %v", err)
	}
	for name, archive := range map[string]string{
		"slip":   zipFile(t, map[string]string{"a.sdPlugin/../../evil": "x"}, ""),
		"two":    zipFile(t, map[string]string{"a.sdPlugin/manifest.json": "{}", "b.sdPlugin/manifest.json": "{}"}, ""),
		"link":   zipFile(t, map[string]string{"a.sdPlugin/manifest.json": nodeManifest}, "a.sdPlugin/link"),
		"device": zipFile(t, map[string]string{"d.sdPlugin/manifest.json": `{"UUID":"d.dev","DeviceNamespace":"99","Actions":[]}`}, ""),
	} {
		if _, err := Install(archive, plugins); err == nil {
			t.Errorf("%s archive accepted", name)
		}
	}
	entries, _ := os.ReadDir(plugins)
	if len(entries) != 1 {
		t.Fatalf("refused installs left files behind: %v", entries)
	}
}

func pngBytes(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestDecodeDataURL(t *testing.T) {
	raw := pngBytes(t)
	got, err := decodeDataURL("data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw), nil)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("png: %v", err)
	}
	svg := func([]byte) ([]byte, error) { return raw, nil }
	if _, err := decodeDataURL("data:image/svg+xml;charset=utf8,%3Csvg%3E%3C/svg%3E", svg); err != nil {
		t.Fatalf("svg: %v", err)
	}
	if _, err := decodeDataURL("data:image/svg+xml,<svg/>", nil); err == nil {
		t.Fatal("svg accepted without a rasterizer")
	}
	for _, bad := range []string{"https://example.com/x.png", "data:image/png;base64,!!!", "data:text/plain,hello"} {
		if _, err := decodeDataURL(bad, nil); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

// recorder collects what the host sends to the service.
type recorder struct {
	mu   sync.Mutex
	msgs []deckplugin.Message
}

func (r *recorder) send(m deckplugin.Message) { r.mu.Lock(); r.msgs = append(r.msgs, m); r.mu.Unlock() }

func (r *recorder) wait(t *testing.T, match func(deckplugin.Message) bool) deckplugin.Message {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, m := range r.msgs {
			if match(m) {
				r.mu.Unlock()
				return m
			}
		}
		r.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for host message")
	return deckplugin.Message{}
}

const testContext = "0123456789abcdef01234567"

func testHost(t *testing.T, launch func(h *Host, p *process) error) (*Host, *recorder, *Plugin) {
	t.Helper()
	root := t.TempDir()
	dir := writePlugin(t, root, "com.example.counter.sdPlugin", nodeManifest, map[string]string{"bin/plugin.js": "//"})
	os.MkdirAll(filepath.Join(dir, "imgs"), 0o700)
	os.WriteFile(filepath.Join(dir, "imgs", "key.png"), pngBytes(t), 0o600)
	plugin, err := loadPlugin(dir, fakeNode)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	h := NewHost(t.TempDir(), rec.send)
	h.launch = func(p *process) error { return launch(h, p) }
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	h.SetPlugins([]*Plugin{plugin})
	return h, rec, plugin
}

func dialHost(t *testing.T, h *Host, header http.Header) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:"+strconv.Itoa(h.port), header)
	return conn, err
}

func TestHostProtocolRoundTrip(t *testing.T) {
	events := make(chan map[string]any, 16)
	var plugin *websocket.Conn
	h, rec, _ := testHost(t, func(h *Host, p *process) error {
		go func() {
			conn, err := dialHost(t, h, nil)
			if err != nil {
				return
			}
			plugin = conn
			conn.WriteJSON(map[string]string{"event": registerEvent, "uuid": p.token})
			for {
				var ev map[string]any
				if conn.ReadJSON(&ev) != nil {
					return
				}
				events <- ev
			}
		}()
		return nil
	})
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeDevices, Devices: []deckplugin.Device{{ID: "deck-1", Name: "Deck", Rows: 3, Columns: 6}}})
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeAppear, Instances: []deckplugin.Instance{{
		Context: testContext, Plugin: "com.example.counter", Action: "com.example.counter.increment",
		Device: "deck-1", Row: 1, Column: 2, Settings: json.RawMessage(`{"count":3}`)}}})

	// The manifest's default image and title are shown before the plugin draws.
	rec.wait(t, func(m deckplugin.Message) bool { return m.Type == deckplugin.TypeImage && len(m.Image) > 0 })
	rec.wait(t, func(m deckplugin.Message) bool { return m.Type == deckplugin.TypeTitle && *m.Title == "0" })

	ev := <-events
	if ev["event"] != "willAppear" || ev["context"] != testContext {
		t.Fatalf("first event %v", ev)
	}
	payload := ev["payload"].(map[string]any)
	if payload["settings"].(map[string]any)["count"] != 3.0 || payload["coordinates"].(map[string]any)["column"] != 2.0 {
		t.Fatalf("payload %v", payload)
	}
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeKeyDown, Context: testContext})
	if ev := <-events; ev["event"] != "keyDown" {
		t.Fatalf("expected keyDown, got %v", ev)
	}

	plugin.WriteJSON(map[string]any{"event": "setTitle", "context": testContext, "payload": map[string]any{"title": "4"}})
	plugin.WriteJSON(map[string]any{"event": "setSettings", "context": testContext, "payload": map[string]any{"count": 4}})
	plugin.WriteJSON(map[string]any{"event": "setImage", "context": testContext,
		"payload": map[string]any{"image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes(t))}})
	// A plugin cannot touch a key it does not own.
	plugin.WriteJSON(map[string]any{"event": "setTitle", "context": "ffffffffffffffffffffffff", "payload": map[string]any{"title": "x"}})
	rec.wait(t, func(m deckplugin.Message) bool { return m.Type == deckplugin.TypeTitle && *m.Title == "4" })
	rec.wait(t, func(m deckplugin.Message) bool {
		return m.Type == deckplugin.TypeSettings && string(m.Settings) == `{"count":4}`
	})
	plugin.WriteJSON(map[string]any{"event": "getSettings", "context": testContext})
	if ev := <-events; ev["event"] != "didReceiveSettings" || ev["payload"].(map[string]any)["settings"].(map[string]any)["count"] != 4.0 {
		t.Fatalf("getSettings reply %v", ev)
	}
	plugin.WriteJSON(map[string]any{"event": "setGlobalSettings", "context": "x", "payload": map[string]any{"token": "abc"}})
	plugin.WriteJSON(map[string]any{"event": "getGlobalSettings", "context": "x"})
	if ev := <-events; ev["event"] != "didReceiveGlobalSettings" ||
		ev["payload"].(map[string]any)["settings"].(map[string]any)["token"] != "abc" {
		t.Fatalf("global settings reply %v", ev)
	}
	rec.mu.Lock()
	for _, m := range rec.msgs {
		if m.Context == "ffffffffffffffffffffffff" {
			t.Fatal("plugin updated a context it does not own")
		}
	}
	rec.mu.Unlock()
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeDisappear, Context: testContext})
	if ev := <-events; ev["event"] != "willDisappear" {
		t.Fatalf("expected willDisappear, got %v", ev)
	}
}

func TestHostRefusesBrowsersAndUnknownTokens(t *testing.T) {
	h, _, _ := testHost(t, func(*Host, *process) error { return nil })
	if _, err := dialHost(t, h, http.Header{"Origin": {"https://evil.example"}}); err == nil {
		t.Fatal("browser origin accepted")
	}
	conn, err := dialHost(t, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.WriteJSON(map[string]string{"event": registerEvent, "uuid": "guess"})
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("unknown registration token kept open")
	}
}

// A real Node.js plugin, using the documented launch arguments.
func TestHostRunsNodePlugin(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	if out, err := exec.Command(node, "-e", "process.exit(typeof WebSocket==='function'?0:1)").CombinedOutput(); err != nil {
		t.Skipf("node has no WebSocket client: %s", out)
	}
	script := `const a=process.argv.slice(2);const arg=n=>a[a.indexOf(n)+1];
const info=JSON.parse(arg('-info'));
const ws=new WebSocket('ws://127.0.0.1:'+arg('-port'));
ws.onopen=()=>ws.send(JSON.stringify({event:arg('-registerEvent'),uuid:arg('-pluginUUID')}));
ws.onmessage=m=>{const e=JSON.parse(m.data);
 if(e.event==='keyDown')ws.send(JSON.stringify({event:'setTitle',context:e.context,payload:{title:'pressed on '+info.devices[0].name}}));};`
	root := t.TempDir()
	dir := writePlugin(t, root, "com.example.counter.sdPlugin", nodeManifest, map[string]string{"bin/plugin.js": script})
	plugin, err := LoadPlugin(dir)
	if err != nil || plugin.Err != nil {
		t.Fatalf("load: %v %v", err, plugin.Err)
	}
	rec := &recorder{}
	h := NewHost(t.TempDir(), rec.send)
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	defer h.Stop()
	h.SetPlugins([]*Plugin{plugin})
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeDevices, Devices: []deckplugin.Device{{ID: "deck-1", Name: "HSV293S", Rows: 3, Columns: 6}}})
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeAppear, Instances: []deckplugin.Instance{{
		Context: testContext, Plugin: plugin.UUID, Action: "com.example.counter.increment", Device: "deck-1"}}})
	// Wait for registration before pressing.
	deadline := time.Now().Add(10 * time.Second)
	for {
		h.mu.Lock()
		p := h.running[plugin.UUID]
		h.mu.Unlock()
		if p != nil {
			p.connMu.Lock()
			ready := p.conn != nil
			p.connMu.Unlock()
			if ready {
				break
			}
		}
		if time.Now().After(deadline) {
			log, _ := os.ReadFile(filepath.Join(h.stateDir, "logs", plugin.UUID+".log"))
			t.Fatalf("plugin never registered; log: %s", log)
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeKeyDown, Context: testContext})
	rec.wait(t, func(m deckplugin.Message) bool {
		return m.Type == deckplugin.TypeTitle && *m.Title == "pressed on HSV293S"
	})
}

func TestManagerInstallsAndRemoves(t *testing.T) {
	plugins := t.TempDir()
	data, err := os.ReadFile(zipFile(t, map[string]string{
		"com.example.counter.sdPlugin/manifest.json": nodeManifest,
		"com.example.counter.sdPlugin/bin/plugin.js": "//",
	}, ""))
	if err != nil {
		t.Fatal(err)
	}
	const request = "0123456789abcdef0123456789abcdef"
	// Declined on the desktop: nothing is installed.
	declined := func(*Plugin) error { return ErrInstallDeclined }
	if res := manage(deckplugin.Message{Type: deckplugin.TypeInstall, Request: request, Archive: data}, plugins, declined); res.Error == "" {
		t.Fatal("declined install succeeded")
	}
	if entries, _ := os.ReadDir(plugins); len(entries) != 0 {
		t.Fatalf("declined install left files: %v", entries)
	}
	var asked *Plugin
	agree := func(p *Plugin) error { asked = p; return nil }
	res := manage(deckplugin.Message{Type: deckplugin.TypeInstall, Request: request, Archive: data}, plugins, agree)
	if asked == nil || asked.UUID != "com.example.counter" {
		t.Fatalf("confirmation not asked with the plugin: %+v", asked)
	}
	if res.Type != deckplugin.TypeResult || res.Request != request || res.Error != "" || res.Plugin != "com.example.counter" {
		t.Fatalf("install result %+v", res)
	}
	if res := manage(deckplugin.Message{Type: deckplugin.TypeInstall, Request: request, Archive: []byte("not a zip")}, plugins, agree); res.Error == "" {
		t.Fatal("garbage archive accepted")
	}
	if res := manage(deckplugin.Message{Type: deckplugin.TypeRemove, Request: "bad", Plugin: "com.example.counter"}, plugins, agree); res.Error == "" {
		t.Fatal("invalid request ID accepted")
	}
	for _, bad := range []string{"../x", "com.example.other"} {
		if res := manage(deckplugin.Message{Type: deckplugin.TypeRemove, Request: request, Plugin: bad}, plugins, agree); res.Error == "" {
			t.Errorf("removed %q", bad)
		}
	}
	if res := manage(deckplugin.Message{Type: deckplugin.TypeRemove, Request: request, Plugin: "com.example.counter"}, plugins, agree); res.Error != "" {
		t.Fatalf("remove: %s", res.Error)
	}
	if entries, _ := os.ReadDir(plugins); len(entries) != 0 {
		t.Fatalf("plugins folder not empty after remove: %v", entries)
	}
}

func TestSetPluginsRestartsChangedPlugins(t *testing.T) {
	var mu sync.Mutex
	launches := 0
	h, rec, plugin := testHost(t, func(h *Host, p *process) error {
		mu.Lock()
		launches++
		mu.Unlock()
		return nil
	})
	in := deckplugin.Instance{Context: testContext, Plugin: plugin.UUID, Action: "com.example.counter.increment"}
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeAppear, Instances: []deckplugin.Instance{in}})
	count := func() int { mu.Lock(); defer mu.Unlock(); return launches }
	if count() != 1 {
		t.Fatalf("launches %d", count())
	}

	// Removed while on screen: the process stops and the key says so.
	h.SetPlugins(nil)
	rec.wait(t, func(m deckplugin.Message) bool {
		return m.Type == deckplugin.TypeTitle && m.Title != nil && *m.Title == "Plugin missing"
	})
	h.mu.Lock()
	running := h.running[plugin.UUID]
	h.mu.Unlock()
	if running != nil {
		t.Fatal("removed plugin still running")
	}

	// Installed again: the key on screen starts it without a page change.
	h.SetPlugins([]*Plugin{plugin})
	if count() != 2 {
		t.Fatalf("reinstall did not restart the plugin: %d launches", count())
	}
	// An unchanged rescan leaves it alone; a rewritten manifest restarts it.
	h.SetPlugins([]*Plugin{plugin})
	updated := *plugin
	updated.stamp = "changed"
	h.SetPlugins([]*Plugin{&updated})
	if count() != 3 {
		t.Fatalf("expected a restart after an update, got %d launches", count())
	}
}

func TestInspectorPathsAndFiles(t *testing.T) {
	root := t.TempDir()
	manifest := `{"UUID":"com.example.pi","Name":"PI","CodePath":"bin/plugin.js","PropertyInspectorPath":"ui/plugin.html",
 "Actions":[{"UUID":"com.example.pi.a","Name":"A","States":[{}]},
            {"UUID":"com.example.pi.b","Name":"B","PropertyInspectorPath":"ui/b.html","States":[{}]},
            {"UUID":"com.example.pi.c","Name":"C","PropertyInspectorPath":"../../etc/passwd","States":[{}]},
            {"UUID":"com.example.pi.d","Name":"D","PropertyInspectorPath":"ui/missing.html","States":[{}]}]}`
	dir := writePlugin(t, root, "com.example.pi.sdPlugin", manifest, map[string]string{
		"bin/plugin.js": "//", "ui/plugin.html": "<html></html>", "ui/b.html": "<p>b</p>", "ui/app.js": "1",
	})
	os.Symlink("/etc/hostname", filepath.Join(dir, "ui", "leak.txt"))
	p, err := loadPlugin(dir, fakeNode)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"com.example.pi.a": "ui/plugin.html", "com.example.pi.b": "ui/b.html",
		"com.example.pi.c": "", "com.example.pi.d": ""}
	for _, a := range p.Actions {
		if a.Inspector != want[a.UUID] {
			t.Errorf("%s inspector %q", a.UUID, a.Inspector)
		}
	}
	if data, err := ReadPluginFile(dir, "ui/app.js"); err != nil || string(data) != "1" {
		t.Fatalf("read: %q %v", data, err)
	}
	for _, bad := range []string{"../x", "/etc/passwd", "ui/leak.txt", "ui", "ui/app.js?x", ""} {
		if _, err := ReadPluginFile(dir, bad); err == nil {
			t.Errorf("read %q", bad)
		}
	}
}

func TestInspectorSessionRelay(t *testing.T) {
	events := make(chan map[string]any, 32)
	var plugin *websocket.Conn
	ready := make(chan struct{})
	h, rec, _ := testHost(t, func(h *Host, p *process) error {
		go func() {
			conn, err := dialHost(t, h, nil)
			if err != nil {
				return
			}
			plugin = conn
			conn.WriteJSON(map[string]string{"event": registerEvent, "uuid": p.token})
			close(ready)
			for {
				var ev map[string]any
				if conn.ReadJSON(&ev) != nil {
					return
				}
				events <- ev
			}
		}()
		return nil
	})
	next := func(name string) map[string]any {
		t.Helper()
		for {
			select {
			case ev := <-events:
				if ev["event"] == name {
					return ev
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("no %s event", name)
			}
		}
	}
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeAppear, Instances: []deckplugin.Instance{{
		Context: testContext, Plugin: "com.example.counter", Action: "com.example.counter.increment", Device: "deck-1"}}})
	<-ready
	next("willAppear")
	const session = "0123456789abcdef0123456789abcdef"
	// A session for a key that is not on screen is ignored.
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeInspectorOpen, Session: "ffffffffffffffffffffffffffffffff", Context: "ffffffffffffffffffffffff"})
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeInspectorOpen, Session: session, Context: testContext})
	if ev := next("propertyInspectorDidAppear"); ev["context"] != testContext {
		t.Fatalf("appear %v", ev)
	}
	pi := func(raw string) {
		h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeInspector, Session: session, Payload: json.RawMessage(raw)})
	}
	toPage := func(match func(map[string]any) bool) map[string]any {
		t.Helper()
		var out map[string]any
		rec.wait(t, func(m deckplugin.Message) bool {
			if m.Type != deckplugin.TypeInspector || m.Session != session {
				return false
			}
			var ev map[string]any
			if json.Unmarshal(m.Payload, &ev) != nil || !match(ev) {
				return false
			}
			out = ev
			return true
		})
		return out
	}
	pi(`{"event":"registerPropertyInspector","uuid":"` + testContext + `"}`)
	// The page claims another key's context; it still edits its own key only.
	pi(`{"event":"setSettings","context":"ffffffffffffffffffffffff","payload":{"city":"Berlin"}}`)
	rec.wait(t, func(m deckplugin.Message) bool {
		return m.Type == deckplugin.TypeSettings && m.Context == testContext && string(m.Settings) == `{"city":"Berlin"}`
	})
	if ev := next("didReceiveSettings"); ev["context"] != testContext {
		t.Fatalf("plugin settings %v", ev)
	}
	pi(`{"event":"getSettings","context":"` + testContext + `"}`)
	toPage(func(ev map[string]any) bool {
		p, _ := ev["payload"].(map[string]any)
		s, _ := p["settings"].(map[string]any)
		return ev["event"] == "didReceiveSettings" && s["city"] == "Berlin"
	})
	pi(`{"event":"sendToPlugin","action":"x","context":"` + testContext + `","payload":{"refresh":true}}`)
	if ev := next("sendToPlugin"); ev["payload"].(map[string]any)["refresh"] != true || ev["action"] != "com.example.counter.increment" {
		t.Fatalf("sendToPlugin %v", ev)
	}
	plugin.WriteJSON(map[string]any{"event": "sendToPropertyInspector", "context": testContext, "payload": map[string]any{"cities": 3}})
	toPage(func(ev map[string]any) bool {
		p, _ := ev["payload"].(map[string]any)
		return ev["event"] == "sendToPropertyInspector" && p["cities"] == 3.0
	})
	pi(`{"event":"setGlobalSettings","payload":{"key":"k"}}`)
	if ev := next("didReceiveGlobalSettings"); ev["payload"].(map[string]any)["settings"].(map[string]any)["key"] != "k" {
		t.Fatalf("global %v", ev)
	}
	h.HandleServiceMessage(deckplugin.Message{Type: deckplugin.TypeInspectorClose, Session: session})
	next("propertyInspectorDidDisappear")
	// A closed session can no longer edit the key.
	pi(`{"event":"setSettings","payload":{"city":"Paris"}}`)
	time.Sleep(50 * time.Millisecond)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, m := range rec.msgs {
		if m.Type == deckplugin.TypeSettings && strings.Contains(string(m.Settings), "Paris") {
			t.Fatal("closed settings page changed the key")
		}
	}
}

func TestInstallQuestionIsPlainText(t *testing.T) {
	q := installQuestion(&Plugin{Name: "Evil\u202e<b>x</b>\x1b[2J", UUID: "com.example.evil", Version: "1", Author: "<i>me</i>",
		Actions: []Action{{Name: "A"}, {Name: "B"}}})
	if strings.ContainsAny(q, "\u202e\x1b") || !strings.Contains(q, "com.example.evil") || !strings.Contains(q, "Actions: A, B") {
		t.Fatalf("question %q", q)
	}
}

func TestPluginSocketsFromOtherUsersAreRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	uid, err := loopbackPeerUID(c.LocalAddr().String(), ln.Addr().String())
	if err != nil || uid != os.Geteuid() {
		t.Fatalf("own socket: uid %d %v", uid, err)
	}

	// A connection owned by another user is refused before the WebSocket
	// handshake.
	table := filepath.Join(t.TempDir(), "tcp")
	_, lport, _ := net.SplitHostPort(ln.Addr().String())
	_, cport, _ := net.SplitHostPort(c.LocalAddr().String())
	lp, _ := strconv.Atoi(lport)
	cp, _ := strconv.Atoi(cport)
	other := os.Geteuid() + 1
	line := fmt.Sprintf("  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"+
		"   0: 0100007F:%04X 0100007F:%04X 01 00000000:00000000 00:00000000 00000000  %d        0 1 1 0 20 4 30 10 -1\n", cp, lp, other)
	os.WriteFile(table, []byte(line), 0o600)
	saved := procNetTCP
	procNetTCP = []string{table}
	defer func() { procNetTCP = saved }()
	if uid, _ := loopbackPeerUID(c.LocalAddr().String(), ln.Addr().String()); uid != other {
		t.Fatalf("parsed uid %d", uid)
	}
	h, _, _ := testHost(t, func(*Host, *process) error { return nil })
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = c.LocalAddr().String()
	h.listener = ln
	h.serveWebSocket(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other user's socket got %d", w.Code)
	}
}

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"OpenLinkHub/src/deckplugin"
	"OpenLinkHub/src/devices/deck"

	"github.com/gorilla/websocket"
)

// Plugin settings pages (property inspectors) are plugin code. They are
// served under a ticket that the signed-in editor obtains for one plugin
// key, always with a CSP sandbox so the page runs in an opaque origin and
// cannot use the WebUI's session or APIs.
const (
	deckPluginUIPrefix     = "/api/deck/plugin-ui/"
	deckPluginSocketPrefix = "/api/deck/plugin-socket/"
	deckInspectorSandbox   = "sandbox allow-scripts allow-forms allow-popups allow-modals"
)

// deckPluginInspectorAPI issues a settings-page ticket:
// POST /api/deck/plugin-inspector?serial=<deck>&key=<index>.
func deckPluginInspectorAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !nexusWriteAllowed(r) {
		http.Error(w, "cross-origin write forbidden", http.StatusForbidden)
		return
	}
	d, ok := deckDevice(r)
	if !ok {
		http.Error(w, "Deck device not found", http.StatusNotFound)
		return
	}
	index, err := strconv.Atoi(r.URL.Query().Get("key"))
	if err != nil || index < 0 || index >= d.Model.Keys() {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}
	ticket, err := d.OpenPluginInspector(index)
	if err != nil {
		deckPluginError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": deckPluginUIPrefix + ticket.ID + "/" + ticket.Inspector})
}

// deckPluginTicket splits /prefix/<ticket>/<rest>.
func deckPluginTicket(r *http.Request, prefix string) (*deck.InspectorTicket, string, bool) {
	if !nexusRequestHostAllowed(r) || r.Method != http.MethodGet {
		return nil, "", false
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	id, file, _ := strings.Cut(rest, "/")
	t, ok := deck.LookupInspectorTicket(id)
	return t, file, ok
}

// deckPluginUIFile serves one settings-page file from the plugin folder.
func deckPluginUIFile(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Add("Content-Security-Policy", deckInspectorSandbox)
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	t, file, ok := deckPluginTicket(r, deckPluginUIPrefix)
	if !ok {
		http.Error(w, "plugin settings page expired; close it and open it again", http.StatusNotFound)
		return
	}
	file = deckplugin.CleanInspectorPath(file)
	if file == "" {
		http.NotFound(w, r)
		return
	}
	data, err := deck.PluginInspectorFile(t, file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(path.Ext(file))
	contentType := mime.TypeByExtension(ext)
	switch ext {
	case ".js", ".mjs":
		contentType = "text/javascript; charset=utf-8"
	case ".html", ".htm":
		contentType = "text/html; charset=utf-8"
		data = injectInspectorShim(data, t)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	h.Set("Content-Type", contentType)
	// The sandboxed page has an opaque ("null") origin; this lets it load
	// its own module scripts and data files.
	h.Set("Access-Control-Allow-Origin", "null")
	_, _ = w.Write(data)
}

// injectInspectorShim starts the page the way the Stream Deck app does,
// with connectElgatoStreamDeckSocket, and points its WebSocket at the
// OpenLinkHub relay instead of a local plugin host port.
func injectInspectorShim(page []byte, t *deck.InspectorTicket) []byte {
	settings := t.Settings
	if len(settings) == 0 {
		settings = json.RawMessage("{}")
	}
	cfg, _ := json.Marshal(map[string]any{
		"ticket": t.ID,
		"uuid":   t.Context,
		"info": map[string]any{
			"application": map[string]any{"language": "en", "platform": "linux", "version": "6.9.0"},
			"plugin":      map[string]string{"uuid": t.Plugin},
			"devices": []map[string]any{{"id": t.Serial, "name": t.Device.Name,
				"size": map[string]int{"columns": t.Device.Columns, "rows": t.Device.Rows}}},
			"colors": map[string]string{},
		},
		"actionInfo": map[string]any{
			"action": t.Action, "context": t.Context, "device": t.Serial,
			"payload": map[string]any{"settings": settings,
				"coordinates": map[string]int{"column": t.Column, "row": t.Row}, "controller": "Keypad"},
		},
	})
	shim := fmt.Sprintf(`<script>(function(){
var cfg=%s;
var socket=(location.protocol==="https:"?"wss://":"ws://")+location.host+%q+cfg.ticket;
var Native=window.WebSocket;
function local(u){try{var x=new URL(String(u),location.href);var h=x.hostname.replace(/^\[|\]$/g,"");return(x.protocol==="ws:"||x.protocol==="wss:")&&(h==="127.0.0.1"||h==="localhost"||h==="::1");}catch(e){return false;}}
function Relay(u,p){if(local(u)){u=socket;}return p===undefined?new Native(u):new Native(u,p);}
Relay.prototype=Native.prototype;Relay.CONNECTING=0;Relay.OPEN=1;Relay.CLOSING=2;Relay.CLOSED=3;
window.WebSocket=Relay;
var started=false;
function start(){if(started){return;}var fn=window.connectElgatoStreamDeckSocket||window.connectSocket||window.connectOpenActionSocket;
if(typeof fn!=="function"){return;}started=true;
fn(28196,cfg.uuid,"registerPropertyInspector",JSON.stringify(cfg.info),JSON.stringify(cfg.actionInfo));}
if(document.readyState==="complete"){setTimeout(start,0);}else{window.addEventListener("load",function(){setTimeout(start,0);});}
})();</script>`, cfg, deckPluginSocketPrefix)
	lower := bytes.ToLower(page)
	at := 0
	if i := bytes.Index(lower, []byte("<head")); i >= 0 {
		if j := bytes.IndexByte(lower[i:], '>'); j >= 0 {
			at = i + j + 1
		}
	}
	var out bytes.Buffer
	out.Write(page[:at])
	out.WriteString(shim)
	out.Write(page[at:])
	return out.Bytes()
}

var inspectorUpgrader = websocket.Upgrader{
	ReadBufferSize:  16 << 10,
	WriteBufferSize: 16 << 10,
	// The sandboxed page sends Origin "null". A same-host origin is also
	// accepted; any other site is refused.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "null" || origin == "" || strings.EqualFold(strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://"), r.Host)
	},
}

// deckPluginSocket relays a settings page's WebSocket to the desktop agent.
func deckPluginSocket(w http.ResponseWriter, r *http.Request) {
	t, rest, ok := deckPluginTicket(r, deckPluginSocketPrefix)
	if !ok || rest != "" {
		http.Error(w, "plugin settings page expired", http.StatusNotFound)
		return
	}
	conn, err := inspectorUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(deckplugin.MaxInspectorFrame)
	out := make(chan json.RawMessage, 64)
	done := make(chan struct{})
	session, err := deck.AttachPluginInspector(t,
		func(frame json.RawMessage) {
			select {
			case out <- frame:
			default:
			}
		},
		func() { conn.Close() })
	if err != nil {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, err.Error()))
		conn.Close()
		return
	}
	go func() {
		for {
			select {
			case frame := <-out:
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if conn.WriteMessage(websocket.TextMessage, frame) != nil {
					conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if kind == websocket.TextMessage {
			session.Send(data)
		}
	}
	close(done)
	session.Close()
	conn.Close()
}

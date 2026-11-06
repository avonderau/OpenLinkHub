package deck

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"OpenLinkHub/src/deckplugin"
)

// A settings page (property inspector) is plugin code. The editor loads it
// in a sandboxed frame from OpenLinkHub; a ticket bound to one plugin key
// authorizes its files and its WebSocket, which the service relays to the
// desktop agent. The page never reaches the plugin host's own socket.

const (
	inspectorTicketIdle = 30 * time.Minute
	maxInspectorTickets = 64
)

// InspectorTicket is what a settings page needs to start.
type InspectorTicket struct {
	ID        string
	Plugin    string
	Action    string
	Context   string
	Device    Model
	Serial    string
	Row       int
	Column    int
	Settings  json.RawMessage
	Inspector string
	lastUsed  time.Time
}

var inspectorTickets = struct {
	sync.Mutex
	byID map[string]*InspectorTicket
}{byID: map[string]*InspectorTicket{}}

// InspectorSession is one open settings page.
type InspectorSession struct {
	id      string
	deliver func(json.RawMessage)
	closed  func()
	once    sync.Once
}

func randomHex() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// OpenPluginInspector issues a ticket for the settings page of a plugin key
// on the page the Deck is showing.
func (d *Device) OpenPluginInspector(index int) (*InspectorTicket, error) {
	d.mu.Lock()
	ctx := d.pluginContext(d.layout.ActivePage, index)
	in, visible := d.pluginInstances[ctx]
	d.mu.Unlock()
	if !visible {
		return nil, errors.New("show this key's page on the Deck and save the layout first, then open its settings")
	}
	connected, catalog := PluginCatalog()
	if !connected {
		return nil, ErrPluginHostOffline
	}
	inspector := ""
	for _, p := range catalog {
		if p.UUID != in.Plugin {
			continue
		}
		for _, a := range p.Actions {
			if a.UUID == in.Action {
				inspector = a.Inspector
			}
		}
	}
	if inspector == "" {
		return nil, errors.New("this plugin action has no settings page")
	}
	id, err := randomHex()
	if err != nil {
		return nil, err
	}
	t := &InspectorTicket{ID: id, Plugin: in.Plugin, Action: in.Action, Context: ctx, Device: d.Model,
		Serial: d.Serial, Row: in.Row, Column: in.Column, Settings: in.Settings, Inspector: inspector,
		lastUsed: time.Now()}
	inspectorTickets.Lock()
	defer inspectorTickets.Unlock()
	pruneInspectorTicketsLocked()
	if len(inspectorTickets.byID) >= maxInspectorTickets {
		return nil, errors.New("too many plugin settings pages are open")
	}
	inspectorTickets.byID[id] = t
	return t, nil
}

func pruneInspectorTicketsLocked() {
	for id, t := range inspectorTickets.byID {
		if time.Since(t.lastUsed) > inspectorTicketIdle {
			delete(inspectorTickets.byID, id)
		}
	}
}

// LookupInspectorTicket returns a live ticket and marks it used.
func LookupInspectorTicket(id string) (*InspectorTicket, bool) {
	if !deckplugin.ValidRequest(id) {
		return nil, false
	}
	inspectorTickets.Lock()
	defer inspectorTickets.Unlock()
	pruneInspectorTicketsLocked()
	t := inspectorTickets.byID[id]
	if t == nil {
		return nil, false
	}
	t.lastUsed = time.Now()
	copy := *t
	return &copy, true
}

// PluginInspectorFile fetches one file of the ticket's plugin from the agent.
func PluginInspectorFile(t *InspectorTicket, rel string) ([]byte, error) {
	clean := deckplugin.CleanInspectorPath(rel)
	if clean == "" {
		return nil, errors.New("invalid file path")
	}
	result, err := plugins.requestResult(deckplugin.Message{Type: deckplugin.TypeFile, Plugin: t.Plugin, Path: clean}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if len(result.Data) > deckplugin.MaxFileBytes {
		return nil, errors.New("file too large")
	}
	return result.Data, nil
}

// AttachPluginInspector connects a settings page to its key. deliver gets
// each message for the page and must not block; closed runs once when the
// agent goes away.
func AttachPluginInspector(t *InspectorTicket, deliver func(json.RawMessage), closed func()) (*InspectorSession, error) {
	id, err := randomHex()
	if err != nil {
		return nil, err
	}
	s := &InspectorSession{id: id, deliver: deliver, closed: closed}
	plugins.mu.Lock()
	if plugins.out == nil {
		plugins.mu.Unlock()
		return nil, ErrPluginHostOffline
	}
	if len(plugins.inspectors) >= 32 {
		plugins.mu.Unlock()
		return nil, errors.New("too many plugin settings pages are open")
	}
	plugins.inspectors[id] = s
	plugins.mu.Unlock()
	plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeInspectorOpen, Session: id, Context: t.Context})
	return s, nil
}

// Send relays one message from the page to the agent.
func (s *InspectorSession) Send(frame json.RawMessage) {
	if len(frame) == 0 || len(frame) > deckplugin.MaxInspectorFrame || !json.Valid(frame) {
		return
	}
	plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeInspector, Session: s.id, Payload: frame})
}

// Close ends the session.
func (s *InspectorSession) Close() {
	plugins.mu.Lock()
	_, open := plugins.inspectors[s.id]
	delete(plugins.inspectors, s.id)
	plugins.mu.Unlock()
	if open {
		plugins.enqueue(deckplugin.Message{Type: deckplugin.TypeInspectorClose, Session: s.id})
	}
	s.once.Do(s.closed)
}

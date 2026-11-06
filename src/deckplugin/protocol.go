// Package deckplugin defines the messages exchanged between the OpenLinkHub
// service and the desktop agent's plugin host over the deck-plugins socket.
//
// The service owns the hardware and the layouts. It tells the agent which
// plugin keys are on screen and when they are pressed. The agent runs the
// plugin processes as the desktop user and sends back only bounded,
// validated key updates (title, image, state, settings). Plugin code never
// runs inside the service.
package deckplugin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// SocketName is the agent socket, next to the launch socket.
const SocketName = "deck-plugins.sock"

// Limits shared by both ends. A message is one JSON line.
const (
	MaxMessageBytes  = 2 << 20
	MaxImageBytes    = 512 << 10
	MaxSettingsBytes = 8 << 10
	MaxTitleRunes    = 64
	MaxCatalogItems  = 256
)

// Message types, service to agent.
const (
	TypeDevices   = "devices"   // Devices: every connected Deck
	TypeAppear    = "appear"    // Instance: key became visible
	TypeDisappear = "disappear" // Context: key left the screen
	TypeKeyDown   = "keyDown"   // Context
	TypeKeyUp     = "keyUp"     // Context
)

// Message types, agent to service.
const (
	TypeCatalog  = "catalog"  // Catalog: installed plugins and actions
	TypeTitle    = "title"    // Context, Title
	TypeImage    = "image"    // Context, Image (PNG/JPEG bytes; empty clears)
	TypeState    = "state"    // Context, State
	TypeSettings = "settings" // Context, Settings
	TypeAlert    = "alert"    // Context
	TypeOK       = "ok"       // Context
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var contextPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

// ValidID reports whether s is an acceptable plugin or action UUID
// (reverse-DNS style, as used by Stream Deck and OpenAction manifests).
func ValidID(s string) bool { return idPattern.MatchString(s) }

// ValidContext reports whether s is a context produced by ContextID.
func ValidContext(s string) bool { return contextPattern.MatchString(s) }

// ContextID is the stable instance ID of one plugin key, derived from the
// device, page and key position.
func ContextID(deviceSerial, pageID string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", deviceSerial, pageID, index)))
	return fmt.Sprintf("%x", sum[:12])
}

// ValidSettings accepts a JSON object of bounded size; empty means {}.
func ValidSettings(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > MaxSettingsBytes {
		return errors.New("plugin settings too large")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return errors.New("plugin settings must be a JSON object")
	}
	return nil
}

type Device struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Rows    int    `json:"rows"`
	Columns int    `json:"columns"`
	Type    int    `json:"type"`
}

// Instance is one plugin key on screen.
type Instance struct {
	Context  string          `json:"context"`
	Plugin   string          `json:"plugin"`
	Action   string          `json:"action"`
	Device   string          `json:"device"`
	Row      int             `json:"row"`
	Column   int             `json:"column"`
	State    int             `json:"state"`
	Settings json.RawMessage `json:"settings,omitempty"`
}

type CatalogAction struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Tooltip string `json:"tooltip,omitempty"`
}

type CatalogPlugin struct {
	UUID    string          `json:"uuid"`
	Name    string          `json:"name"`
	Version string          `json:"version,omitempty"`
	Author  string          `json:"author,omitempty"`
	Actions []CatalogAction `json:"actions"`
	// Error is set when the plugin is installed but cannot run here.
	Error string `json:"error,omitempty"`
}

// Message is one line on the socket. Only the fields for Type are set.
type Message struct {
	Type      string          `json:"type"`
	Context   string          `json:"context,omitempty"`
	Devices   []Device        `json:"devices,omitempty"`
	Instances []Instance      `json:"instances,omitempty"`
	Catalog   []CatalogPlugin `json:"catalog,omitempty"`
	Title     *string         `json:"title,omitempty"`
	Image     []byte          `json:"image,omitempty"`
	State     int             `json:"state,omitempty"`
	Settings  json.RawMessage `json:"settings,omitempty"`
}

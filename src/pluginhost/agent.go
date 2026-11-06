package pluginhost

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"OpenLinkHub/src/deckplugin"
	"OpenLinkHub/src/devices/nexus"
)

// Run connects the desktop agent's plugin host to the service socket at
// socketPath and keeps it connected until ctx ends. Plugins are read from
// PluginsDir and rescanned when that folder changes.
func Run(ctx context.Context, socketPath string) error {
	pluginsDir, err := PluginsDir()
	if err != nil {
		return err
	}
	stateDir, err := xdgDir("XDG_STATE_HOME", ".local/state", "deck-plugins")
	if err != nil {
		return err
	}
	var writerMu sync.Mutex
	var conn net.Conn
	send := func(msg deckplugin.Message) {
		writerMu.Lock()
		defer writerMu.Unlock()
		if conn == nil {
			return
		}
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if json.NewEncoder(conn).Encode(msg) != nil {
			conn.Close()
		}
	}
	host := NewHost(stateDir, send)
	if err := host.Start(); err != nil {
		return err
	}
	defer host.Stop()

	signature := ""
	rescan := func() bool {
		next := folderSignature(pluginsDir)
		if next == signature {
			return false
		}
		signature = next
		host.SetPlugins(ScanPlugins(pluginsDir))
		return true
	}
	rescan()
	lastFailure := ""
	for ctx.Err() == nil {
		c, err := nexus.DialDesktopAgentSocket(socketPath)
		if err != nil {
			if msg := err.Error(); msg != lastFailure {
				fmt.Fprintf(os.Stderr, "Deck plugin host waiting for service: %v\n", err)
				lastFailure = msg
			}
			if sleepCtx(ctx, 2*time.Second) != nil {
				break
			}
			continue
		}
		lastFailure = ""
		writerMu.Lock()
		conn = c
		writerMu.Unlock()
		fmt.Fprintln(os.Stderr, "Deck plugin host connected to local service")
		sendCatalog(send, pluginsDir)

		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					c.Close()
					return
				case <-done:
					return
				case <-ticker.C:
					if rescan() {
						sendCatalog(send, pluginsDir)
					}
				}
			}
		}()
		scanner := bufio.NewScanner(c)
		scanner.Buffer(make([]byte, 64<<10), deckplugin.MaxMessageBytes)
		for scanner.Scan() {
			var msg deckplugin.Message
			if json.Unmarshal(scanner.Bytes(), &msg) == nil {
				host.HandleServiceMessage(msg)
			}
		}
		close(done)
		writerMu.Lock()
		conn = nil
		writerMu.Unlock()
		c.Close()
		host.Reset()
		if ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "Deck plugin host lost the local service; reconnecting")
			sleepCtx(ctx, time.Second)
		}
	}
	return nil
}

func sendCatalog(send func(deckplugin.Message), pluginsDir string) {
	send(deckplugin.Message{Type: deckplugin.TypeCatalog, Catalog: catalog(ScanPlugins(pluginsDir))})
}

// folderSignature changes when a plugin folder is added, removed or its
// manifest is edited.
func folderSignature(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sdPlugin") {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, e.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%d", e.Name(), info.Size(), info.ModTime().UnixNano()))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// SocketPathFor returns the plugin socket next to the launch socket.
func SocketPathFor(launchSocketPath string) string {
	return filepath.Join(filepath.Dir(launchSocketPath), deckplugin.SocketName)
}

package nexus

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNexusDesktopBrokerForwardsOnlyValidatedLocalLaunches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nexus", "desktop-launch.sock")
	broker, err := startNexusDesktopLaunchBroker(path)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0660 {
		t.Fatalf("unsafe socket permissions: %v", info.Mode().Perm())
	}
	if err := broker.send("url", "https://example.com/first"); err == nil || !strings.Contains(err.Error(), "no Nexus desktop agent") {
		t.Fatalf("expected actionable disconnected error: %v", err)
	}
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.After(time.Second)
	for {
		err = broker.send("url", "https://example.com/path?q=test")
		if err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("broker never accepted agent: %v", err)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bufio.NewReader(conn))
	var event nexusDesktopLaunchEvent
	if err := decoder.Decode(&event); err != nil {
		t.Fatal(err)
	}
	if event.Status != nexusDesktopStatusReady {
		t.Fatalf("expected ready handshake, got %+v", event)
	}
	if err := decoder.Decode(&event); err != nil {
		t.Fatal(err)
	}
	if event.Action != "url" || event.Target != "https://example.com/path?q=test" {
		t.Fatalf("unexpected forwarded event: %+v", event)
	}
	if err := broker.send("url", "file:///etc/passwd"); err == nil {
		t.Fatal("unsafe URL should be rejected")
	}
	if err := broker.send("application", "../../bin/sh"); err == nil {
		t.Fatal("unsafe application should be rejected")
	}
}

// A second agent must be refused instead of evicting the first one, so touches
// stay with one desktop session; the slot frees once the first agent leaves.
func TestNexusDesktopBrokerKeepsFirstAgentAndRefusesSecond(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nexus", "desktop-launch.sock")
	broker, err := startNexusDesktopLaunchBroker(path)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.close()
	readStatus := func(conn net.Conn) (*json.Decoder, string) {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(conn)
		var event nexusDesktopLaunchEvent
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		return decoder, event.Status
	}
	first, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	firstDecoder, status := readStatus(first)
	if status != nexusDesktopStatusReady {
		t.Fatalf("first agent not accepted: %q", status)
	}
	second, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, status := readStatus(second); status != nexusDesktopStatusBusy {
		t.Fatalf("second agent should be refused, got %q", status)
	}
	if err := broker.send("url", "https://example.com/first"); err != nil {
		t.Fatal(err)
	}
	var event nexusDesktopLaunchEvent
	if err := firstDecoder.Decode(&event); err != nil || event.Target != "https://example.com/first" {
		t.Fatalf("first agent lost its launches: %+v %v", event, err)
	}
	first.Close()
	deadline := time.Now().Add(2 * time.Second)
	for {
		third, err := net.DialTimeout("unix", path, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_, status := readStatus(third)
		third.Close()
		if status == nexusDesktopStatusReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("slot never freed after first agent disconnected")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNexusDesktopBrokerRejectsWorldWritableAndExistingFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nexus")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "desktop-launch.sock")
	if err := os.WriteFile(path, []byte("not a socket"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := startNexusDesktopLaunchBroker(path); err == nil {
		t.Fatal("non-socket file unexpectedly removed")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "not a socket" {
		t.Fatalf("existing file clobbered: %s %v", data, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := startNexusDesktopLaunchBroker(path); err == nil {
		t.Fatal("world-writable socket directory accepted")
	}
}

func TestNexusDesktopAgentReceivesValidatedTouchEvents(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("desktop agent must run as non-root")
	}
	t.Setenv("DISPLAY", ":99")
	t.Setenv("WAYLAND_DISPLAY", "")
	path := filepath.Join(t.TempDir(), "nexus", "desktop-launch.sock")
	broker, err := startNexusDesktopLaunchBroker(path)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan nexusDesktopLaunchEvent, 1)
	stopped := make(chan error, 1)
	go func() {
		stopped <- runNexusDesktopLaunchAgent(ctx, path, func(a, u string) error {
			events <- nexusDesktopLaunchEvent{Action: a, Target: u}
			return nil
		})
	}()
	deadline := time.After(2 * time.Second)
	for {
		err = broker.send("url", "https://example.com/agent")
		if err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("agent did not connect: %v", err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	select {
	case event := <-events:
		if event.Action != "url" || event.Target != "https://example.com/agent" {
			t.Fatalf("unexpected action: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("desktop agent did not receive touch")
	}
	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("desktop agent failed to stop")
	}
}

// An agent must never attach to an arbitrary local socket impersonating the
// daemon, nor accept commands via a symlink or writable socket directory.
func TestNexusDesktopSocketIdentityAndRuntimeSeparation(t *testing.T) {
	root, err := os.MkdirTemp("", "nx-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	path := nexusDesktopLaunchSocketPath(root)
	if strings.Contains(path, filepath.Join("database", "nexus")) {
		t.Fatalf("socket overlaps private Nexus media: %q", path)
	}
	broker, err := startNexusDesktopLaunchBroker(path)
	if err != nil {
		t.Fatal(err)
	}
	defer broker.close()
	owner, err := verifyNexusDesktopSocket(path)
	if err != nil {
		t.Fatal(err)
	}
	if owner != uint32(os.Geteuid()) {
		t.Fatalf("unexpected socket owner %d", owner)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0750 {
		t.Fatalf("socket directory permissions = %#o", dir.Mode().Perm())
	}
	agent, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	peer, err := nexusDesktopPeerUID(agent)
	if err != nil {
		t.Fatal(err)
	}
	if peer != owner {
		t.Fatalf("unexpected server peer uid %d", peer)
	}
	if err := os.Chmod(filepath.Dir(path), 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyNexusDesktopSocket(path); err == nil {
		t.Fatal("unsafe socket directory accepted")
	}
	if err := os.Chmod(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyNexusDesktopSocket(path); err == nil {
		t.Fatal("world-accessible socket accepted")
	}
}
func TestNexusDesktopSocketRejectsSymlinkDirectory(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "private")
	if err := os.Mkdir(realDir, 0750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := startNexusDesktopLaunchBroker(filepath.Join(link, "desktop-launch.sock")); err == nil {
		t.Fatal("symlink launcher directory unexpectedly accepted")
	}
}

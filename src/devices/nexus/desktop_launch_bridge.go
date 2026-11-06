package nexus

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// A system OpenLinkHub service is not the signed-in desktop user. The desktop
// agent opts in by connecting to a group-protected Unix socket. The daemon
// sends only validated physical-touch events and never accepts commands here.
const nexusDesktopSocketName = "desktop-launch.sock"

type nexusDesktopLaunchEvent struct {
	Action string `json:"action"`
	Target string `json:"target"`
}
type nexusDesktopLaunchBroker struct {
	mu       sync.Mutex
	listener net.Listener
	client   net.Conn
	path     string
}

// Keep launch IPC outside the 0700 media/config folder. A system service
// normally owns this directory as user/group openlinkhub, while authorized
// desktop users can traverse via the openlinkhub group.
func nexusDesktopLaunchSocketPath(root string) string {
	return filepath.Join(root, "runtime", "nexus-desktop", nexusDesktopSocketName)
}

func nexusSocketOwner(info os.FileInfo) (uint32, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("cannot determine Unix socket owner")
	}
	return stat.Uid, nil
}

func verifyNexusDesktopSocket(path string) (uint32, error) {
	directory, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return 0, err
	}
	if !directory.IsDir() || directory.Mode()&os.ModeSymlink != 0 ||
		directory.Mode().Perm()&0022 != 0 || directory.Mode().Perm()&0005 != 0 {
		return 0, errors.New("Nexus launcher directory has unsafe permissions")
	}
	owner, err := nexusSocketOwner(directory)
	if err != nil {
		return 0, err
	}
	entry, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	if entry.Mode()&os.ModeSocket == 0 || entry.Mode()&os.ModeSymlink != 0 ||
		entry.Mode().Perm()&0007 != 0 {
		return 0, errors.New("Nexus launcher socket has unsafe type or permissions")
	}
	socketOwner, err := nexusSocketOwner(entry)
	if err != nil {
		return 0, err
	}
	if socketOwner != owner {
		return 0, errors.New("Nexus launcher socket has a different owner than its directory")
	}
	return owner, nil
}

// Confirm the process on the other end of the Unix connection owns the
// inspected socket. This prevents a local socket-path swap from redirecting
// an agent into a different user's launcher.
func nexusDesktopPeerUID(conn net.Conn) (uint32, error) {
	socket, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("launcher transport is not a Unix connection")
	}
	raw, err := socket.SyscallConn()
	if err != nil {
		return 0, err
	}
	var creds *unix.Ucred
	var credentialErr error
	if err = raw.Control(func(fd uintptr) {
		creds, credentialErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if credentialErr != nil {
		return 0, credentialErr
	}
	if creds == nil {
		return 0, errors.New("missing Unix peer credentials")
	}
	return creds.Uid, nil
}
func startNexusDesktopLaunchBroker(path string) (*nexusDesktopLaunchBroker, error) {
	// Linux AF_UNIX sun_path is 108 bytes including the terminating NUL.
	if len(path) > 107 {
		return nil, errors.New("Nexus launcher socket path exceeds Linux Unix socket limit (107 bytes)")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create Nexus launcher directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&0022 != 0 {
		return nil, errors.New("Nexus launcher directory must be private and not group/world-writable")
	}
	owner, err := nexusSocketOwner(info)
	if err != nil {
		return nil, err
	}
	if owner != uint32(os.Geteuid()) {
		return nil, errors.New("Nexus launcher directory must be owned by daemon user")
	}
	// The desktop user needs group execute permission, but must not be able
	// to replace the daemon-owned socket file.
	if err := os.Chmod(dir, 0750); err != nil {
		return nil, err
	}
	if existing, err := os.Lstat(path); err == nil {
		if existing.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("Nexus launcher socket path is not a socket")
		}
		staleOwner, err := nexusSocketOwner(existing)
		if err != nil || staleOwner != owner {
			return nil, errors.New("Nexus launcher socket owner mismatch")
		}
		if conn, err := net.DialTimeout("unix", path, 150*time.Millisecond); err == nil {
			conn.Close()
			return nil, errors.New("Nexus launcher socket already in use")
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen for Nexus desktop agent: %w", err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		listener.Close()
		os.Remove(path)
		return nil, err
	}
	if _, err := verifyNexusDesktopSocket(path); err != nil {
		listener.Close()
		os.Remove(path)
		return nil, err
	}
	b := &nexusDesktopLaunchBroker{listener: listener, path: path}
	go b.accept()
	return b, nil
}
func (b *nexusDesktopLaunchBroker) accept() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		if b.client != nil {
			b.client.Close()
		}
		b.client = conn
		b.mu.Unlock()
	}
}
func (b *nexusDesktopLaunchBroker) send(action, target string) error {
	if err := validateNexusLaunchAction(action, target); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.client == nil {
		return errors.New("no Nexus desktop agent connected; start OpenLinkHub --nexus-desktop-agent in the logged-in desktop session")
	}
	b.client.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	if err := json.NewEncoder(b.client).Encode(nexusDesktopLaunchEvent{Action: action, Target: target}); err != nil {
		b.client.Close()
		b.client = nil
		return fmt.Errorf("Nexus desktop agent disconnected: %w", err)
	}
	return nil
}
func (b *nexusDesktopLaunchBroker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.client != nil {
		b.client.Close()
		b.client = nil
	}
	b.listener.Close()
	os.Remove(b.path)
}

var nexusLaunchBrokerGlobal struct {
	once   sync.Once
	broker *nexusDesktopLaunchBroker
	err    error
}

func initializeNexusDesktopLaunchBroker(root string) error {
	nexusLaunchBrokerGlobal.once.Do(func() {
		nexusLaunchBrokerGlobal.broker, nexusLaunchBrokerGlobal.err = startNexusDesktopLaunchBroker(nexusDesktopLaunchSocketPath(root))
	})
	return nexusLaunchBrokerGlobal.err
}
func forwardNexusDesktopLaunch(action, target string) error {
	broker := nexusLaunchBrokerGlobal.broker
	if broker == nil {
		return errors.New("Nexus desktop agent unavailable; run OpenLinkHub --nexus-desktop-agent in the graphical session")
	}
	return broker.send(action, target)
}

func runNexusDesktopLaunchAgent(ctx context.Context, socketPath string, execute func(string, string) error) error {
	if err := launchPermissionCheck(os.Geteuid(), os.Getenv("DISPLAY"), os.Getenv("WAYLAND_DISPLAY")); err != nil {
		return fmt.Errorf("Nexus desktop agent must run as a logged-in desktop user: %w", err)
	}
	if execute == nil {
		return errors.New("missing desktop launcher")
	}
	lastFailure := ""
	reportFailure := func(stage string, err error) {
		message := fmt.Sprintf("%s: %v", stage, err)
		if message != lastFailure {
			fmt.Fprintf(os.Stderr, "Nexus desktop agent connection failed: %s\n", message)
			lastFailure = message
		}
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		// Refuse spoofed socket files/symlinks or directories writable by other
		// local users. A disconnected service is retried without executing events.
		expectedOwner, err := verifyNexusDesktopSocket(socketPath)
		if err != nil {
			reportFailure("socket verification", err)
			if waitNexusAgentRetry(ctx) != nil {
				return nil
			}
			continue
		}
		conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
		if err != nil {
			reportFailure("socket connection", err)
			if waitNexusAgentRetry(ctx) != nil {
				return nil
			}
			continue
		}
		peer, err := nexusDesktopPeerUID(conn)
		if err != nil || peer != expectedOwner {
			if err != nil {
				reportFailure("peer verification", err)
			} else {
				reportFailure("peer verification", fmt.Errorf("unexpected socket owner UID %d (expected %d)", peer, expectedOwner))
			}
			conn.Close()
			if waitNexusAgentRetry(ctx) != nil {
				return nil
			}
			continue
		}
		lastFailure = ""
		fmt.Fprintln(os.Stderr, "Nexus desktop agent connected to local service")
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				conn.Close()
			case <-done:
			}
		}()
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 4096), 4096)
		for scanner.Scan() {
			var event nexusDesktopLaunchEvent
			if json.Unmarshal(scanner.Bytes(), &event) != nil {
				continue
			}
			if validateNexusLaunchAction(event.Action, event.Target) != nil {
				continue
			}
			if err := execute(event.Action, event.Target); err != nil {
				fmt.Fprintf(os.Stderr, "Nexus desktop %s launch failed: %v\n", event.Action, err)
			}
		}
		close(done)
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			reportFailure("connection lost", err)
		}
		conn.Close()
		if waitNexusAgentRetry(ctx) != nil {
			return nil
		}
	}
}
func waitNexusAgentRetry(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// RunNexusDesktopLaunchAgent runs locally as a desktop user; it does not
// start the WebUI or access HID devices. A blank path uses the current cwd.
func RunNexusDesktopLaunchAgent(ctx context.Context, socketPath string) error {
	if socketPath == "" {
		root, err := os.Getwd()
		if err != nil {
			return err
		}
		socketPath = nexusDesktopLaunchSocketPath(root)
	}
	return runNexusDesktopLaunchAgent(ctx, socketPath, runNexusLaunchAction)
}

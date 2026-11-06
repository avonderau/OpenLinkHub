package common

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeCommandFixture(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-command")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+contents+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBoundedCommandOutputAcceptsSmallValidOutput(t *testing.T) {
	path := writeCommandFixture(t, "printf '%s' \"$1\"")
	data, err := BoundedCommandOutput(time.Second, 128, path, "GPU Model Name")
	if err != nil || string(data) != "GPU Model Name" {
		t.Fatalf("valid command output = %q, err %v", data, err)
	}
}

func TestBoundedCommandOutputRejectsExcessStdout(t *testing.T) {
	path := writeCommandFixture(t, "printf '%05000d' 1")
	start := time.Now()
	data, err := BoundedCommandOutput(time.Second, 64, path)
	if err == nil || len(data) != 0 {
		t.Fatalf("oversized output accepted: %d bytes, %v", len(data), err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("oversized command output stalled execution")
	}
}

func TestBoundedCommandOutputKillsHungProcess(t *testing.T) {
	path := writeCommandFixture(t, "sleep 10")
	start := time.Now()
	data, err := BoundedCommandOutput(100*time.Millisecond, 64, path)
	if err == nil || len(data) != 0 {
		t.Fatalf("hung command returned %q and %v", data, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("hung command exceeded expected timeout")
	}
}

func TestBoundedCommandOutputNeverUsesShell(t *testing.T) {
	path := writeCommandFixture(t, "printf '42'")
	data, err := BoundedCommandOutput(time.Second, 64, path)
	if err != nil || string(data) != "42" {
		t.Fatalf("normal command failed: %q %v", data, err)
	}
	if _, err := BoundedCommandOutput(time.Second, 64, path+"; echo exploit"); err == nil {
		t.Fatal("shell metacharacters were interpreted as an executable")
	}
}

func TestBoundedCommandOutputInvalidLimits(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		n    int
		name string
	}{
		{0, 32, "/bin/sh"}, {time.Second, 0, "/bin/sh"}, {time.Second, 32, ""},
	} {
		_, err := BoundedCommandOutput(tc.d, tc.n, tc.name)
		if err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("limits %+v accepted, err %v", tc, err)
		}
	}
	out := &boundedCommandOutput{limit: 4, cancel: func() {}}
	if _, err := out.Write([]byte("42")); err != nil {
		t.Fatal(err)
	}
	if _, err := out.Write([]byte("unexpected")); !errors.Is(err, ErrCommandOutputTooLarge) {
		t.Fatalf("expected explicit oversized output error, got %v", err)
	}
	if out.buffer.Len() > 4 {
		t.Fatal("output buffer exceeded limit")
	}
}

package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUdevadmInfoBoundsDeviceDiscovery(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "udevadm")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(executable, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	write("printf 'ID_PATH_WITH_USB_REVISION=pci-0000:00:14.0-usb-0:1.2:1.0\\n'")
	text, err := runUdevadmInfo("/dev/hidraw0")
	if err != nil || !strings.Contains(text, "ID_PATH_WITH_USB_REVISION=") {
		t.Fatalf("valid udevadm output = %q, err %v", text, err)
	}

	write("printf '%0150000d' 1")
	text, err = runUdevadmInfo("/dev/hidraw0")
	if err == nil || text != "" {
		t.Fatalf("oversized udevadm output accepted (%d bytes), err %v", len(text), err)
	}
}

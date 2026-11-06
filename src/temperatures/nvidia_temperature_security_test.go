package temperatures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNvidiaTemperatureCommandUsesBoundedOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "nvidia-smi")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write("printf '54\\n'")
	if got := GetNVIDIAGpuTemperature(0); got != 54 {
		t.Fatalf("valid GPU temperature lost: %v", got)
	}
	write("printf '%0256d' 1")
	if got := GetNVIDIAGpuTemperature(0); got != 0 {
		t.Fatalf("oversized GPU temperature output accepted: %v", got)
	}
}

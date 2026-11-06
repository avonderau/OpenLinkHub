package systeminfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeHardwareCommand(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return file
}

func TestNvidiaCommandsAcceptBoundedReadings(t *testing.T) {
	fakeHardwareCommand(t, "nvidia-smi", `case "$*" in
		*gpu_name*) printf 'Test NVIDIA GPU\n' ;;
		*utilization.gpu*) printf '47\n' ;;
		*) exit 1 ;;
	esac`)
	if got := GetNVIDIAGpuModel(0); got != "Test NVIDIA GPU" {
		t.Fatalf("NVIDIA model: %q", got)
	}
	if got := getNVIDIAUtilization(0); got != 47 {
		t.Fatalf("NVIDIA utilization: %d", got)
	}
}

func TestNvidiaCommandsRejectExcessOutput(t *testing.T) {
	fakeHardwareCommand(t, "nvidia-smi", "printf '%02048d' 1")
	if got := GetNVIDIAGpuModel(0); got != "" {
		t.Fatalf("oversized model output accepted: %q", got)
	}
	if got := getNVIDIAUtilization(0); got != 0 {
		t.Fatalf("oversized utilization output accepted: %d", got)
	}
}

func TestAMDModelEmptyArrayNoPanic(t *testing.T) {
	path := fakeHardwareCommand(t, "fake-amd-smi", `printf '%s\n' '{"gpu_data":[]}'`)
	oldPath, oldFound := amdsmi, isAmdsmiFound
	amdsmi, isAmdsmiFound = path, true
	t.Cleanup(func() { amdsmi, isAmdsmiFound = oldPath, oldFound })
	if got := GetAMDGpuModel(); got != "" {
		t.Fatalf("empty AMD GPU list returned model %q", got)
	}
}

func TestAMDModelNormalOutput(t *testing.T) {
	path := fakeHardwareCommand(t, "fake-amd-smi", `printf '%s\n' '{"gpu_data":[{"gpu":0,"asic":{"market_name":"Test AMD GPU"}}]}'`)
	oldPath, oldFound := amdsmi, isAmdsmiFound
	amdsmi, isAmdsmiFound = path, true
	t.Cleanup(func() { amdsmi, isAmdsmiFound = oldPath, oldFound })
	if got := GetAMDGpuModel(); got != "Test AMD GPU" {
		t.Fatalf("valid AMD model lost: %q", got)
	}
}

func TestAMDModelRejectsExcessOutput(t *testing.T) {
	path := fakeHardwareCommand(t, "fake-amd-smi", "printf '%070000d' 1")
	oldPath, oldFound := amdsmi, isAmdsmiFound
	amdsmi, isAmdsmiFound = path, true
	t.Cleanup(func() { amdsmi, isAmdsmiFound = oldPath, oldFound })
	if got := GetAMDGpuModel(); strings.TrimSpace(got) != "" {
		t.Fatalf("oversized AMD JSON accepted: %q", got)
	}
}

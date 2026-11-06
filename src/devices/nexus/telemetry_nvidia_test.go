package nexus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNvidiaMetrics(t *testing.T) {
	result := parseNvidiaMetrics("2048, 8192, 1650, 124.5\n")
	if result["gpu.vram.used_mb"] != 2048 || result["gpu.vram.total_mb"] != 8192 || result["gpu.vram.utilization"] != 25 {
		t.Fatalf("bad VRAM results: %+v", result)
	}
	if result["gpu.clock_mhz"] != 1650 || result["gpu.power_w"] != 124.5 {
		t.Fatalf("bad clock or power: %+v", result)
	}
}

func TestNvidiaOptionalAndInvalidReadings(t *testing.T) {
	cases := []struct {
		line      string
		wantVRAM  bool
		wantClock bool
		wantPower bool
	}{
		{"N/A, N/A, 1200, N/A", false, true, false},
		{"9999, 8192, 30000, 12000", false, false, false},
		{"0, 8192, N/A, 0", true, false, true},
		{"garbage", false, false, false},
		{"1200, 8192, 1500, 100", true, true, true},
	}
	for _, tc := range cases {
		found := parseNvidiaMetrics(tc.line)
		_, vram := found["gpu.vram.utilization"]
		_, clock := found["gpu.clock_mhz"]
		_, power := found["gpu.power_w"]
		if vram != tc.wantVRAM || clock != tc.wantClock || power != tc.wantPower {
			t.Errorf("%q: %+v", tc.line, found)
		}
	}
	for _, index := range []int{-1, 16, 999} {
		if got := ReadNvidiaMetrics(index); len(got) != 0 {
			t.Fatalf("invalid index %d returned metrics", index)
		}
	}
}

func TestReadNvidiaMetricsBoundsSubprocessOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "nvidia-smi")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	write("printf '1024, 4096, 1680, 120.5\\n'")
	metrics := ReadNvidiaMetrics(0)
	if metrics["gpu.vram.used_mb"] != 1024 || metrics["gpu.clock_mhz"] != 1680 {
		t.Fatalf("bounded NVIDIA telemetry incorrectly rejected: %+v", metrics)
	}
	write("printf '%03000d' 1")
	if got := ReadNvidiaMetrics(0); len(got) != 0 {
		t.Fatalf("oversized NVIDIA telemetry was accepted: %+v", got)
	}
}

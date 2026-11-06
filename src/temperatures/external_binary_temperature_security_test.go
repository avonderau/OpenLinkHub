package temperatures

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func temperatureProbeFixture(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "temperature-probe")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExternalTemperatureProbeParsesBoundedCelsius(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		want         float32
	}{
		{"positive", "42.67", 42.67},
		{"negative", "-12.25", -12.25},
		{"integer", "63", 63},
		{"nonfinite nan", "NaN", 0},
		{"nonfinite infinity", "+Inf", 0},
		{"nonfinite negative infinity", "-Inf", 0},
		{"implausibly hot", "100000000", 0},
		{"implausibly cold", "-100000000", 0},
		{"invalid", "not a number", 0},
		{"multiple readings", "42.4 52.9", 0},
		{"no reading", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := temperatureProbeFixture(t, "printf '%s\\n' '"+tc.output+"'")
			got := GetExternalBinaryTemperature(path)
			if math.Abs(float64(got-tc.want)) > 0.001 {
				t.Fatalf("reading %q = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

func TestExternalTemperatureProbeRejectsUnboundedOutput(t *testing.T) {
	path := temperatureProbeFixture(t, "printf '%0300d\\n' 1")
	start := time.Now()
	if got := externalBinaryTemperatureWithTimeout(path, time.Second); got != 0 {
		t.Fatalf("oversized probe output accepted as %v", got)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("oversized probe output blocked the caller")
	}
}

func TestExternalTemperatureProbeTimeoutStopsBlockedProcess(t *testing.T) {
	path := temperatureProbeFixture(t, "sleep 5")
	start := time.Now()
	if got := externalBinaryTemperatureWithTimeout(path, 125*time.Millisecond); got != 0 {
		t.Fatalf("timed-out probe returned %v", got)
	}
	if elapsed := time.Since(start); elapsed >= 3*time.Second {
		t.Fatalf("probe blocked for %s despite timeout", elapsed)
	}
}

func TestExternalTemperatureProbeDoesNotRunShellCommands(t *testing.T) {
	// The pathname must be passed as a single executable to exec.Command;
	// it must never be interpreted by a shell.
	path := temperatureProbeFixture(t, "printf '42\\n'")
	if got := GetExternalBinaryTemperature(path + " ; printf 99"); got != 0 {
		t.Fatalf("input with command separators was executed: %v", got)
	}
	if got := GetExternalBinaryTemperature(path); got != 42 {
		t.Fatalf("expected a plain executable path to work, got %v", got)
	}
}

func TestExternalTemperatureOutputWriterBoundsMemory(t *testing.T) {
	out := &boundedTemperatureOutput{cancel: func() {}}
	if n, err := out.Write([]byte("42")); n != 2 || err != nil {
		t.Fatalf("valid small reading rejected: %d %v", n, err)
	}
	if n, err := out.Write([]byte(strings.Repeat("x", 400))); n != 0 || err == nil {
		t.Fatalf("oversized output accepted: %d %v", n, err)
	}
	if out.Len() > externalTemperatureMaxOutput {
		t.Fatalf("output buffer exceeded quota: %d", out.Len())
	}
}

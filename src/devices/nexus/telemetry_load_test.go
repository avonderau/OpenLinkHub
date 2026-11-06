package nexus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLinuxLoadAverage(t *testing.T) {
	for _, tc := range []struct {
		input string
		ok    bool
	}{
		{"1.25 2.50 3.75 2/300 12345", true},
		{"not-a-load 2.5 3.0", false},
		{"-1.0 2.5 3.0", false},
		{"1.0 2.5", false},
	} {
		metrics := parseLinuxLoadAverage(tc.input)
		if (len(metrics) == 3) != tc.ok {
			t.Fatalf("unexpected result %v for %q", metrics, tc.input)
		}
		if tc.ok && (metrics["cpu.load_1m"] != 1.25 || metrics["cpu.load_15m"] != 3.75) {
			t.Fatalf("incorrect load %v", metrics)
		}
	}
}
func TestReadLinuxLoadAverage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "loadavg")
	if err := os.WriteFile(file, []byte("0.50 1.50 2.50 1/100 23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	metrics := readLinuxLoadAverage(file)
	if metrics["cpu.load_5m"] != 1.5 {
		t.Fatalf("unexpected 5m load %v", metrics)
	}
	if len(readLinuxLoadAverage(file+".absent")) != 0 {
		t.Fatal("missing source should omit metrics")
	}
}

package nexus

import (
	"testing"
	"time"
)

func TestCounterRates(t *testing.T) {
	var s CounterSampler
	start := time.Unix(100, 0)
	if s.Sample(LinuxCounters{NetworkRX: 100, DiskReadSectors: 5}, start).Valid {
		t.Fatal("first sample must be unavailable")
	}
	got := s.Sample(LinuxCounters{NetworkRX: 300, DiskReadSectors: 9}, start.Add(2*time.Second))
	if !got.Valid || got.NetworkRXBytesPerSecond != 100 || got.DiskReadBytesPerSecond != 1024 {
		t.Fatalf("bad rates: %+v", got)
	}
	if s.Sample(LinuxCounters{}, start.Add(3*time.Second)).Valid {
		t.Fatal("counter reset accepted")
	}
}
func TestParseNetworkCounters(t *testing.T) {
	data := "Inter-| Receive | Transmit\n eth0: 100 0 0 0 0 0 0 0 250 0 0 0 0 0 0 0\n"
	rx, tx, err := ParseNetworkCounters(data, "eth0")
	if err != nil || rx != 100 || tx != 250 {
		t.Fatalf("network: %d %d %v", rx, tx, err)
	}
}
func TestParseDiskCounters(t *testing.T) {
	read, write, err := ParseDiskCounters("8 0 sda 1 0 42 0 1 0 87 0 0 0 0\n", "sda")
	if err != nil || read != 42 || write != 87 {
		t.Fatalf("disk: %d %d %v", read, write, err)
	}
}

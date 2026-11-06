package devices

import "testing"

func TestLightingNodeProRequiresCorrectHIDUsagePage(t *testing.T) {
	entry, ok := deviceRegisterMap[3083]
	if !ok {
		t.Fatal("Lighting Node Pro is absent from supported registry")
	}
	if got := entry.UsagePage; got != 65472 {
		t.Fatalf("Lighting Node Pro UsagePage = %#x, want %#x", got, 65472)
	}
}

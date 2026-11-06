package nexus

import (
	"testing"
	"time"
)

func TestPresetPulseChangesOverTime(t *testing.T) {
	a := PresetNexusBackground("preset_pulse", time.Unix(100, 0))
	b := PresetNexusBackground("preset_pulse", time.Unix(103, 0))
	if a == nil || b == nil {
		t.Fatal("missing pulse preset")
	}
	changed := false
	for x := 0; x < ScreenWidth; x += 8 {
		if a.At(x, 24) != b.At(x, 24) {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("pulse frame did not animate")
	}
	if PresetNexusBackground("unknown", time.Now()) != nil {
		t.Fatal("unknown preset accepted")
	}
}

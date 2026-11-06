package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Deck model exclusions are intentionally independent of legacy PID-only
// Corsair exclusions: a shared PID must not disable two different brands.
func TestDeckSettingsPersistWithoutChangingCorsairExclusions(t *testing.T) {
	previous, previousLocation := configuration, location
	t.Cleanup(func() { configuration = previous; location = previousLocation })
	location = filepath.Join(t.TempDir(), "config.json")
	configuration = Configuration{Exclude: []uint16{0x6670}, DeckExclude: []string{}}

	if !DeckEnabled("5548:6670") {
		t.Fatal("Deck starts excluded even though only the Corsair PID is excluded")
	}
	if got := UpdateSupportedDeckDevices(map[string]bool{"5548:6670": false, "0fd9:0080": false}); got != 1 {
		t.Fatalf("update result: %d", got)
	}
	if DeckEnabled("5548:6670") || DeckEnabled("0fd9:0080") {
		t.Fatal("disabled Deck models must not be enabled")
	}
	if !reflect.DeepEqual(configuration.Exclude, []uint16{0x6670}) {
		t.Fatal("legacy Corsair exclusion changed")
	}

	content, err := os.ReadFile(location)
	if err != nil {
		t.Fatal(err)
	}
	var saved Configuration
	if err := json.Unmarshal(content, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.DeckExclude) != 2 {
		t.Fatalf("missing exclusions in on-disk config: %+v", saved.DeckExclude)
	}

	UpdateSupportedDeckDevices(map[string]bool{"5548:6670": true})
	if !DeckEnabled("5548:6670") || DeckEnabled("0fd9:0080") {
		t.Fatal("one vendor-qualified toggle affected another")
	}
	if !reflect.DeepEqual(configuration.Exclude, []uint16{0x6670}) {
		t.Fatal("enabling Deck changed legacy exclusions")
	}
}

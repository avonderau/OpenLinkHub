package deck

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestDeckMediaKeysHaveBuiltInLCDGlyphs(t *testing.T) {
	for _, size := range []int{85, 95} {
		background := renderKeyLabel(size, "").(*image.RGBA)
		patterns := map[string][]byte{}
		for _, command := range []string{"play-pause", "previous-track", "next-track",
			"stop", "volume-up", "volume-down", "volume-mute"} {
			key := KeyConfig{Index: 1, Action: "media", Target: command}
			frame, ok := renderDeckMediaKey(size, key).(*image.RGBA)
			if !ok || frame.Bounds() != image.Rect(0, 0, size, size) {
				t.Fatalf("%s: unexpected LCD dimensions", command)
			}
			if bytes.Equal(frame.Pix, background.Pix) {
				t.Fatalf("%s: expected a visible built-in media symbol on %dpx key", command, size)
			}
			patterns[command] = append([]byte(nil), frame.Pix...)
			withLabel := renderDeckMediaKey(size, KeyConfig{Action: "media", Target: command, Label: "Control"})
			if withLabel.Bounds() != frame.Bounds() {
				t.Fatalf("%s: custom label altered LCD bounds", command)
			}
			if pixel := frame.RGBAAt(size/2, size/2); pixel == (color.RGBA{}) {
				t.Fatalf("%s: uninitialized icon center", command)
			}
		}
		if bytes.Equal(patterns["play-pause"], patterns["stop"]) {
			t.Fatal("play/pause and stop must have different symbols")
		}
		if bytes.Equal(patterns["volume-down"], patterns["volume-mute"]) {
			t.Fatal("mute icon must be distinguishable from volume-down")
		}
	}
}

func TestDeckMediaCaptionDefaultsAndCustomOverride(t *testing.T) {
	for _, tc := range []struct{ target, caption string }{
		{"play-pause", "Play/Pause"}, {"previous-track", "Previous"},
		{"next-track", "Next Track"}, {"volume-mute", "Mute"},
	} {
		key := KeyConfig{Action: "media", Target: tc.target}
		if got := keyDisplayLabel(key); got != tc.caption {
			t.Fatalf("default %s: %q", tc.target, got)
		}
		blank := renderDeckMediaKey(85, key).(*image.RGBA)
		explicit := renderDeckMediaKey(85, KeyConfig{Action: "media", Target: tc.target, Label: tc.caption}).(*image.RGBA)
		if !bytes.Equal(blank.Pix, explicit.Pix) {
			t.Fatalf("missing LCD default caption for %s", tc.target)
		}
		custom := renderDeckMediaKey(85, KeyConfig{Action: "media", Target: tc.target, Label: "Custom"}).(*image.RGBA)
		if bytes.Equal(custom.Pix, blank.Pix) {
			t.Fatalf("custom label did not replace default for %s", tc.target)
		}
	}
}

package nexus

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestNexusGestureClassification(t *testing.T) {
	now := time.Unix(500, 0)
	var state touchGestureState
	state.begin(380, now)
	state.update(230)
	_, direction, tap := state.finish(now.Add(300 * time.Millisecond))
	if direction != 1 || tap {
		t.Fatalf("left swipe expected next, got %d/%t", direction, tap)
	}
	state.begin(160, now)
	state.update(320)
	_, direction, tap = state.finish(now.Add(400 * time.Millisecond))
	if direction != -1 || tap {
		t.Fatalf("right swipe expected previous, got %d/%t", direction, tap)
	}
	state.begin(300, now)
	state.update(250)
	position, direction, tap := state.finish(now.Add(100 * time.Millisecond))
	if !tap || direction != 0 || position != 300 {
		t.Fatalf("short move should tap start: %d/%d/%t", position, direction, tap)
	}
	state.begin(360, now)
	state.update(120)
	position, direction, tap = state.finish(now.Add(3 * time.Second))
	if tap || direction != 1 {
		t.Fatal("long-hold swipe should navigate without tapping")
	}
	state.begin(200, now)
	position, direction, tap = state.finish(now.Add(3 * time.Second))
	if tap || direction != 0 {
		t.Fatal("long stationary hold activated a button")
	}
	state.begin(200, now)
	position, direction, tap = state.finish(now.Add(100 * time.Millisecond))
	if !tap || direction != 0 || position != 200 {
		t.Fatal("short tap should execute on release")
	}
	_, direction, tap = state.finish(now.Add(time.Second))
	if tap || direction != 0 {
		t.Fatal("duplicate release triggered action")
	}
}

func TestNexusTouchReportsDispatchSwipeOnBlankAreas(t *testing.T) {
	d := &Device{customScreens: ScreenConfiguration{
		Version: 1, CyclingMode: "manual", ActiveScreenID: "one",
		Screens: []Screen{{ID: "one", Name: "One", Widgets: []Widget{}}, {ID: "two", Name: "Two", Widgets: []Widget{}}},
	}}
	var state touchGestureState
	send := func(active byte, x uint16) {
		data := make([]byte, 8)
		data[5] = active
		binary.LittleEndian.PutUint16(data[6:8], x)
		d.processCustomTouchReport(data, &state)
	}
	send(1, 400)
	if d.customScreens.ActiveScreenID != "one" {
		t.Fatal("blank-area press navigated before release")
	}
	send(1, 250)
	send(0, 0)
	if d.customScreens.ActiveScreenID != "two" {
		t.Fatal("left swipe did not go to next screen")
	}
	send(1, 180)
	send(1, 340)
	send(0, 0)
	if d.customScreens.ActiveScreenID != "one" {
		t.Fatal("right swipe did not go to previous screen")
	}
	// Edge taps remain functional, but dispatch once on release.
	send(1, 620)
	if d.customScreens.ActiveScreenID != "one" {
		t.Fatal("edge tap triggered before release")
	}
	send(0, 0)
	if d.customScreens.ActiveScreenID != "two" {
		t.Fatal("right edge tap stopped working")
	}
	d.customScreens.CyclingMode = "off"
	send(1, 360)
	send(1, 180)
	send(0, 0)
	if d.customScreens.ActiveScreenID != "two" {
		t.Fatal("swipe bypassed fixed-screen mode")
	}
}

func TestNexusButtonTapsAreDeferredAndSwipesTakePrecedence(t *testing.T) {
	d := &Device{customScreens: ScreenConfiguration{
		Version: 1, CyclingMode: "manual", ActiveScreenID: "one",
		Screens: []Screen{
			{ID: "one", Name: "One", Widgets: []Widget{{ID: "next", Type: "button", X: 80, Y: 0, Width: 160, Height: 48, Action: "next"}}},
			{ID: "two", Name: "Two", Widgets: []Widget{}},
			{ID: "three", Name: "Three", Widgets: []Widget{}},
		},
	}}
	var state touchGestureState
	press := make([]byte, 8)
	press[5] = 1
	binary.LittleEndian.PutUint16(press[6:8], 120)
	d.processCustomTouchReport(press, &state)
	if d.customScreens.ActiveScreenID != "one" {
		t.Fatal("button action fired on touch-down")
	}
	move := make([]byte, 8)
	move[5] = 1
	binary.LittleEndian.PutUint16(move[6:8], 360)
	d.processCustomTouchReport(move, &state)
	if d.customScreens.ActiveScreenID != "one" {
		t.Fatal("movement executed the button before release")
	}
	release := make([]byte, 6)
	d.processCustomTouchReport(release, &state)
	if d.customScreens.ActiveScreenID != "three" {
		t.Fatal("rightward button swipe failed to navigate to previous screen")
	}
	d.customScreens.ActiveScreenID = "one"
	// A regular short tap on that same button executes once on release.
	d.processCustomTouchReport(press, &state)
	if d.customScreens.ActiveScreenID != "one" {
		t.Fatal("tap fired before release")
	}
	d.processCustomTouchReport(release, &state)
	if d.customScreens.ActiveScreenID != "two" {
		t.Fatal("button tap did not execute on release")
	}
	d.processCustomTouchReport(release, &state)
	if d.customScreens.ActiveScreenID != "two" {
		t.Fatal("duplicate release executed button twice")
	}
}

func TestNexusTouchReportsRejectOutOfBoundsPosition(t *testing.T) {
	d := &Device{customScreens: ScreenConfiguration{
		Version: 1, CyclingMode: "manual", ActiveScreenID: "first",
		Screens: []Screen{{ID: "first", Name: "First"}, {ID: "other", Name: "Other"}},
	}}
	var state touchGestureState
	bad := make([]byte, 8)
	bad[5] = 1
	binary.LittleEndian.PutUint16(bad[6:8], 65535)
	d.processCustomTouchReport(bad, &state)
	if state.pressed {
		t.Fatal("invalid HID coordinate accepted")
	}
	d.processCustomTouchReport([]byte{0, 0, 0, 0, 0, 0}, &state)
	if d.customScreens.ActiveScreenID != "first" {
		t.Fatal("invalid coordinate navigated screen")
	}
}

func TestNexusSwipeFromAnyActionButton(t *testing.T) {
	for _, tc := range []struct{ action, target string }{
		{"next", ""}, {"previous", ""}, {"screen", "second"},
		{"media", "play-pause"}, {"hotkey", "Ctrl+Alt+T"},
		{"macro", "1"}, {"text", "hello"}, {"url", "https://example.org"},
		{"application", "/usr/bin/true"}, {"device", "demo"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			d := &Device{customScreens: ScreenConfiguration{
				Version: 1, CyclingMode: "manual", ActiveScreenID: "first",
				Screens: []Screen{
					{ID: "first", Name: "Full panel", Widgets: []Widget{
						{ID: "action", Type: "button", X: 0, Y: 0, Width: 640, Height: 48, Action: tc.action, Target: tc.target},
					}},
					{ID: "second", Name: "Next"},
				},
			}}
			var state touchGestureState
			send := func(active byte, x uint16) {
				packet := make([]byte, 8)
				packet[5] = active
				binary.LittleEndian.PutUint16(packet[6:8], x)
				d.processCustomTouchReport(packet, &state)
			}
			send(1, 400)
			if d.customScreens.ActiveScreenID != "first" {
				t.Fatal("action fired on press")
			}
			send(1, 220)
			if d.customScreens.ActiveScreenID != "first" {
				t.Fatal("action fired during drag")
			}
			send(0, 0)
			if d.customScreens.ActiveScreenID != "second" {
				t.Fatal("drag across button did not navigate")
			}
			send(0, 0)
			if d.customScreens.ActiveScreenID != "second" {
				t.Fatal("duplicate release re-ran action")
			}
		})
	}
}

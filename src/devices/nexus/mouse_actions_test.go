package nexus

import (
	"OpenLinkHub/src/inputmanager"
	"testing"
)

func TestNexusMouseCommands(t *testing.T) {
	cases := []struct {
		name   string
		button uint16
		scroll int
		count  int
	}{
		{"left", inputmanager.BtnLeft, 0, 1},
		{"right", inputmanager.BtnRight, 0, 1},
		{"middle", inputmanager.BtnMiddle, 0, 1},
		{"back", inputmanager.BtnBack, 0, 1},
		{"forward", inputmanager.BtnForward, 0, 1},
		{"double-left", inputmanager.BtnLeft, 0, 2},
		{"scroll-up", 0, 1, 1},
		{"scroll-down", 0, -1, 1},
		{"scroll-left", 0, 2, 1},
		{"scroll-right", 0, -2, 1},
	}
	for _, tc := range cases {
		actual, err := parseNexusMouseAction(tc.name)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if actual.button != tc.button || actual.scroll != tc.scroll || actual.count != tc.count {
			t.Errorf("%s: unexpected command %+v", tc.name, actual)
		}
	}
	for _, v := range []string{"", "triple-left", "move:100,100", "scroll-up:1000", "left;echo hi", "CTRL+LEFT", " scroll-up"} {
		if _, err := parseNexusMouseAction(v); err == nil {
			t.Errorf("invalid mouse action accepted: %q", v)
		}
	}
}

func TestNexusMouseConfigValidation(t *testing.T) {
	config := ScreenConfiguration{Version: 1, ActiveScreenID: "mouse", Screens: []Screen{{
		ID: "mouse", Name: "Mouse",
		Widgets: []Widget{{
			ID: "click", Type: "button", Action: "mouse", Target: "double-left", X: 0, Y: 0, Width: 100, Height: 48,
		}, {
			ID: "macro", Type: "button", Action: "macro", X: 120, Y: 0, Width: 100, Height: 48,
			Target: "1",
		}},
	}}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	config.Screens[0].Widgets[0].Target = "scroll-up:1000"
	if err := config.Validate(); err == nil {
		t.Fatal("accepted unbounded mouse scroll")
	}
	config.Screens[0].Widgets[0].Target = "left"
	config.Screens[0].Widgets[1].Target = "unknown"
	if err := config.Validate(); err == nil {
		t.Fatal("accepted invalid saved macro ID")
	}
}

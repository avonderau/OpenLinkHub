package nexus

import "testing"

func TestNexusTextShadowSettings(t *testing.T) {
	for _, value := range []string{"", "none", "soft", "strong"} {
		if !validNexusTextShadow(value) {
			t.Fatalf("valid text shadow %q rejected", value)
		}
	}
	for _, value := range []string{"huge", "blur(10px)", "red", "../foo"} {
		if validNexusTextShadow(value) {
			t.Fatalf("unsupported text shadow %q accepted", value)
		}
	}
	cfg := ScreenConfiguration{Version: 1, ActiveScreenID: "demo", Screens: []Screen{{
		ID: "demo", Name: "Demo", Widgets: []Widget{{
			ID: "text", Type: "text", X: 0, Y: 0, Width: 200, Height: 48,
			Settings: map[string]string{"text": "Shadow", "textShadow": "soft"},
		}},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Screens[0].Widgets[0].Settings["textShadow"] = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid shadow passed layout validation")
	}
}

func TestTextShadowChangesRenderedPixels(t *testing.T) {
	screen := Screen{ID: "demo", Name: "Demo", Widgets: []Widget{{
		ID: "text", Type: "text", X: 0, Y: 0, Width: 200, Height: 48,
		Settings: map[string]string{"text": "SHADOW", "textColor": "#ffffff", "backgroundColor": "#ffffff"},
	}}}
	noShadow, err := RenderScreen(screen, nil)
	if err != nil {
		t.Fatal(err)
	}
	screen.Widgets[0].Settings["textShadow"] = "strong"
	withShadow, err := RenderScreen(screen, nil)
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	for y := 0; y < 48; y++ {
		for x := 0; x < 200; x++ {
			if noShadow.RGBAAt(x, y) != withShadow.RGBAAt(x, y) {
				changed++
			}
		}
	}
	if changed == 0 {
		t.Fatal("text shadow did not affect frame pixels")
	}
}

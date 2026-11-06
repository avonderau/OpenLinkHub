package nexus

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestMediaIconsDrawWithinButton(t *testing.T) {
	icons := []string{"previous-track", "play-pause", "next-track", "stop", "volume-down", "volume-up", "volume-mute", "next", "previous"}
	for _, name := range icons {
		t.Run(name, func(t *testing.T) {
			frame := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
			bounds := image.Rect(100, 0, 200, 48)
			drawMediaIcon(frame, bounds, name, color.RGBA{225, 245, 255, 255})
			lit := 0
			for y := 0; y < ScreenHeight; y++ {
				for x := 0; x < ScreenWidth; x++ {
					c := frame.RGBAAt(x, y)
					if c != (color.RGBA{}) {
						if !image.Pt(x, y).In(bounds) {
							t.Fatalf("icon %s drew outside bounds at %d,%d", name, x, y)
						}
						lit++
					}
				}
			}
			if lit == 0 {
				t.Fatalf("icon %s rendered no pixels", name)
			}
		})
	}
}

func TestMediaButtonIconAppearsOnRenderedScreen(t *testing.T) {
	screen := Screen{ID: "media", Name: "Media", Widgets: []Widget{{
		ID: "play", Type: "button", X: 0, Y: 0, Width: 100, Height: 48,
		Action: "media", Target: "play-pause", Settings: map[string]string{"icon": "play-pause"},
	}}}
	image, err := RenderScreen(screen, nil)
	if err != nil {
		t.Fatal(err)
	}
	background := color.RGBA{27, 41, 53, 255}
	found := false
	for y := 4; y < 40; y++ {
		for x := 15; x < 80; x++ {
			if image.RGBAAt(x, y) != background {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("media control icon was not drawn")
	}
}

func TestStyledMacroButtonRender(t *testing.T) {
	screen := Screen{ID: "styled", Name: "Styled", Widgets: []Widget{{
		ID: "action", Type: "button", X: 0, Y: 0, Width: 96, Height: 48, Action: "macro",
		Target:   "1",
		Settings: map[string]string{"icon": "play-pause", "backgroundColor": "#123456", "textColor": "#aabbcc", "borderColor": "#ff9900", "style": "solid"},
	}}}
	result, err := RenderScreen(screen, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.RGBAAt(20, 2); got != (color.RGBA{0x12, 0x34, 0x56, 255}) {
		t.Fatalf("background color %v", got)
	}
	if got := result.RGBAAt(0, 0); got != (color.RGBA{0xff, 0x99, 0, 255}) {
		t.Fatalf("border color %v", got)
	}
}

func TestPreviousTrackRewindBarOutsideTriangles(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 48, 48))
	drawMediaIcon(frame, frame.Bounds(), "previous-track", color.RGBA{255, 255, 255, 255})
	cy, cx := 21, 24
	// Opaque bar and triangles must be separated by at least three empty columns.
	for y := cy - 7; y <= cy+7; y++ {
		if frame.RGBAAt(cx-15, y).A == 0 {
			t.Fatalf("missing previous-track bar at %d,%d", cx-15, y)
		}
		for x := cx - 12; x <= cx-10; x++ {
			if frame.RGBAAt(x, y).A != 0 {
				t.Fatalf("previous-track bar overlaps rewind triangle at %d,%d", x, y)
			}
		}
	}
	if frame.RGBAAt(cx-9, cy).A == 0 {
		t.Fatal("missing first rewind triangle tip")
	}
}

func TestDefaultMediaLabels(t *testing.T) {
	for _, tc := range []struct{ target, label string }{
		{"play-pause", "Play/Pause"}, {"previous-track", "Previous"},
		{"next-track", "Next Track"}, {"stop", "Stop"},
		{"volume-up", "Volume +"}, {"volume-down", "Volume -"}, {"volume-mute", "Mute"},
	} {
		if got := DefaultMediaLabel(tc.target); got != tc.label {
			t.Fatalf("%s: %q", tc.target, got)
		}
	}
	if DefaultMediaLabel("unknown") != "" {
		t.Fatal("unknown media action has a default caption")
	}
}

func TestNexusMediaCaptionDefaultsAndCustomOverride(t *testing.T) {
	for _, label := range []string{"", "My Playlist"} {
		screen := Screen{ID: "media", Name: "Media", Widgets: []Widget{{
			ID: "prev", Type: "button", X: 0, Y: 0, Width: 150, Height: 48,
			Action: "media", Target: "previous-track",
			Settings: map[string]string{"icon": "previous-track", "label": label},
		}}}
		frame, err := RenderScreen(screen, nil)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Bounds() != image.Rect(0, 0, ScreenWidth, ScreenHeight) {
			t.Fatal("wrong media dimensions")
		}
		actualCaption := label
		if actualCaption == "" {
			actualCaption = DefaultMediaLabel("previous-track")
		}
		expected := screen
		expected.Widgets = []Widget{screen.Widgets[0]}
		expected.Widgets[0].Settings = map[string]string{"icon": "previous-track", "label": actualCaption}
		same, err := RenderScreen(expected, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(frame.Pix, same.Pix) {
			t.Fatalf("custom/default caption failed for %q", label)
		}
	}
}

func TestMediaPreviewMasksMatchDeviceGlyphPixels(t *testing.T) {
	type svgRect struct {
		X      int `xml:"x,attr"`
		Y      int `xml:"y,attr"`
		Width  int `xml:"width,attr"`
		Height int `xml:"height,attr"`
	}
	type svgArtwork struct {
		Rects []svgRect `xml:"g>rect"`
	}
	// SVG mask assets are checked against the actual built-in LCD glyph pixels.
	// This protects both the Nexus and Deck editor previews against icon drift.
	for _, command := range []string{
		"play-pause", "previous-track", "next-track", "stop",
		"volume-up", "volume-down", "volume-mute", "next", "previous",
	} {
		t.Run(command, func(t *testing.T) {
			bytes, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "media-glyphs", command+".svg"))
			if err != nil {
				t.Fatal(err)
			}
			var artwork svgArtwork
			if err = xml.Unmarshal(bytes, &artwork); err != nil {
				t.Fatal(err)
			}
			if len(artwork.Rects) == 0 {
				t.Fatal("preview asset contains no pixel rectangles")
			}
			mask := make([]bool, 48*48)
			for _, rect := range artwork.Rects {
				if rect.Width <= 0 || rect.Height <= 0 || rect.X < 0 || rect.Y < 0 || rect.X+rect.Width > 48 || rect.Y+rect.Height > 48 {
					t.Fatalf("invalid rectangle %+v", rect)
				}
				for y := rect.Y; y < rect.Y+rect.Height; y++ {
					for x := rect.X; x < rect.X+rect.Width; x++ {
						mask[y*48+x] = true
					}
				}
			}
			frame := image.NewRGBA(image.Rect(0, 0, 48, 48))
			drawMediaIcon(frame, frame.Bounds(), command, color.RGBA{255, 255, 255, 255})
			for y := 0; y < 48; y++ {
				for x := 0; x < 48; x++ {
					actual := frame.RGBAAt(x, y).A != 0
					if actual != mask[y*48+x] {
						t.Fatalf("%s preview/LCD pixel mismatch at (%d,%d)", command, x, y)
					}
				}
			}
		})
	}
}

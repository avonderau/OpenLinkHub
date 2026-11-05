package nexus

import (
	"image"
	"image/color"
	"testing"
)

func TestCustomNexusLCDFramePreservesPreviewColorsAndSwapsHardwareChannels(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	frame.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	frame.SetRGBA(1, 0, color.RGBA{B: 255, A: 255})
	frame.SetRGBA(2, 0, color.RGBA{R: 17, G: 34, B: 51, A: 255})

	hardware := encodeCustomNexusLCDFrame(frame)
	if len(hardware) != ScreenWidth*ScreenHeight*4 {
		t.Fatalf("unexpected LCD frame size: %d", len(hardware))
	}
	for _, tc := range []struct {
		offset int
		want   [4]byte
	}{
		{0, [4]byte{0, 0, 255, 255}},
		{4, [4]byte{255, 0, 0, 255}},
		{8, [4]byte{51, 34, 17, 255}},
	} {
		got := [4]byte{hardware[tc.offset], hardware[tc.offset+1], hardware[tc.offset+2], hardware[tc.offset+3]}
		if got != tc.want {
			t.Errorf("LCD byte offset %d: got %v, want %v", tc.offset, got, tc.want)
		}
	}
	if got := frame.RGBAAt(0, 0); got != (color.RGBA{R: 255, A: 255}) {
		t.Errorf("renderer preview was modified: %v", got)
	}
	if got := frame.RGBAAt(1, 0); got != (color.RGBA{B: 255, A: 255}) {
		t.Errorf("renderer preview blue pixel was modified: %v", got)
	}
}

func TestCustomNexusLCDColorMatchesRenderedRedButton(t *testing.T) {
	screen := Screen{ID: "red", Name: "Red", Widgets: []Widget{{
		ID: "button", Type: "button", X: 0, Y: 0, Width: 80, Height: 48,
		Settings: map[string]string{"backgroundColor": "#ff0000"},
	}}}
	frame, err := RenderScreen(screen, nil)
	if err != nil {
		t.Fatal(err)
	}
	red := frame.RGBAAt(20, 4)
	if red.R != 255 || red.G != 0 || red.B != 0 {
		t.Fatalf("preview should display pure red, got %v", red)
	}
	bytes := encodeCustomNexusLCDFrame(frame)
	offset := (4*ScreenWidth + 20)*4
	if bytes[offset] != 0 || bytes[offset+1] != 0 || bytes[offset+2] != 255 || bytes[offset+3] != 255 {
		t.Fatalf("LCD should receive BGR red, got %v", bytes[offset:offset+4])
	}
}

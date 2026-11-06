package nexus

import (
	"image"
	"image/color"
	"image/draw"
)

// drawMediaIcon renders small vector glyphs without external assets or fonts.
func drawMediaIcon(dst *image.RGBA, bounds image.Rectangle, icon string, inkColor color.RGBA) {
	ink := image.NewUniform(inkColor)
	cx, cy := (bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2
	if bounds.Dy() > 25 {
		cy -= 3
	}
	rect := func(x0, y0, x1, y1 int) {
		r := image.Rect(x0, y0, x1, y1).Intersect(bounds)
		if !r.Empty() {
			draw.Draw(dst, r, ink, image.Point{}, draw.Src)
		}
	}
	dot := func(x, y int) {
		if image.Pt(x, y).In(bounds) {
			dst.Set(x, y, ink.C)
		}
	}
	triangle := func(x, y, dir int) {
		for i := 0; i < 12; i++ {
			dx := i
			if dir < 0 {
				dx = -i
			}
			span := (11 - i) / 2
			for j := -span; j <= span; j++ {
				dot(x+dx, y+j)
			}
		}
	}
	switch icon {
	case "play-pause":
		triangle(cx-12, cy, 1)
		rect(cx+5, cy-7, cx+9, cy+8)
		rect(cx+12, cy-7, cx+16, cy+8)
	case "play", "next":
		triangle(cx-6, cy, 1)
	case "previous":
		triangle(cx+6, cy, -1)
	case "pause":
		rect(cx-9, cy-8, cx-3, cy+9)
		rect(cx+3, cy-8, cx+9, cy+9)
	case "stop":
		rect(cx-8, cy-8, cx+9, cy+9)
	case "previous-track":
		rect(cx-16, cy-8, cx-13, cy+9)
		// Keep the bar outside both rewind triangles, matching Next Track.
		// Previously the first triangle extended left through the bar.
		triangle(cx+2, cy, -1)
		triangle(cx+15, cy, -1)
	case "next-track":
		rect(cx+13, cy-8, cx+16, cy+9)
		triangle(cx-15, cy, 1)
		triangle(cx-2, cy, 1)
	case "volume-up", "volume-down", "volume-mute":
		rect(cx-15, cy-4, cx-9, cy+5)
		for i := 0; i < 7; i++ {
			span := 3 + i/2
			for yy := cy - span; yy <= cy+span; yy++ {
				dot(cx-9+i, yy)
			}
		}
		if icon == "volume-mute" {
			// Crossed speaker: distinct from volume down.
			for i := -6; i <= 6; i++ {
				dot(cx+10+i, cy+i)
				dot(cx+10+i, cy-i)
			}
		} else {
			for yy := cy - 6; yy <= cy+6; yy++ {
				dx := 6 - absMedia(yy-cy)/2
				dot(cx+3+dx, yy)
				if icon == "volume-up" {
					dx = 12 - absMedia(yy-cy)
					dot(cx+3+dx, yy)
				}
			}
		}
	}
}
func absMedia(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// DrawDeckMediaGlyph exposes the same bounded, built-in vector media icons
// to the Mirabox renderer. Only the fixed supported media-action set calls it.
func DrawDeckMediaGlyph(dst *image.RGBA, bounds image.Rectangle, command string, ink color.RGBA) {
	drawMediaIcon(dst, bounds, command, ink)
}

// DefaultMediaLabel is the display fallback when the user hasn't entered
// a custom caption. The short names fit on both Mirabox and Nexus.
func DefaultMediaLabel(target string) string {
	switch target {
	case "play-pause":
		return "Play/Pause"
	case "previous-track":
		return "Previous"
	case "next-track":
		return "Next Track"
	case "stop":
		return "Stop"
	case "volume-up":
		return "Volume +"
	case "volume-down":
		return "Volume -"
	case "volume-mute":
		return "Mute"
	default:
		return ""
	}
}

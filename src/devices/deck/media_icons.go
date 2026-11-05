package deck

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"OpenLinkHub/src/devices/nexus"
	xdraw "golang.org/x/image/draw"
)

// Built-in icons appear on media keys unless the user explicitly uploaded
// custom artwork. Render in-process; no external assets or file reads.
func renderDeckMediaKey(size int, key KeyConfig) image.Image {
	canvas := renderKeyLabel(size, "").(*image.RGBA)
	const sourceSize = 48
	icon := image.NewRGBA(image.Rect(0, 0, sourceSize, sourceSize))
	nexus.DrawDeckMediaGlyph(icon, icon.Bounds(), key.Target, color.RGBA{R: 242, G: 207, B: 147, A: 255})
	label := strings.TrimSpace(key.Label)
	glyphSize := size * 80 / 100
	top := (size - glyphSize) / 2
	if label != "" {
		glyphSize = size * 65 / 100
		top = 1
	}
	left := (size - glyphSize) / 2
	xdraw.NearestNeighbor.Scale(canvas, image.Rect(left, top, left+glyphSize, top+glyphSize),
		icon, icon.Bounds(), draw.Over, nil)
	if label != "" {
		drawCenteredDeckString(canvas, label, size-6, color.White)
	}
	return canvas
}

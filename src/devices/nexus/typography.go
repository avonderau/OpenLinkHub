package nexus

import (
	"image"
	"image/color"
	"image/draw"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func validNexusFontSize(value string) bool {
	switch value {
	case "", "normal", "small", "large":
		return true
	}
	return false
}
func validNexusTextShadow(value string) bool {
	switch value {
	case "", "none", "soft", "strong":
		return true
	}
	return false
}
func validNexusVerticalAlign(value string) bool {
	switch value {
	case "", "top", "middle", "bottom":
		return true
	}
	return false
}
func validNexusTextAlign(value string) bool {
	switch value {
	case "", "left", "center", "right":
		return true
	}
	return false
}

var customNexusFonts struct {
	sync.Once
	small font.Face
	large font.Face
}

func nexusFace(size string) font.Face {
	if size != "small" && size != "large" {
		return basicfont.Face7x13
	}
	customNexusFonts.Do(func() {
		parsed, err := opentype.Parse(goregular.TTF)
		if err != nil {
			return
		}
		customNexusFonts.small, _ = opentype.NewFace(parsed, &opentype.FaceOptions{Size: 10, DPI: 72, Hinting: font.HintingFull})
		customNexusFonts.large, _ = opentype.NewFace(parsed, &opentype.FaceOptions{Size: 17, DPI: 72, Hinting: font.HintingFull})
	})
	if size == "small" && customNexusFonts.small != nil {
		return customNexusFonts.small
	}
	if size == "large" && customNexusFonts.large != nil {
		return customNexusFonts.large
	}
	return basicfont.Face7x13
}

func clipNexusTextToFace(face font.Face, text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if font.MeasureString(face, text).Ceil() <= maxWidth {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if font.MeasureString(face, string(runes)).Ceil() <= maxWidth {
			return string(runes)
		}
	}
	return ""
}

func drawStyledNexusText(dst *image.RGBA, w Widget, baseY int, value string, ink color.Color) {
	if value == "" {
		return
	}
	size := w.Settings["fontSize"]
	alignment := w.Settings["textAlign"]
	vertical := w.Settings["verticalAlign"]
	shadow := w.Settings["textShadow"]
	if (size == "" || size == "normal") && (alignment == "" || alignment == "left") && (vertical == "" || vertical == "top") && (shadow == "" || shadow == "none") {
		writeWidgetText(dst, w.X+3, baseY, clipText(value, w.Width), ink)
		return
	}
	face := nexusFace(size)
	text := clipNexusTextToFace(face, value, w.Width-6)
	if text == "" {
		return
	}
	measured := font.MeasureString(face, text).Ceil()
	x := w.X + 3
	switch alignment {
	case "center":
		x = w.X + (w.Width-measured)/2
	case "right":
		x = w.X + w.Width - measured - 3
	}
	bounds := image.Rect(w.X, w.Y, w.X+w.Width, w.Y+w.Height).Intersect(dst.Bounds())
	if bounds.Empty() {
		return
	}
	if vertical != "" && vertical != "top" && (w.Type == "text" || w.Type == "button" || w.Type == "macro") {
		metrics := face.Metrics()
		ascent := metrics.Ascent.Ceil()
		descent := metrics.Descent.Ceil()
		if vertical == "middle" {
			baseY = w.Y + (w.Height-ascent-descent)/2 + ascent
		}
		if vertical == "bottom" {
			baseY = w.Y + w.Height - 3 - descent
		}
	} else if size == "large" && (w.Type == "text" || w.Type == "button" || w.Type == "macro") {
		baseY += 6
	}
	if shadow == "soft" || shadow == "strong" {
		offset := 1
		if shadow == "strong" {
			offset = 2
		}
		shadowColor := color.RGBA{0, 0, 0, 180}
		shadowDrawer := font.Drawer{Dst: dst.SubImage(bounds).(draw.Image), Src: image.NewUniform(shadowColor), Face: face, Dot: fixed.P(x+offset, baseY+offset)}
		shadowDrawer.DrawString(text)
	}
	d := font.Drawer{Dst: dst.SubImage(bounds).(draw.Image), Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, baseY)}
	d.DrawString(text)
}

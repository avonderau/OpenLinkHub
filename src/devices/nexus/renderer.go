package nexus

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// MetricSnapshot is an immutable snapshot supplied by the device's telemetry
// collector. Missing metrics are intentionally displayed as unavailable.
type MetricSnapshot map[string]float64

// RenderScreen draws a validated screen to the Nexus's native 640x48 RGBA
// framebuffer. It does not interact with HID and can be tested independently.
func RenderScreen(screen Screen, values MetricSnapshot) (*image.RGBA, error) {
	return renderNexusScreen(screen, values, "", nil, nil, "")
}

func renderNexusScreen(screen Screen, values MetricSnapshot, iconDir string, background image.Image, history MetricHistory, pressedID string) (*image.RGBA, error) {
	cfg := ScreenConfiguration{Version: 1, ActiveScreenID: screen.ID, Screens: []Screen{screen}}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	if background != nil {
		xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), background, background.Bounds(), draw.Src, nil)
	} else {
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{12, 19, 27, 255}), image.Point{}, draw.Src)
	}
	for _, w := range screen.Widgets {
		bounds := image.Rect(w.X, w.Y, w.X+w.Width, w.Y+w.Height)
		bg := nexusWidgetColor(w.Settings, "backgroundColor", color.RGBA{27, 41, 53, 255})
		ink := nexusWidgetColor(w.Settings, "textColor", color.RGBA{235, 245, 252, 255})
		border := nexusWidgetColor(w.Settings, "borderColor", color.RGBA{128, 189, 217, 255})
		style := w.Settings["style"]
		if style == "" {
			style = "solid"
		}
		// Item styling is rendered to a separate layer so opacity affects the entire item,
		// including text and icons, and the screen background remains visible beneath it.
		target := canvas
		canvas = image.NewRGBA(target.Bounds())
		radius, _ := strconv.Atoi(w.Settings["cornerRadius"])
		if radius < 0 {
			radius = 0
		}
		maxRadius := int(math.Min(float64(bounds.Dx()), float64(bounds.Dy()))) / 2
		if radius > maxRadius {
			radius = maxRadius
		}
		if style != "flat" && style != "outline" {
			direction := w.Settings["gradientDirection"]
			if direction == "horizontal" || direction == "vertical" {
				end := nexusWidgetColor(w.Settings, "gradientColor", bg)
				drawNexusRoundedGradient(canvas, bounds, bg, end, radius, direction == "horizontal")
			} else {
				drawNexusRoundedFill(canvas, bounds, bg, radius)
			}
		}
		if style == "outline" || (style == "solid" && w.Settings["borderColor"] != "") {
			drawNexusRoundedBorder(canvas, bounds, border, radius)
		}
		func() {
			defer func() {
				opacity := 100
				if value, err := strconv.Atoi(w.Settings["opacity"]); err == nil {
					opacity = value
				}
				if opacity < 0 {
					opacity = 0
				}
				if opacity > 100 {
					opacity = 100
				}
				mask := image.NewUniform(color.Alpha{uint8((opacity*255 + 50) / 100)})
				draw.DrawMask(target, bounds, canvas, bounds.Min, mask, image.Point{}, draw.Over)
				canvas = target
			}()
			if w.Type == "image" {
				id := w.Settings["customIcon"]
				if id != "" && iconDir != "" {
					if picture, err := ReadCustomIcon(iconDir, id); err == nil {
						targetW, targetH := bounds.Dx()-4, bounds.Dy()-4
						if targetW > 0 && targetH > 0 {
							scale := math.Min(float64(targetW)/float64(picture.Bounds().Dx()), float64(targetH)/float64(picture.Bounds().Dy()))
							width := int(math.Round(scale * float64(picture.Bounds().Dx())))
							height := int(math.Round(scale * float64(picture.Bounds().Dy())))
							if width > 0 && height > 0 {
								centerX := (bounds.Min.X + bounds.Max.X) / 2
								centerY := (bounds.Min.Y + bounds.Max.Y) / 2
								dest := image.Rect(centerX-width/2, centerY-height/2, centerX+(width+1)/2, centerY+(height+1)/2).Intersect(bounds)
								xdraw.CatmullRom.Scale(canvas, dest, picture, picture.Bounds(), draw.Over, nil)
							}
						}
					}
				}
				return
			}
			label := w.Settings["label"]
			if label == "" {
				label = w.Metric
			}
			if w.Type == "text" {
				label = w.Settings["text"]
			}
			if w.Type == "clock" {
				if label != "" {
					drawStyledNexusText(canvas, w, w.Y+12, label, ink)
				}
				drawStyledNexusText(canvas, w, w.Y+27, formatNexusClock(w.Settings, time.Now()), ink)
				return
			}
			if w.Type == "button" || w.Type == "macro" {
				if label == "" {
					label = defaultNexusButtonLabel(w)
				}
			}
			if (w.Type == "button" || w.Type == "macro") && w.Settings["customIcon"] != "" && iconDir != "" {
				icon, err := ReadCustomIcon(iconDir, w.Settings["customIcon"])
				if err == nil {
					size := 32
					if max := bounds.Dx() - 4; max < size {
						size = max
					}
					if max := bounds.Dy() - 4; max < size {
						size = max
					}
					if size > 0 {
						cx := (bounds.Min.X + bounds.Max.X) / 2
						cy := (bounds.Min.Y + bounds.Max.Y) / 2
						if label != "" && bounds.Dy() > 30 {
							cy -= 5
						}
						destination := image.Rect(cx-size/2, cy-size/2, cx+(size+1)/2, cy+(size+1)/2).Intersect(bounds)
						xdraw.CatmullRom.Scale(canvas, destination, icon, icon.Bounds(), draw.Over, nil)
					}
					if label != "" {
						drawStyledNexusText(canvas, w, w.Y+w.Height-3, label, ink)
					}
					return
				}
			}
			if (w.Type == "button" || w.Type == "macro") && w.Settings["icon"] != "" && w.Settings["icon"] != "none" && w.Settings["icon"] != "custom" {
				drawMediaIcon(canvas, bounds, w.Settings["icon"], ink)
				if label != "" {
					writeWidgetText(canvas, w.X+3, w.Y+w.Height-3, clipText(label, w.Width), ink)
				}
				return
			}
			// Text clipping uses each widget's selected font metrics.
			drawStyledNexusText(canvas, w, w.Y+11, label, ink)
			if w.Type == "graph" {
				drawNexusHistoryGraph(canvas, bounds, w.Metric, history[QualifiedMetricKey(w.Metric, w.Settings["metricSource"])], ink)
				return
			}
			if w.Type == "text" || w.Type == "button" || w.Type == "macro" {
				return
			}
			value, ok := values[QualifiedMetricKey(w.Metric, w.Settings["metricSource"])]
			shown := "--"
			if ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
				shown = fmt.Sprintf("%.1f", value)
			}
			if unit := w.Settings["unit"]; unit != "" && shown != "--" {
				shown += unit
			}
			drawStyledNexusText(canvas, w, w.Y+24, shown, ink)
			if w.Type == "bar" && ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
				pct := math.Max(0, math.Min(100, value))
				barY := w.Y + w.Height - 7
				if barY >= w.Y+25 {
					background := image.Rect(w.X+3, barY, w.X+w.Width-3, barY+3)
					draw.Draw(canvas, background, image.NewUniform(color.RGBA{65, 79, 91, 255}), image.Point{}, draw.Src)
					fill := image.Rect(background.Min.X, barY, background.Min.X+int(float64(background.Dx())*pct/100), barY+3)
					draw.Draw(canvas, fill, image.NewUniform(ink), image.Point{}, draw.Src)
				}
			}
		}()
		if pressedID != "" && w.ID == pressedID && (w.Type == "button" || w.Type == "macro") {
			drawNexusButtonPressFeedback(canvas, bounds)
		}
	}
	return canvas, nil
}
func writeWidgetText(dst *image.RGBA, x, y int, s string, c color.Color) {
	drawer := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	drawer.DrawString(s)
}
func clipText(s string, width int) string {
	limit := (width - 6) / 7
	if limit <= 0 {
		return ""
	}
	r := []rune(strings.ToValidUTF8(s, "?"))
	if len(r) > limit {
		return string(r[:limit])
	}
	return string(r)
}

// RenderScreenWithBackground composites a trusted, decoded image behind widgets.
// Images are scaled to the exact Nexus framebuffer dimensions.
func RenderScreenWithBackground(screen Screen, values MetricSnapshot, background image.Image) (*image.RGBA, error) {
	return RenderScreenWithBackgroundAndIcons(screen, values, background, "")
}

func RenderScreenWithBackgroundAndIcons(screen Screen, values MetricSnapshot, background image.Image, iconDir string) (*image.RGBA, error) {
	return renderNexusScreen(screen, values, iconDir, background, nil, "")
}

// RenderScreenWithBackgroundAndIconsHistoryPressed highlights only the
// momentarily touched button; this state is never saved into layouts.
func RenderScreenWithBackgroundAndIconsHistoryPressed(screen Screen, values MetricSnapshot, background image.Image, iconDir string, history MetricHistory, pressedID string) (*image.RGBA, error) {
	return renderNexusScreen(screen, values, iconDir, background, history, pressedID)
}

// Draw a translucent white press highlight over the whole widget, including
// custom icons and text. It intentionally does not alter normal widget colors.
func drawNexusButtonPressFeedback(canvas *image.RGBA, bounds image.Rectangle) {
	mask := image.NewUniform(color.Alpha{65})
	draw.DrawMask(canvas, bounds, image.NewUniform(color.White), image.Point{}, mask, image.Point{}, draw.Over)
}

// ReadBackground returns the current frame for either a static or animated
// background, using the shared bounded image cache.
func RenderScreenWithBackgroundAndIconsHistory(screen Screen, values MetricSnapshot, background image.Image, iconDir string, history MetricHistory) (*image.RGBA, error) {
	return renderNexusScreen(screen, values, iconDir, background, history, "")
}

func ReadBackground(dir, id string) (image.Image, error) {
	return ReadBackgroundAt(dir, id, time.Now())
}

func nexusWidgetColor(settings map[string]string, key string, fallback color.RGBA) color.RGBA {
	value := settings[key]
	if !validHexColor(value) {
		return fallback
	}
	n, err := strconv.ParseUint(value[1:], 16, 24)
	if err != nil {
		return fallback
	}
	return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
}

// Rounded edges are applied to the background and border, not clipped text.
func nexusInsideRounded(r image.Rectangle, x, y, radius int) bool {
	if !image.Pt(x, y).In(r) {
		return false
	}
	if radius <= 0 {
		return true
	}
	cx, cy := x, y
	if x < r.Min.X+radius {
		cx = r.Min.X + radius - 1
	} else if x >= r.Max.X-radius {
		cx = r.Max.X - radius
	}
	if y < r.Min.Y+radius {
		cy = r.Min.Y + radius - 1
	} else if y >= r.Max.Y-radius {
		cy = r.Max.Y - radius
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}
func drawNexusRoundedFill(dst *image.RGBA, r image.Rectangle, c color.RGBA, radius int) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if nexusInsideRounded(r, x, y, radius) {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}
func drawNexusRoundedBorder(dst *image.RGBA, r image.Rectangle, c color.RGBA, radius int) {
	inner := r.Inset(1)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if nexusInsideRounded(r, x, y, radius) && !nexusInsideRounded(inner, x, y, int(math.Max(0, float64(radius-1)))) {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

// drawNexusRoundedGradient interpolates the selected fill colors across an
// item's clipped bounds. A one-pixel axis safely uses the start color.
func drawNexusRoundedGradient(dst *image.RGBA, r image.Rectangle, start, end color.RGBA, radius int, horizontal bool) {
	steps := r.Dy() - 1
	if horizontal {
		steps = r.Dx() - 1
	}
	if steps < 1 {
		steps = 1
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if !nexusInsideRounded(r, x, y, radius) {
				continue
			}
			index := y - r.Min.Y
			if horizontal {
				index = x - r.Min.X
			}
			mix := func(a, b uint8) uint8 { return uint8((int(a)*(steps-index) + int(b)*index + steps/2) / steps) }
			dst.SetRGBA(x, y, color.RGBA{R: mix(start.R, end.R), G: mix(start.G, end.G), B: mix(start.B, end.B), A: 255})
		}
	}
}

// Explicit labels always win. Defaults are calculated only for display,
// without persisting a synthetic label into a saved widget.
func defaultNexusButtonLabel(w Widget) string {
	if w.Type == "macro" {
		return "Run Macro"
	}
	if w.Type != "button" {
		return ""
	}
	switch w.Action {
	case "media":
		return DefaultMediaLabel(w.Target)
	case "next":
		return "Next Screen"
	case "previous":
		return "Previous Screen"
	case "screen":
		return "Go to Screen"
	case "macro":
		return "Run Macro"
	case "hotkey":
		return "Shortcut"
	case "mouse":
		return "Mouse"
	case "text":
		return "Type Text"
	case "url":
		return "Open URL"
	case "application":
		return "Launch App"
	case "device":
		return "Device"
	default:
		return "Touch"
	}
}

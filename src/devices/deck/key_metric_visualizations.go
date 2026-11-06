package deck

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// The style field is additive: existing saved layouts with showGraph=true
// continue to display the original line graph after upgrading.
func deckMetricVisualizationStyle(key KeyConfig) string {
	if key.MetricStyle != "" {
		return key.MetricStyle
	}
	if key.ShowGraph {
		return "line"
	}
	return "value"
}

var defaultDeckMetricAccent = color.RGBA{R: 213, G: 166, B: 90, A: 255}

// Only #RRGGBB colors are allowed. Missing values preserve existing layouts.
func deckMetricAccent(key KeyConfig) color.RGBA {
	if !deckMetricColorPattern.MatchString(key.MetricColor) {
		return defaultDeckMetricAccent
	}
	rgb, err := strconv.ParseUint(key.MetricColor[1:], 16, 24)
	if err != nil {
		return defaultDeckMetricAccent
	}
	return color.RGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 255}
}

func renderDeckMetricVisualization(dst *image.RGBA, key KeyConfig, value float64, available bool, history []float64) {
	ink := deckMetricAccent(key)
	switch deckMetricVisualizationStyle(key) {
	case "line":
		drawDeckSparkline(dst, history, key.Metric, ink)
	case "bars":
		drawDeckHistoryBars(dst, history, key.Metric, ink)
	case "fuel":
		drawDeckFuelGauge(dst, deckMetricGaugeFraction(key.Metric, value, history), available && deckFinite(value), ink)
	case "ring":
		drawDeckRingGauge(dst, deckMetricGaugeFraction(key.Metric, value, history), available && deckFinite(value), ink)
	}
}

func deckFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// Utilization readings represent absolute percentages. Temperature uses a
// practical 0–100 C scale. Other readings use a scale of 120% of the recent
// maximum, rather than inventing a device-specific maximum for RPM or bytes/s.
func deckMetricGaugeFraction(metric string, value float64, history []float64) float64 {
	if !deckFinite(value) {
		return 0
	}
	maximum := 0.0
	switch {
	case strings.HasSuffix(metric, ".utilization"):
		maximum = 100
	case strings.HasSuffix(metric, ".temperature"):
		maximum = 100
	default:
		maximum = math.Max(0, value)
		for _, sample := range history {
			if deckFinite(sample) && sample > maximum {
				maximum = sample
			}
		}
		maximum *= 1.2
	}
	if maximum <= 0 || !deckFinite(maximum) {
		return 0
	}
	return math.Max(0, math.Min(1, value/maximum))
}

// The bar graph plots the history (not just the current value); missing
// samples leave gaps instead of producing artificial zero spikes.
func drawDeckHistoryBars(dst *image.RGBA, history []float64, metric string, ink color.RGBA) {
	if len(history) == 0 {
		return
	}
	size := dst.Bounds().Dx()
	left, right := 6, size-6
	top, bottom := size-23, size-6
	if right <= left || bottom <= top {
		return
	}
	maximum := 0.0
	for _, v := range history {
		if deckFinite(v) && v > maximum {
			maximum = v
		}
	}
	if strings.HasSuffix(metric, ".utilization") {
		maximum = 100
	}
	if maximum <= 0 {
		return
	}
	for i, value := range history {
		if !deckFinite(value) {
			continue
		}
		x0 := left + i*(right-left)/len(history)
		x1 := left + (i+1)*(right-left)/len(history) - 1
		if x1 < x0 {
			x1 = x0
		}
		if x0 >= right {
			continue
		}
		if x1 >= right {
			x1 = right - 1
		}
		fraction := math.Max(0, math.Min(1, value/maximum))
		height := int(math.Round(fraction * float64(bottom-top)))
		if height == 0 && value > 0 {
			height = 1
		}
		for y := bottom - height; y < bottom; y++ {
			for x := x0; x <= x1; x++ {
				dst.SetRGBA(x, y, ink)
			}
		}
	}
}

// A segmented horizontal fuel gauge leaves enough room for the actual
// reading; only finite, currently available values fill segments.
func drawDeckFuelGauge(dst *image.RGBA, fraction float64, available bool, accent color.RGBA) {
	size := dst.Bounds().Dx()
	if size < 32 {
		return
	}
	const segments = 10
	filled := int(math.Round(fraction * segments))
	left, width := 6, size-12
	for i := 0; i < segments; i++ {
		x0 := left + i*width/segments + 1
		x1 := left + (i+1)*width/segments - 2
		ink := color.RGBA{R: 52, G: 64, B: 76, A: 255}
		if available && i < filled {
			ink = accent
		}
		for y := size - 20; y < size-7; y++ {
			for x := x0; x <= x1; x++ {
				dst.SetRGBA(x, y, ink)
			}
		}
	}
}

// Place the ring below the metric label. Unlike the previous small
// bottom-only indicator, it has a clear center for the numeric reading.
func deckRingGeometry(size int) (cx, cy, radius int) {
	radius = (size - 28) / 2
	return size / 2, size - radius - 5, radius
}

func drawDeckRingGauge(dst *image.RGBA, fraction float64, available bool, accent color.RGBA) {
	size := dst.Bounds().Dx()
	if size < 52 {
		return
	}
	cx, cy, radius := deckRingGeometry(size)
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			distance := math.Hypot(float64(x-cx), float64(y-cy))
			if distance < float64(radius-4) || distance > float64(radius) {
				continue
			}
			theta := math.Atan2(float64(y-cy), float64(x-cx)) + math.Pi/2
			if theta < 0 {
				theta += 2 * math.Pi
			}
			ink := color.RGBA{R: 52, G: 64, B: 76, A: 255}
			if available && theta <= 2*math.Pi*fraction {
				ink = accent
			}
			dst.SetRGBA(x, y, ink)
		}
	}
}

// Draw the live reading in the center of the ring, not above it.
// Long readouts are split into a numeric line and a unit line, keeping
// both inside the narrow 85x85 LCD key without spilling into the ring.
func drawDeckRingValue(dst *image.RGBA, value string, accent color.RGBA) {
	size := dst.Bounds().Dx()
	if size < 52 {
		return
	}
	cx, cy, radius := deckRingGeometry(size)
	innerWidth := 2 * (radius - 5)
	parts := strings.SplitN(value, " ", 2)
	if len(parts) == 2 && parts[1] != "" {
		drawDeckRingText(dst, parts[0], cx, cy-1, innerWidth, color.White)
		drawDeckRingText(dst, parts[1], cx, cy+12, innerWidth, accent)
		return
	}
	drawDeckRingText(dst, value, cx, cy+5, innerWidth, color.White)
}

// Center text within the inner diameter. Truncate exceptionally long
// sensor values (e.g. unbounded RPM) rather than painting over the arc.
func drawDeckRingText(dst *image.RGBA, text string, cx, baseline, maxWidth int, ink color.Color) {
	runes := []rune(strings.TrimSpace(text))
	for len(runes) > 0 && font.MeasureString(basicfont.Face7x13, string(runes)).Ceil() > maxWidth {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == 0 {
		return
	}
	label := string(runes)
	width := font.MeasureString(basicfont.Face7x13, label).Ceil()
	drawer := font.Drawer{
		Dst: dst, Src: image.NewUniform(ink),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(cx-width/2, baseline),
	}
	drawer.DrawString(label)
}

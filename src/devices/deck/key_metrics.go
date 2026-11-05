package deck

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"OpenLinkHub/src/devices/nexus"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const maxDeckMetricHistory = 24

var metricShortName = map[string]string{
	"cpu.temperature": "CPU Temp", "gpu.temperature": "GPU Temp",
	"cpu.utilization": "CPU Load", "gpu.utilization": "GPU Load",
	"memory.utilization": "RAM", "memory.used": "RAM Used",
	"fan.rpm": "Fan", "pump.rpm": "Pump",
	"cpu.clock_mhz": "CPU Clock", "gpu.clock_mhz": "GPU Clock",
	"gpu.vram.utilization": "VRAM", "gpu.vram.used_mb": "VRAM Used",
	"gpu.power_w": "GPU Power", "disk.utilization": "Disk",
	"disk.free_gb": "Free Disk", "network.rx_rate": "Net RX",
	"network.tx_rate": "Net TX", "disk.read_rate": "Disk Read",
	"disk.write_rate": "Disk Write",
	"cpu.load_1m":     "Load 1m", "cpu.load_5m": "Load 5m",
	"cpu.load_15m": "Load 15m", "gpu.vram.total_mb": "Total VRAM",
}

func deckMetricInterval(key KeyConfig) time.Duration {
	if key.RefreshSeconds >= 2 && key.RefreshSeconds <= 60 {
		return time.Duration(key.RefreshSeconds) * time.Second
	}
	return 5 * time.Second
}

func metricDueKeys(page Page, updated map[string]time.Time, now time.Time) []KeyConfig {
	var due []KeyConfig
	for _, key := range page.Keys {
		if key.Metric == "" {
			continue
		}
		stamp := updated[page.ID+"/"+strconv.Itoa(key.Index)]
		if stamp.IsZero() || now.Sub(stamp) >= deckMetricInterval(key) {
			due = append(due, key)
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].Index < due[j].Index })
	return due
}

func deckMetricNumber(key string, value float64) string {
	switch {
	case strings.HasSuffix(key, ".temperature"):
		return fmt.Sprintf("%.0f C", value)
	case strings.HasSuffix(key, ".utilization"):
		return fmt.Sprintf("%.0f%%", value)
	case strings.HasSuffix(key, ".rpm"):
		return fmt.Sprintf("%.0f rpm", value)
	case key == "memory.used":
		return fmt.Sprintf("%.1f GiB", value/(1024*1024*1024))
	case strings.HasSuffix(key, "_rate"):
		if math.Abs(value) >= 1e6 {
			return fmt.Sprintf("%.1f MB/s", value/1e6)
		}
		return fmt.Sprintf("%.0f KB/s", value/1e3)
	case strings.HasSuffix(key, "_mb"):
		return fmt.Sprintf("%.0f MB", value)
	case strings.HasSuffix(key, "_mhz"):
		return fmt.Sprintf("%.0f MHz", value)
	case strings.HasSuffix(key, "_w"):
		return fmt.Sprintf("%.0f W", value)
	case key == "disk.free_gb":
		return fmt.Sprintf("%.0f GB", value)
	default:
		return fmt.Sprintf("%.1f", value)
	}
}

// Draw a compact metric card over the same uploaded artwork as static keys.
func renderDeckMetric(size int, key KeyConfig, value float64, available bool, history []float64, artwork image.Image) image.Image {
	var canvas *image.RGBA
	if artwork != nil {
		canvas = renderDeckArtwork(size, artwork, "").(*image.RGBA)
		draw.DrawMask(canvas, canvas.Bounds(), image.NewUniform(color.Black), image.Point{},
			image.NewUniform(color.Alpha{A: 160}), image.Point{}, draw.Over)
	} else {
		canvas = renderKeyLabel(size, "").(*image.RGBA)
	}
	label := key.Label
	if label == "" {
		label = metricShortName[key.Metric]
	}
	if label == "" {
		label = key.Metric
	}
	drawCenteredDeckString(canvas, label, 14, color.RGBA{R: 244, G: 210, B: 151, A: 255})
	text := "--"
	if available && !math.IsNaN(value) && !math.IsInf(value, 0) {
		text = deckMetricNumber(key.Metric, value)
	}
	ink := deckMetricAccent(key)
	if deckMetricVisualizationStyle(key) == "ring" {
		drawDeckRingGauge(canvas, deckMetricGaugeFraction(key.Metric, value, history), available && deckFinite(value), ink)
		drawDeckRingValue(canvas, text, ink)
	} else {
		textInk := color.Color(color.White)
		if deckMetricVisualizationStyle(key) == "value" && key.MetricColor != "" {
			textInk = ink
		}
		drawCenteredDeckString(canvas, text, size/2+6, textInk)
		renderDeckMetricVisualization(canvas, key, value, available, history)
	}
	return canvas
}

func drawCenteredDeckString(dst *image.RGBA, label string, baseline int, ink color.Color) {
	width := dst.Bounds().Dx()
	text := []rune(strings.TrimSpace(label))
	for len(text) > 0 && font.MeasureString(basicfont.Face7x13, string(text)).Ceil() > width-6 {
		text = text[:len(text)-1]
	}
	if len(text) == 0 {
		return
	}
	textWidth := font.MeasureString(basicfont.Face7x13, string(text)).Ceil()
	drawer := font.Drawer{Dst: dst, Src: image.NewUniform(ink), Face: basicfont.Face7x13,
		Dot: fixed.P((width-textWidth)/2, baseline)}
	drawer.DrawString(string(text))
}

func drawDeckSparkline(dst *image.RGBA, samples []float64, metric string, ink color.RGBA) {
	if len(samples) < 2 {
		return
	}
	size := dst.Bounds().Dx()
	xStart, yStart := 6, size-23
	width, height := size-12, 17
	if width < 4 || height < 4 {
		return
	}
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for _, sample := range samples {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			continue
		}
		if sample < minimum {
			minimum = sample
		}
		if sample > maximum {
			maximum = sample
		}
	}
	if math.IsInf(minimum, 1) {
		return
	}
	if strings.HasSuffix(metric, ".utilization") {
		minimum, maximum = 0, 100
	}
	if maximum-minimum < 1 {
		maximum += .5
		minimum -= .5
	}
	var previous image.Point
	hasPrevious := false
	for i, sample := range samples {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			hasPrevious = false
			continue
		}
		x := xStart + int(float64(i)*float64(width-1)/float64(len(samples)-1))
		fraction := (sample - minimum) / (maximum - minimum)
		fraction = math.Max(0, math.Min(1, fraction))
		y := yStart + height - 1 - int(fraction*float64(height-1))
		point := image.Pt(x, y)
		if hasPrevious {
			drawDeckLine(dst, previous, point, ink)
		}
		previous, hasPrevious = point, true
	}
}

func drawDeckLine(dst *image.RGBA, a, b image.Point, ink color.RGBA) {
	dx := int(math.Abs(float64(b.X - a.X)))
	dy := -int(math.Abs(float64(b.Y - a.Y)))
	sx, sy := -1, -1
	if a.X < b.X {
		sx = 1
	}
	if a.Y < b.Y {
		sy = 1
	}
	e := dx + dy
	for {
		if a.In(dst.Bounds()) {
			dst.SetRGBA(a.X, a.Y, ink)
		}
		if a == b {
			return
		}
		twice := 2 * e
		if twice >= dy {
			e += dy
			a.X += sx
		}
		if twice <= dx {
			e += dx
			a.Y += sy
		}
	}
}

func copyDeckMetrics(values nexus.MetricSnapshot) map[string]float64 {
	copyOf := make(map[string]float64, len(values))
	for name, value := range values {
		if nexus.IsSupportedMetric(name) && !math.IsNaN(value) && !math.IsInf(value, 0) {
			copyOf[name] = value
		}
	}
	return copyOf
}

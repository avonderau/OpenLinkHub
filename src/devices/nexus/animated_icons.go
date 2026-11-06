package nexus

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
)

const (
	maxAnimatedIconFrames    = 32
	maxAnimatedIconDimension = 128
)

// decodeAnimatedCustomIcon validates an uploaded GIF before expanding frames,
// then composites GIF disposal modes into transparent 32x32 RGBA frames.
func decodeAnimatedCustomIcon(data []byte) (*animatedBackground, error) {
	width, height, count, err := inspectGIF(data)
	if err != nil {
		return nil, err
	}
	if width > maxAnimatedIconDimension || height > maxAnimatedIconDimension || count > maxAnimatedIconFrames {
		return nil, errors.New("GIF icon exceeds 128x128 or 32 frames")
	}
	source, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(source.Image) != count {
		return nil, errors.New("GIF icon frame count mismatch")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	animation := &animatedBackground{
		frames: make([]*image.RGBA, 0, count),
		delays: make([]time.Duration, 0, count),
	}
	var background color.Color = color.Transparent
	if palette, ok := source.Config.ColorModel.(color.Palette); ok && int(source.BackgroundIndex) < len(palette) {
		background = palette[source.BackgroundIndex]
	}
	for i, frame := range source.Image {
		bounds := frame.Bounds().Intersect(canvas.Bounds())
		var previous *image.RGBA
		if i < len(source.Disposal) && source.Disposal[i] == gif.DisposalPrevious {
			previous = image.NewRGBA(canvas.Bounds())
			draw.Draw(previous, previous.Bounds(), canvas, image.Point{}, draw.Src)
		}
		draw.Draw(canvas, bounds, frame, bounds.Min, draw.Over)
		image32 := image.NewRGBA(image.Rect(0, 0, 32, 32))
		scale := float64(32) / float64(width)
		if other := float64(32) / float64(height); other < scale {
			scale = other
		}
		dx, dy := int(float64(width)*scale), int(float64(height)*scale)
		if dx < 1 {
			dx = 1
		}
		if dy < 1 {
			dy = 1
		}
		destination := image.Rect((32-dx)/2, (32-dy)/2, (32-dx)/2+dx, (32-dy)/2+dy)
		xdraw.CatmullRom.Scale(image32, destination, canvas, canvas.Bounds(), draw.Over, nil)
		animation.frames = append(animation.frames, image32)
		delay := minAnimationDelay
		if i < len(source.Delay) && source.Delay[i] > delay {
			delay = source.Delay[i]
		}
		if delay > 1000 {
			delay = 1000
		}
		duration := time.Duration(delay) * 10 * time.Millisecond
		animation.delays = append(animation.delays, duration)
		animation.duration += duration
		if i < len(source.Disposal) {
			switch source.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, bounds, image.NewUniform(background), image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				if previous != nil {
					draw.Draw(canvas, canvas.Bounds(), previous, image.Point{}, draw.Src)
				}
			}
		}
	}
	return animation, nil
}

func customIconFilePath(dir, id string) (string, error) {
	if !safeID.MatchString(id) {
		return "", errors.New("invalid icon ID")
	}
	for _, ext := range []string{".gif", ".png"} {
		path := filepath.Join(dir, id+ext)
		stat, err := os.Stat(path)
		if err == nil && stat.Mode().IsRegular() {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func IsAnimatedCustomIcon(dir, id string) bool {
	path, err := customIconFilePath(dir, id)
	return err == nil && filepath.Ext(path) == ".gif"
}

type cachedIcon struct {
	path     string
	modified time.Time
	size     int64
	frame    image.Image
	animated *animatedBackground
}

var customIconCache = struct {
	sync.Mutex
	entries []cachedIcon
}{}

func invalidateCustomIconCache(dir, id string) {
	customIconCache.Lock()
	defer customIconCache.Unlock()
	filtered := customIconCache.entries[:0]
	for _, entry := range customIconCache.entries {
		if entry.path != filepath.Join(dir, id+".gif") && entry.path != filepath.Join(dir, id+".png") {
			filtered = append(filtered, entry)
		}
	}
	customIconCache.entries = filtered
}

func ReadCustomIconAt(dir, id string, now time.Time) (image.Image, error) {
	path, err := customIconFilePath(dir, id)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if stat.Size() < 1 || stat.Size() > maxCustomIconBytes {
		return nil, errors.New("invalid custom icon size")
	}
	customIconCache.Lock()
	defer customIconCache.Unlock()
	for _, entry := range customIconCache.entries {
		if entry.path == path && entry.size == stat.Size() && entry.modified.Equal(stat.ModTime()) {
			if entry.animated != nil {
				return entry.animated.frameAt(now), nil
			}
			return entry.frame, nil
		}
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entry cachedIcon
	entry.path = path
	entry.size = stat.Size()
	entry.modified = stat.ModTime()
	if filepath.Ext(path) == ".gif" {
		entry.animated, err = decodeAnimatedCustomIcon(file)
		if err != nil {
			return nil, err
		}
	} else {
		entry.frame, err = png.Decode(io.LimitReader(bytes.NewReader(file), maxCustomIconBytes))
		if err != nil {
			return nil, err
		}
		if entry.frame.Bounds() != image.Rect(0, 0, 32, 32) {
			return nil, errors.New("invalid stored icon dimensions")
		}
	}
	customIconCache.entries = append(customIconCache.entries, entry)
	if len(customIconCache.entries) > 8 {
		customIconCache.entries = customIconCache.entries[len(customIconCache.entries)-8:]
	}
	if entry.animated != nil {
		return entry.animated.frameAt(now), nil
	}
	return entry.frame, nil
}

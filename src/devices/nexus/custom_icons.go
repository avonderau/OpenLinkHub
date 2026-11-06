package nexus

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/image/draw"
)

const maxCustomIconBytes = 512 * 1024

// SaveCustomIcon stores bounded PNG/JPEG artwork as 32x32 PNG, or retains
// safe GIF artwork for animated rendering on the Nexus.
// Stored IDs, rather than user-provided filenames, are referenced by widgets.
func SaveCustomIcon(dir, id string, contents []byte) (string, error) {
	if !safeID.MatchString(id) {
		return "", fmt.Errorf("invalid icon ID")
	}
	if len(contents) == 0 || len(contents) > maxCustomIconBytes {
		return "", fmt.Errorf("icon must be at most 512 KiB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(contents))
	if err != nil {
		return "", fmt.Errorf("invalid icon: %w", err)
	}
	if format != "png" && format != "jpeg" && format != "gif" {
		return "", fmt.Errorf("only PNG, JPEG or GIF icons are supported")
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 512 || config.Height > 512 {
		return "", fmt.Errorf("icon dimensions must be 1 to 512 pixels")
	}
	if format == "gif" {
		if _, err := decodeAnimatedCustomIcon(contents); err != nil {
			return "", fmt.Errorf("invalid animated icon: %w", err)
		}
		if err = os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
		temp, err := os.CreateTemp(dir, ".nexus-icon-*")
		if err != nil {
			return "", err
		}
		defer os.Remove(temp.Name())
		if err = temp.Chmod(0600); err != nil {
			temp.Close()
			return "", err
		}
		if _, err = temp.Write(contents); err != nil {
			temp.Close()
			return "", err
		}
		if err = temp.Sync(); err != nil {
			temp.Close()
			return "", err
		}
		if err = temp.Close(); err != nil {
			return "", err
		}
		path := filepath.Join(dir, id+".gif")
		if err = os.Rename(temp.Name(), path); err != nil {
			return "", err
		}
		_ = os.Remove(filepath.Join(dir, id+".png"))
		invalidateCustomIconCache(dir, id)
		return path, nil
	}
	source, _, err := image.Decode(bytes.NewReader(contents))
	if err != nil {
		return "", fmt.Errorf("invalid icon pixels: %w", err)
	}
	// Fit the artwork in a 32x32 RGBA image while retaining transparency and aspect ratio.
	icon := image.NewRGBA(image.Rect(0, 0, 32, 32))
	scale := float64(32) / float64(config.Width)
	if s := float64(32) / float64(config.Height); s < scale {
		scale = s
	}
	width := int(float64(config.Width) * scale)
	height := int(float64(config.Height) * scale)
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	target := image.Rect((32-width)/2, (32-height)/2, (32+width)/2, (32+height)/2)
	draw.CatmullRom.Scale(icon, target, source, source.Bounds(), draw.Over, nil)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".nexus-icon-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return "", err
	}
	if err = png.Encode(f, icon); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".png")
	if err = os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Join(dir, id+".gif"))
	invalidateCustomIconCache(dir, id)
	return path, nil
}

func ReadCustomIcon(dir, id string) (image.Image, error) {
	return ReadCustomIconAt(dir, id, time.Now())
}

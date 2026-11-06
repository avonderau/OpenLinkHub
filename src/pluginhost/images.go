package pluginhost

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"OpenLinkHub/src/deckplugin"
	"OpenLinkHub/src/devices/nexus"
)

const maxImagePixels = 1024

// imageLoader turns plugin artwork into PNG or JPEG bytes for the service.
// SVG is rasterized here, in the desktop session, with rsvg-convert when it
// is installed; the service never parses SVG from plugins.
type imageLoader struct {
	mu    sync.Mutex
	cache map[string][]byte
	svg   func([]byte) ([]byte, error)
}

func newImageLoader() *imageLoader {
	return &imageLoader{cache: map[string][]byte{}, svg: rasterizeSVG}
}

// manifestImage loads a manifest state image. Paths have no extension;
// Stream Deck looks for @2x.png, .png and .svg variants.
func (l *imageLoader) manifestImage(dir, base string) []byte {
	if base == "" {
		return nil
	}
	key := dir + "\x00" + base
	l.mu.Lock()
	defer l.mu.Unlock()
	if data, ok := l.cache[key]; ok {
		return data
	}
	var out []byte
	for _, candidate := range []string{base + "@2x.png", base + ".png", base + ".jpg", base + ".svg", base} {
		full, err := pluginPath(dir, candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() || info.Size() > deckplugin.MaxImageBytes {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if out, err = normalizeImage(data, l.svg); err == nil {
			break
		}
	}
	l.cache[key] = out
	return out
}

// decodeDataURL accepts the data: URLs plugins pass to setImage.
func decodeDataURL(raw string, svg func([]byte) ([]byte, error)) ([]byte, error) {
	if !strings.HasPrefix(raw, "data:") {
		return nil, errors.New("setImage expects a data: URL")
	}
	meta, body, ok := strings.Cut(raw[len("data:"):], ",")
	if !ok {
		return nil, errors.New("malformed data URL")
	}
	if len(body) > deckplugin.MaxImageBytes*2 {
		return nil, errors.New("image too large")
	}
	var data []byte
	var err error
	if strings.HasSuffix(meta, ";base64") {
		data, err = base64.StdEncoding.DecodeString(body)
	} else {
		var text string
		text, err = url.PathUnescape(body)
		data = []byte(text)
	}
	if err != nil {
		return nil, errors.New("malformed data URL")
	}
	return normalizeImage(data, svg)
}

// normalizeImage returns PNG or JPEG bytes no larger than the shared limit.
func normalizeImage(data []byte, svg func([]byte) ([]byte, error)) ([]byte, error) {
	if len(data) == 0 || len(data) > deckplugin.MaxImageBytes {
		return nil, errors.New("image too large")
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<svg")) || bytes.HasPrefix(trimmed, []byte("<?xml")) {
		if svg == nil {
			return nil, errUnsupportedImage
		}
		png, err := svg(data)
		if err != nil {
			return nil, err
		}
		return checkRaster(png)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errUnsupportedImage
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxImagePixels || cfg.Height > maxImagePixels {
		return nil, errors.New("image dimensions out of range")
	}
	switch format {
	case "png", "jpeg":
		return data, nil
	case "gif":
		// Keys are static for now: show the first frame.
		frame, err := gif.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, frame); err != nil {
			return nil, err
		}
		return checkRaster(buf.Bytes())
	}
	return nil, errUnsupportedImage
}

func checkRaster(data []byte) ([]byte, error) {
	if len(data) > deckplugin.MaxImageBytes {
		return nil, errors.New("image too large")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") ||
		cfg.Width > maxImagePixels || cfg.Height > maxImagePixels {
		return nil, errUnsupportedImage
	}
	return data, nil
}

var svgWarnOnce sync.Once

func rasterizeSVG(data []byte) ([]byte, error) {
	binary, err := exec.LookPath("rsvg-convert")
	if err != nil {
		svgWarnOnce.Do(func() {
			os.Stderr.WriteString("Deck plugins: SVG key images need rsvg-convert (librsvg2-bin)\n")
		})
		return nil, errUnsupportedImage
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-w", "144", "-h", "144", "-f", "png")
	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stdout = &limitedBuffer{buf: &out, limit: deckplugin.MaxImageBytes}
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type limitedBuffer struct {
	buf   *bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > b.limit {
		return 0, errors.New("output too large")
	}
	return b.buf.Write(p)
}

// openURL lets a plugin open a web page through the same validation as
// Deck website keys.
func openURL(raw string) {
	if nexus.ValidateDeckLaunchAction("url", raw) != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	exec.CommandContext(ctx, "xdg-open", raw).Run()
}

package deck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"os"
	"path/filepath"
	"regexp"

	xdraw "golang.org/x/image/draw"
)

const (
	MaxDeckIconBytes     = 512 * 1024
	maxDeckIconDimension = 512
	maxDeckIconFiles     = 256
	deckIconPixels       = 128
)

var deckIconID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Serialize icon-library writes so concurrent uploads cannot bypass the file cap.
var deckIconSaveMu sync.Mutex

// Icon IDs are derived from file contents. A key stores an ID rather than a
// browser-supplied path; all images stay under a dedicated private directory.
func deckIconDir(root string) string {
	return filepath.Join(root, "database", "deck", "icons")
}

func deckIconPath(root, id string) (string, error) {
	if !deckIconID.MatchString(id) {
		return "", errors.New("invalid Deck icon ID")
	}
	return filepath.Join(deckIconDir(root), id+".png"), nil
}

// SaveDeckIcon decodes bounded raster artwork and stores a normalized
// transparent 128x128 PNG. JPEG and PNG are accepted; animation is separate.
func SaveDeckIcon(root string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxDeckIconBytes {
		return "", errors.New("Deck icon must be 1 to 512 KiB")
	}
	info, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return "", errors.New("Deck icon must be PNG or JPEG")
	}
	if info.Width < 1 || info.Height < 1 ||
		info.Width > maxDeckIconDimension || info.Height > maxDeckIconDimension {
		return "", errors.New("Deck icon dimensions must be 1 to 512 pixels")
	}
	source, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return "", errors.New("invalid Deck icon image data")
	}
	digest := sha256.Sum256(data)
	id := hex.EncodeToString(digest[:16])
	path, err := deckIconPath(root, id)
	if err != nil {
		return "", err
	}
	// Guard the existence check, quota check and atomic rename as one operation.
	deckIconSaveMu.Lock()
	defer deckIconSaveMu.Unlock()
	if _, err := os.Stat(path); err == nil {
		return id, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	destination := image.NewRGBA(image.Rect(0, 0, deckIconPixels, deckIconPixels))
	scale := float64(deckIconPixels) / float64(info.Width)
	if candidate := float64(deckIconPixels) / float64(info.Height); candidate < scale {
		scale = candidate
	}
	width := int(float64(info.Width) * scale)
	height := int(float64(info.Height) * scale)
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	bounds := image.Rect((deckIconPixels-width)/2, (deckIconPixels-height)/2,
		(deckIconPixels-width)/2+width, (deckIconPixels-height)/2+height)
	xdraw.CatmullRom.Scale(destination, bounds, source, source.Bounds(), draw.Over, nil)

	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, destination); err != nil {
		return "", err
	}
	dir := deckIconDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) >= maxDeckIconFiles {
		return "", fmt.Errorf("Deck icon library is limited to %d images", maxDeckIconFiles)
	}
	temp, err := os.CreateTemp(dir, ".deck-icon-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return "", err
	}
	if _, err := temp.Write(imageBytes.Bytes()); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return "", err
	}
	return id, nil
}

func LoadDeckIcon(root, id string) (image.Image, error) {
	path, err := deckIconPath(root, id)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	if img.Bounds().Dx() != deckIconPixels || img.Bounds().Dy() != deckIconPixels {
		return nil, errors.New("unexpected Deck icon dimensions")
	}
	return img, nil
}

// ReadDeckIcon returns the bounded normalized file for the authenticated
// same-origin preview endpoint without exposing arbitrary paths.
func ReadDeckIcon(root, id string) ([]byte, error) {
	path, err := deckIconPath(root, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > 128*1024 {
		return nil, errors.New("invalid normalized icon")
	}
	return data, nil
}

func (d *Device) SaveIcon(data []byte) (string, error) {
	return SaveDeckIcon(d.root, data)
}

func (d *Device) ReadIcon(id string) ([]byte, error) {
	return ReadDeckIcon(d.root, id)
}

// Background with letterboxing, then the uploaded image and optional label.
func renderDeckArtwork(size int, icon image.Image, label string) image.Image {
	canvas := renderKeyLabel(size, "")
	if icon != nil {
		xdraw.CatmullRom.Scale(canvas.(*image.RGBA), canvas.Bounds(), icon, icon.Bounds(), draw.Over, nil)
	}
	if label == "" {
		return canvas
	}
	// Keep text legible against a photograph without treating it as markup.
	return drawDeckKeyCaption(canvas.(*image.RGBA), label)
}

func drawDeckKeyCaption(img *image.RGBA, label string) image.Image {
	size := img.Bounds().Dx()
	text := []rune(strings.TrimSpace(label))
	for len(text) > 0 && font.MeasureString(basicfont.Face7x13, string(text)).Ceil() > size-8 {
		text = text[:len(text)-1]
	}
	if len(text) == 0 {
		return img
	}
	band := image.Rect(0, size-22, size, size)
	draw.DrawMask(img, band, image.NewUniform(color.Black), image.Point{},
		image.NewUniform(color.Alpha{A: 180}), image.Point{}, draw.Over)
	width := font.MeasureString(basicfont.Face7x13, string(text)).Ceil()
	drawer := font.Drawer{
		Dst: img, Src: image.NewUniform(color.White), Face: basicfont.Face7x13,
		Dot: fixed.P((size-width)/2, size-6),
	}
	drawer.DrawString(string(text))
	return img
}

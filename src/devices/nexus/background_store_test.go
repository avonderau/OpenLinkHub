package nexus

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestSaveAndReadBackground(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 640, 48))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveBackground(dir, "sample", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadBackground(dir, "sample")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Bounds().Dx() != 640 {
		t.Fatal("unexpected dimensions")
	}
	if _, err := SaveBackground(dir, "../escape", buf.Bytes()); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := os.Stat(dir + "/sample.png"); err != nil {
		t.Fatal(err)
	}
}

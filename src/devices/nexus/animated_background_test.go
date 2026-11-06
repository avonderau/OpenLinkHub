package nexus

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func animatedFixture(t *testing.T) []byte {
	t.Helper()
	palette := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 40, 40, 255}, color.RGBA{20, 200, 80, 255}}
	first := image.NewPaletted(image.Rect(0, 0, 64, 24), palette)
	second := image.NewPaletted(image.Rect(0, 0, 64, 24), palette)
	for y := 0; y < 24; y++ {
		for x := 0; x < 64; x++ {
			first.SetColorIndex(x, y, 1)
			second.SetColorIndex(x, y, 2)
		}
	}
	var buf bytes.Buffer
	animation := &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{10, 10}, Disposal: []byte{gif.DisposalNone, gif.DisposalNone}, Config: image.Config{Width: 64, Height: 24, ColorModel: palette}}
	if err := gif.EncodeAll(&buf, animation); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAnimatedGIFUploadAndPlayback(t *testing.T) {
	data := animatedFixture(t)
	dir := t.TempDir()
	name, err := SaveBackground(dir, "demo", data)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(name) != ".gif" {
		t.Fatalf("expected .gif, got %q", name)
	}
	if !IsAnimatedBackground(dir, "demo") {
		t.Fatal("gif should be animated")
	}
	frame0, err := ReadBackgroundAt(dir, "demo", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	frame1, err := ReadBackgroundAt(dir, "demo", time.Unix(0, int64(120*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	r0, g0, _, _ := frame0.At(320, 24).RGBA()
	r1, g1, _, _ := frame1.At(320, 24).RGBA()
	if r0 <= g0 || r1 >= g1 {
		t.Fatalf("GIF did not advance frames: %d/%d then %d/%d", r0, g0, r1, g1)
	}
	looped, err := ReadBackgroundAt(dir, "demo", time.Unix(0, int64(220*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if frame0.At(320, 24) != looped.At(320, 24) {
		t.Fatal("GIF did not loop")
	}
	// The returned GIF frame is a fully usable image for the real Nexus renderer.
	screen := Screen{ID: "gif", Name: "Animated", Background: "demo"}
	if _, err := RenderScreenWithBackground(screen, nil, frame1); err != nil {
		t.Fatal(err)
	}
}

func TestAnimatedGIFRejectsInvalidOrExcessiveData(t *testing.T) {
	valid := animatedFixture(t)
	cases := map[string][]byte{"empty": {}, "truncated": valid[:len(valid)/2], "oversize": make([]byte, maxBackgroundBytes+1), "bad signature": []byte("GIF89abad")}
	for name, data := range cases {
		if _, err := SaveBackground(t.TempDir(), name, data); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	_, _, count, err := inspectGIF(valid)
	if err != nil || count != 2 {
		t.Fatalf("frame count: %d, %v", count, err)
	}
	// A stream with too many descriptors is rejected before expanding the frames.
	source, err := gif.DecodeAll(bytes.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	over := &gif.GIF{Config: source.Config}
	for i := 0; i < maxAnimationFrames+1; i++ {
		over.Image = append(over.Image, source.Image[0])
		over.Delay = append(over.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, over); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveBackground(t.TempDir(), "too_many", buf.Bytes()); err == nil || !strings.Contains(err.Error(), "frame") {
		t.Fatalf("frame limit not enforced: %v", err)
	}
}

func TestBackgroundFormatReplacement(t *testing.T) {
	dir := t.TempDir()
	if _, err := SaveBackground(dir, "shared", animatedFixture(t)); err != nil {
		t.Fatal(err)
	}
	pngImage := image.NewRGBA(image.Rect(0, 0, 64, 24))
	pngImage.Set(0, 0, color.RGBA{10, 20, 30, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, pngImage); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveBackground(dir, "shared", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "shared.gif")); !os.IsNotExist(err) {
		t.Fatalf("old GIF not removed: %v", err)
	}
	if IsAnimatedBackground(dir, "shared") {
		t.Fatal("PNG replacement still flagged as animated")
	}
	if _, err := ReadBackgroundAt(dir, "shared", time.Now()); err != nil {
		t.Fatal(err)
	}
}

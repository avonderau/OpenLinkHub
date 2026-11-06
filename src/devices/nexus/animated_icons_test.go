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

func animatedIconFixture(t *testing.T, frames int) []byte {
	t.Helper()
	palette := color.Palette{
		color.RGBA{0, 0, 0, 0},
		color.RGBA{235, 32, 32, 255},
		color.RGBA{32, 225, 72, 255},
	}
	animation := &gif.GIF{Config: image.Config{Width: 32, Height: 16, ColorModel: palette}}
	for i := 0; i < frames; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 32, 16), palette)
		selected := uint8(1)
		if i%2 == 1 {
			selected = 2
		}
		for y := 0; y < 16; y++ {
			for x := 0; x < 32; x++ {
				frame.SetColorIndex(x, y, selected)
			}
		}
		animation.Image = append(animation.Image, frame)
		animation.Delay = append(animation.Delay, 10)
		animation.Disposal = append(animation.Disposal, gif.DisposalNone)
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, animation); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestAnimatedIconUploadAndPlayback(t *testing.T) {
	dir := t.TempDir()
	path, err := SaveCustomIcon(dir, "animated", animatedIconFixture(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".gif" {
		t.Fatalf("wrong stored icon format %q", path)
	}
	if !IsAnimatedCustomIcon(dir, "animated") {
		t.Fatal("GIF icon not recognized as animated")
	}
	first, err := ReadCustomIconAt(dir, "animated", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadCustomIconAt(dir, "animated", time.Unix(0, int64(120*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if first.Bounds() != image.Rect(0, 0, 32, 32) || second.Bounds() != first.Bounds() {
		t.Fatal("icon was not normalized to 32x32")
	}
	r1, g1, _, _ := first.At(16, 16).RGBA()
	r2, g2, _, _ := second.At(16, 16).RGBA()
	if r1 <= g1 || r2 >= g2 {
		t.Fatalf("GIF icon failed to animate %d/%d then %d/%d", r1, g1, r2, g2)
	}
	_, _, _, alpha := first.At(16, 0).RGBA()
	if alpha != 0 {
		t.Fatal("letterbox transparency was not preserved")
	}
	looped, err := ReadCustomIconAt(dir, "animated", time.Unix(0, int64(220*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if looped.At(16, 16) != first.At(16, 16) {
		t.Fatal("GIF failed to loop")
	}
}

func TestAnimatedIconRejectsTooManyFramesAndMalformedData(t *testing.T) {
	if _, err := decodeAnimatedCustomIcon(animatedIconFixture(t, maxAnimatedIconFrames+1)); err == nil {
		t.Fatal("too many icon frames accepted")
	}
	if _, err := SaveCustomIcon(t.TempDir(), "broken", []byte("GIF89a")); err == nil {
		t.Fatal("malformed GIF accepted")
	}
	if _, err := SaveCustomIcon(t.TempDir(), "oversized", make([]byte, maxCustomIconBytes+1)); err == nil {
		t.Fatal("oversize upload accepted")
	}
	raw := animatedIconFixture(t, 1)
	index := bytes.IndexByte(raw, 0x2c)
	if index < 0 || index+9 >= len(raw) {
		t.Fatal("fixture missing image descriptor")
	}
	mutated := append([]byte(nil), raw...)
	// Simulate a frame wider than the logical canvas; reject before gif.DecodeAll.
	mutated[index+5], mutated[index+6] = 0xff, 0xff
	if _, _, _, err := inspectGIF(mutated); err == nil || !strings.Contains(err.Error(), "canvas") {
		t.Fatalf("unsafe GIF frame accepted: %v", err)
	}
}

func TestAnimatedIconCanBeReplacedByStaticPNG(t *testing.T) {
	dir := t.TempDir()
	if _, err := SaveCustomIcon(dir, "changing", animatedIconFixture(t, 2)); err != nil {
		t.Fatal(err)
	}
	static := image.NewRGBA(image.Rect(0, 0, 48, 48))
	static.SetRGBA(24, 24, color.RGBA{88, 140, 200, 255})
	var data bytes.Buffer
	if err := png.Encode(&data, static); err != nil {
		t.Fatal(err)
	}
	path, err := SaveCustomIcon(dir, "changing", data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("not a PNG: %q", path)
	}
	if _, err := os.Stat(filepath.Join(dir, "changing.gif")); !os.IsNotExist(err) {
		t.Fatalf("stale GIF remained: %v", err)
	}
	if IsAnimatedCustomIcon(dir, "changing") {
		t.Fatal("static replacement still marked animated")
	}
	icon, err := ReadCustomIconAt(dir, "changing", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if icon.Bounds() != image.Rect(0, 0, 32, 32) {
		t.Fatal("static icon dimensions changed")
	}
}

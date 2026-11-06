package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func capacityPackFixture(t *testing.T, count int) []byte {
	t.Helper()
	manifest, index := streamDeckFixture()
	index = nil
	frame := image.NewRGBA(image.Rect(0, 0, 16, 16))
	frame.Set(4, 4, color.RGBA{R: 255, A: 255})
	var picture bytes.Buffer
	if err := png.Encode(&picture, frame); err != nil {
		t.Fatal(err)
	}
	images := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("item-%03d.png", i)
		index = append(index, map[string]any{
			"name": fmt.Sprintf("Icon %03d", i),
			"path": filename,
			"tags": []string{"capacity"},
		})
		images["icons/"+filename] = picture.Bytes()
	}
	return makeStreamDeckZIP(t, manifest, index, images)
}

func TestStreamDeckCapacity256Icons(t *testing.T) {
	root := t.TempDir()
	pack, err := importIconSet(root, "", capacityPackFixture(t, 256))
	if err != nil {
		t.Fatalf("256-icon archive rejected: %v", err)
	}
	if len(pack.Icons) != 256 {
		t.Fatalf("expected 256 imported icons, got %d", len(pack.Icons))
	}
	saved, err := readIconSetManifest(root, pack.ID)
	if err != nil || len(saved.Icons) != 256 {
		t.Fatalf("256-icon manifest not readable: %v, %d icons", err, len(saved.Icons))
	}
}

func TestStreamDeckCapacityRejects4097Icons(t *testing.T) {
	_, err := importIconSet(t.TempDir(), "", capacityPackFixture(t, 4097))
	if err == nil {
		t.Fatal("4097-icon archive was accepted")
	}
}

func TestStreamDeckArchiveEntryCap(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for i := 0; i <= maxIconSetArchiveEntries; i++ {
		f, err := writer.Create(fmt.Sprintf("icons/%04d.png", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := importIconSet(t.TempDir(), "Too many", buffer.Bytes()); err == nil {
		t.Fatal("oversized archive entry list accepted")
	}
}

func TestIconPackZIPBudget(t *testing.T) {
	if maxIconSetImages != 4096 || maxIconSetZipBytes != 32<<20 ||
		maxIconSetTotalImageBytes != 64<<20 || maxIconSetArchiveEntries != 8192 {
		t.Fatal("unexpected icon pack capacity limits")
	}
}

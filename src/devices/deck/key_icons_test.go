package deck

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleDeckIcon(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 224, G: 20, B: 38, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSaveDeckIconAndPreview(t *testing.T) {
	root := t.TempDir()
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			data := sampleDeckIcon(t, format)
			id, err := SaveDeckIcon(root, data)
			if err != nil {
				t.Fatal(err)
			}
			if !deckIconID.MatchString(id) {
				t.Fatalf("invalid content-addressed ID: %q", id)
			}
			duplicate, err := SaveDeckIcon(root, data)
			if err != nil || duplicate != id {
				t.Fatalf("deduplication failed: %q %v", duplicate, err)
			}
			raw, err := ReadDeckIcon(root, id)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(raw, []byte{0x89, 'P', 'N', 'G'}) {
				t.Fatal("normalized image is not PNG")
			}
			img, err := LoadDeckIcon(root, id)
			if err != nil || img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
				t.Fatalf("unexpected image dimensions: %v %v", img, err)
			}
			if filepath.Base(deckIconDir(root)) != "icons" {
				t.Fatal("artwork escaped private directory")
			}
			out := renderDeckArtwork(85, img, "CPU")
			if out.Bounds().Dx() != 85 || out.Bounds().Dy() != 85 {
				t.Fatal("rendering changed the required physical key size")
			}
		})
	}
}

func TestDeckIconValidationAndFilesystemIsolation(t *testing.T) {
	root := t.TempDir()
	for _, invalid := range [][]byte{nil, {}, []byte("<svg onload=alert(1)>"),
		bytes.Repeat([]byte("a"), MaxDeckIconBytes+1)} {
		if _, err := SaveDeckIcon(root, invalid); err == nil {
			t.Fatalf("invalid content accepted: %d bytes", len(invalid))
		}
	}
	for _, id := range []string{"../../etc/passwd", "", strings.Repeat("a", 33), "ABCDEF"} {
		if _, err := ReadDeckIcon(root, id); err == nil {
			t.Fatalf("unsafe icon path accepted: %q", id)
		}
	}
	id := strings.Repeat("a", 32)
	if _, err := ReadDeckIcon(root, id); !os.IsNotExist(err) {
		t.Fatalf("missing icon should fail: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 513, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveDeckIcon(root, buf.Bytes()); err == nil {
		t.Fatal("oversized image dimensions accepted")
	}
}

// The 256-image limit must hold even when separate HTTP requests upload
// different images concurrently. Existing (deduplicated) icons still work.
func TestDeckIconConcurrentUploadsRespectLibraryLimit(t *testing.T) {
	root := t.TempDir()
	dir := deckIconDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxDeckIconFiles-1; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%032x.png", i))
		if err := os.WriteFile(name, []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	uploads := [][]byte{sampleDeckIcon(t, "png"), sampleDeckIcon(t, "jpeg")}
	start := make(chan struct{})
	results := make(chan error, len(uploads))
	for _, data := range uploads {
		data := data
		go func() {
			<-start
			_, err := SaveDeckIcon(root, data)
			results <- err
		}()
	}
	close(start)

	succeeded := 0
	for range uploads {
		if err := <-results; err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected exactly one upload within remaining quota; got %d", succeeded)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxDeckIconFiles {
		t.Fatalf("concurrent upload bypassed library limit: %d files", len(entries))
	}
	// A repeated image is a no-op and should not consume another slot.
	for _, data := range uploads {
		sha := sha256.Sum256(data)
		id := fmt.Sprintf("%x", sha[:16])
		if _, err := os.Stat(filepath.Join(dir, id+".png")); err == nil {
			if _, err := SaveDeckIcon(root, data); err != nil {
				t.Fatalf("deduplicated upload rejected at capacity: %v", err)
			}
		}
	}
}

func TestDeckIconPersistsInLayoutAndRendersAfterReconnect(t *testing.T) {
	d := testDevice(t)
	id, err := d.SaveIcon(sampleDeckIcon(t, "png"))
	if err != nil {
		t.Fatal(err)
	}
	layout := DefaultLayout(d.Model)
	layout.Pages[0].Keys = []KeyConfig{{Index: 3, Icon: id, Label: "Open"}}
	result, err := d.UpdateLayout(layout)
	if err != nil || !result.Saved {
		t.Fatalf("image assignment failed: %+v %v", result, err)
	}
	restored, err := LoadLayout(d.root, d.Serial, d.Model)
	if err != nil || restored.Pages[0].Keys[0].Icon != id {
		t.Fatalf("artwork reference did not survive reload: %+v %v", restored, err)
	}
	h := new(fakeDeckSession)
	if err := d.attachSession(h); err != nil {
		t.Fatal(err)
	}
	if countMiraboxOperation(h.reports, "BAT") != d.Model.Keys() {
		t.Fatal("artwork device page not written to Mirabox")
	}
	if countMiraboxOperation(h.reports, "STP") != 1 {
		t.Fatal("artwork image transfer must use one flush")
	}
	d.Stop()

	layout.Pages[0].Keys[0].Icon = "../invalid"
	if err := layout.Validate(d.Model); err == nil {
		t.Fatal("unsafe artwork ID permitted in saved layout")
	}
}

func TestDeckArtworkCaptionReservesSpaceForIcon(t *testing.T) {
	icon := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			icon.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	labeled := renderDeckArtwork(85, icon, "Open")
	if red := color.RGBAModel.Convert(labeled.At(42, 30)).(color.RGBA); red.R != 255 || red.G != 0 {
		t.Fatalf("artwork missing from upper icon area: %#v", red)
	}
	if bottom := color.RGBAModel.Convert(labeled.At(42, 67)).(color.RGBA); bottom.R != 0 || bottom.G != 0 || bottom.B != 0 {
		t.Fatalf("artwork overlaps reserved caption band: %#v", bottom)
	}
	if margin := color.RGBAModel.Convert(labeled.At(2, 30)).(color.RGBA); margin.R == 255 {
		t.Fatalf("icon should be proportionally reduced, not stretched across whole width: %#v", margin)
	}
	withoutCaption := renderDeckArtwork(85, icon, "")
	if bottom := color.RGBAModel.Convert(withoutCaption.At(42, 67)).(color.RGBA); bottom.R != 255 {
		t.Fatalf("unlabeled artwork should continue to use full LCD: %#v", bottom)
	}
}

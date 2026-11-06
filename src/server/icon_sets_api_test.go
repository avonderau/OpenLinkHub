package server

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func makeIconSetZIP(t *testing.T, name string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	file, err := z.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, 12, 12))
	frame.SetRGBA(5, 5, color.RGBA{R: 255, A: 255})
	if err = png.Encode(file, frame); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestSharedIconSetImportAndPersistence(t *testing.T) {
	root := t.TempDir()
	imported, err := importIconSet(root, "Desktop Icons", makeIconSetZIP(t, "media/rewind.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Icons) != 1 || imported.Icons[0].Name != "rewind" || len(imported.Icons[0].DeckID) != 32 || imported.Icons[0].NexusID == "" {
		t.Fatalf("bad imported pack: %+v", imported)
	}
	all, err := listIconSets(root)
	if err != nil || len(all) != 1 || all[0].ID != imported.ID {
		t.Fatalf("pack listing not persistent: %+v %v", all, err)
	}
	repeated, err := importIconSet(root, "Another Title", makeIconSetZIP(t, "media/rewind.png"))
	if err != nil || repeated.ID != imported.ID {
		t.Fatalf("same archive imported twice: %+v %v", repeated, err)
	}
}
func TestSharedIconSetRejectsUnsafeArchive(t *testing.T) {
	for _, tc := range []struct {
		Name string
		Zip  string
	}{
		{"traversal", "../escape.png"},
		{"absolute", "/bad.png"},
		{"non-raster", "test.svg"},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			if _, err := importIconSet(t.TempDir(), "Safe", makeIconSetZIP(t, tc.Zip)); err == nil {
				t.Fatal("unsafe pack accepted")
			}
		})
	}
	// An image-only ZIP with no manifest now gets a generated name instead
	// of being rejected for a missing user-supplied title.
}

func TestSharedIconSetRenameAndPreviewMetadata(t *testing.T) {
	root := t.TempDir()
	pack, err := importIconSet(root, "Old name", makeIconSetZIP(t, "media/rewind.png"))
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := renameIconSet(root, pack.ID, "New name")
	if err != nil || renamed.Name != "New name" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	all, err := listIconSets(root)
	if err != nil || len(all) != 1 || all[0].Name != "New name" || all[0].Icons[0].DeckID != pack.Icons[0].DeckID {
		t.Fatalf("rename changed stored artwork: %+v %v", all, err)
	}
	if _, err := renameIconSet(root, pack.ID, "\n"); err == nil {
		t.Fatal("empty set name accepted")
	}
}

func TestLegacyZIPAutomaticNaming(t *testing.T) {
	root := t.TempDir()
	first, err := importIconSet(root, "", makeIconSetZIP(t, "one.png"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "NewPack1" {
		t.Fatalf("expected NewPack1, got %q", first.Name)
	}
	second, err := importIconSet(root, "", makeIconSetZIP(t, "two.png"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Name != "NewPack2" {
		t.Fatalf("expected NewPack2, got %q", second.Name)
	}
	again, err := importIconSet(root, "", makeIconSetZIP(t, "one.png"))
	if err != nil || again.ID != first.ID || again.Name != "NewPack1" {
		t.Fatalf("duplicate import must retain original name: %+v %v", again, err)
	}
	if _, err := renameIconSet(root, first.ID, "Custom Name"); err != nil {
		t.Fatal(err)
	}
	third, err := importIconSet(root, "", makeIconSetZIP(t, "three.png"))
	if err != nil || third.Name != "NewPack1" {
		t.Fatalf("available default name should be reusable: %+v %v", third, err)
	}
}

func TestLegacyZIPNamingDoesNotOverrideEmbeddedManifest(t *testing.T) {
	manifest, icons := streamDeckFixture()
	packs, err := importIconSetBundle(t.TempDir(), "", makeStreamDeckZIP(t, manifest, icons, nil))
	if err != nil || len(packs) != 1 {
		t.Fatalf("Stream Deck ZIP import: %+v %v", packs, err)
	}
	if packs[0].Name != "Sample Pack" {
		t.Fatalf("embedded manifest name ignored: %q", packs[0].Name)
	}
}

func TestLegacyZIPAutomaticNamesSkipExistingNumbers(t *testing.T) {
	root := t.TempDir()
	first, err := importIconSet(root, "", makeIconSetZIP(t, "one.png"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renameIconSet(root, first.ID, "newpack2"); err != nil {
		t.Fatal(err)
	}
	next, err := importIconSet(root, "", makeIconSetZIP(t, "two.png"))
	if err != nil || next.Name != "NewPack1" {
		t.Fatalf("should use lowest unused number: %+v %v", next, err)
	}
	third, err := importIconSet(root, "", makeIconSetZIP(t, "three.png"))
	if err != nil || third.Name != "NewPack3" {
		t.Fatalf("name collision with existing NewPack2: %+v %v", third, err)
	}
}

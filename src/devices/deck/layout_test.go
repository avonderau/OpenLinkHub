package deck

import (
	"os"
	"testing"
)

func TestDeckLayoutRoundTripAndValidation(t *testing.T) {
	m, _ := LookupModel(0x5548, 0x6670)
	root := t.TempDir()
	id := "../device/serial"
	l := DefaultLayout(m)
	l.Pages[0].Keys = []KeyConfig{{Index: 0, Label: "CPU"}, {Index: 17, Label: "Last"}}
	if err := SaveLayout(root, id, m, l); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLayout(root, id, m)
	if err != nil || len(got.Pages) != 1 || len(got.Pages[0].Keys) != 2 {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if _, err := os.Stat(layoutFilename(root, id)); err != nil {
		t.Fatal(err)
	}
	if layoutFilename(root, id) == layoutFilename(root, "device/serial") {
		t.Fatal("IDs collide")
	}
	invalid := l
	invalid.Pages = append([]Page(nil), l.Pages...)
	invalid.Pages[0].Keys = []KeyConfig{{Index: 0}, {Index: 0}}
	if err := SaveLayout(root, id, m, invalid); err == nil {
		t.Fatal("duplicate key accepted")
	}
	invalid.Pages[0].Keys = []KeyConfig{{Index: 18}}
	if err := invalid.Validate(m); err == nil {
		t.Fatal("out of range key accepted")
	}
	invalid.Pages[0].Keys = []KeyConfig{{Index: 1, Label: "line\nbreak"}}
	if err := invalid.Validate(m); err == nil {
		t.Fatal("control character accepted")
	}
	invalid = l
	invalid.ActivePage = "missing"
	if err := invalid.Validate(m); err == nil {
		t.Fatal("missing page accepted")
	}
}

package deck

import (
	"strings"
	"testing"
)

func testThreePageDeck(t *testing.T) *Device {
	t.Helper()
	d := testDevice(t)
	d.layout = Layout{
		Version: 1, ActivePage: "home",
		Pages: []Page{
			{ID: "home", Name: "Home", Keys: []KeyConfig{
				{Index: 0, Action: "page-next"},
				{Index: 1, Action: "page-go", TargetPage: "third", Label: "Jump"},
			}},
			{ID: "second", Name: "Second", Keys: []KeyConfig{
				{Index: 0, Action: "page-previous"},
				{Index: 2, Action: "page-next"},
			}},
			{ID: "third", Name: "Third", Keys: []KeyConfig{
				{Index: 0, Action: "page-previous"},
				{Index: 1, Action: "page-next"},
			}},
		},
	}
	if err := d.layout.Validate(d.Model); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestMiraboxPhysicalKeyPageNavigation(t *testing.T) {
	d := testThreePageDeck(t)
	handle := new(fakeDeckSession)
	if err := d.attachSession(handle); err != nil {
		t.Fatal(err)
	}
	if got := d.GetStatus().ActivePage; got != "home" {
		t.Fatalf("initial page: %q", got)
	}
	initialFlushes := countMiraboxOperation(handle.reports, "STP")
	d.handleKeyPress(0)
	if got := d.GetStatus().ActivePage; got != "second" {
		t.Fatalf("next action switched to %q", got)
	}
	if countMiraboxOperation(handle.reports, "STP") != initialFlushes+1 {
		t.Fatal("navigation did not update physical LCD")
	}
	persisted, err := LoadLayout(d.root, d.Serial, d.Model)
	if err != nil || persisted.ActivePage != "second" {
		t.Fatalf("page not persisted: %+v %v", persisted, err)
	}

	d.handleKeyPress(0)
	if got := d.GetStatus().ActivePage; got != "home" {
		t.Fatalf("previous action switched to %q", got)
	}
	d.handleKeyPress(1)
	if got := d.GetStatus().ActivePage; got != "third" {
		t.Fatalf("go-to-page action switched to %q", got)
	}
	d.handleKeyPress(1)
	if got := d.GetStatus().ActivePage; got != "home" {
		t.Fatalf("last-page wrap switched to %q", got)
	}

	status := d.GetStatus()
	if status.ButtonEvents != 4 || status.LastPressedKey != 1 {
		t.Fatalf("incorrect key event diagnostics: %+v", status)
	}
	persisted, err = LoadLayout(d.root, d.Serial, d.Model)
	if err != nil || persisted.ActivePage != "home" {
		t.Fatalf("final active page not persisted: %+v %v", persisted, err)
	}
	d.Stop()
}

func TestUnassignedButtonDoesNotChangePage(t *testing.T) {
	d := testThreePageDeck(t)
	handle := new(fakeDeckSession)
	if err := d.attachSession(handle); err != nil {
		t.Fatal(err)
	}
	before := countMiraboxOperation(handle.reports, "STP")
	d.handleKeyPress(16) // Physical button 17; LCD position 18 is display-only.
	if d.GetStatus().ActivePage != "home" {
		t.Fatal("unassigned key changed page")
	}
	if countMiraboxOperation(handle.reports, "STP") != before {
		t.Fatal("unassigned key refreshed LCD")
	}
	if d.GetStatus().ButtonEvents != 1 {
		t.Fatal("button event not tracked")
	}
	d.Stop()
}

func TestPageNavigationLayoutValidation(t *testing.T) {
	d := testThreePageDeck(t)
	valid := cloneLayout(d.layout)
	if err := valid.Validate(d.Model); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, action, target string
	}{
		{"unrecognized", "run-command", ""},
		{"missing target", "page-go", ""},
		{"unknown target", "page-go", "does-not-exist"},
		{"non-page target", "page-next", "third"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := cloneLayout(valid)
			l.Pages[0].Keys[0].Action = tc.action
			l.Pages[0].Keys[0].TargetPage = tc.target
			if err := l.Validate(d.Model); err == nil {
				t.Fatal("invalid action accepted")
			}
		})
	}
	if keyDisplayLabel(KeyConfig{Action: "page-next"}) != "Next Page" {
		t.Fatal("missing default action label")
	}
	if keyDisplayLabel(KeyConfig{Label: "Custom", Action: "page-previous"}) != "Custom" {
		t.Fatal("custom label was overwritten")
	}
}

func TestPageNavigationOutputFailureKeepsSelectedPage(t *testing.T) {
	d := testThreePageDeck(t)
	handle := new(fakeDeckSession)
	if err := d.attachSession(handle); err != nil {
		t.Fatal(err)
	}
	handle.failOutput = true
	d.handleKeyPress(0)
	status := d.GetStatus()
	if status.ActivePage != "second" || !strings.Contains(status.Status, "LCD refresh failed") {
		t.Fatalf("navigation failure status: %+v", status)
	}
	persisted, err := LoadLayout(d.root, d.Serial, d.Model)
	if err != nil || persisted.ActivePage != "second" {
		t.Fatalf("selected page not persisted: %+v %v", persisted, err)
	}
	d.Stop()
}

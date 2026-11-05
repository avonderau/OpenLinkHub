package deck

import (
	"strings"
	"testing"
	"time"
)

func TestDeckLaunchAndMacroTargetsAreValidated(t *testing.T) {
	d := testDevice(t)
	layout := DefaultLayout(d.Model)
	layout.Pages[0].Keys = []KeyConfig{
		{Index: 0, Action: "url", Target: "https://example.com/path"},
		{Index: 1, Action: "application", Target: "org.gnome.Terminal.desktop"},
		{Index: 2, Action: "macro", Target: "42"},
	}
	if err := layout.Validate(d.Model); err != nil {
		t.Fatal(err)
	}
	if err := SaveLayout(d.root, d.Serial, d.Model, layout); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLayout(d.root, d.Serial, d.Model)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Pages[0].Keys) != 3 || loaded.Pages[0].Keys[2].Target != "42" {
		t.Fatalf("new action targets did not round-trip: %+v", loaded.Pages[0].Keys)
	}

	cases := []KeyConfig{
		{Action: "url", Target: "file:///etc/shadow"},
		{Action: "url", Target: "javascript:alert(1)"},
		{Action: "url", Target: "https://user:password@example.com/"},
		{Action: "application", Target: "/bin/sh"},
		{Action: "application", Target: "terminal.desktop;touch /tmp/test"},
		{Action: "macro", Target: "../../unsafe"},
		{Action: "macro", Target: "0001"},
		{Action: "macro", Target: "0"},
		{Action: "url", Target: "https://example.com", TargetPage: "home"},
		{Action: "macro", Target: "3", TargetPage: "home"},
	}
	for i, key := range cases {
		key.Index = 0
		invalid := DefaultLayout(d.Model)
		invalid.Pages[0].Keys = []KeyConfig{key}
		if err := invalid.Validate(d.Model); err == nil {
			t.Errorf("case %d: invalid key allowed: %+v", i, key)
		}
	}
}

func TestDeckLaunchDispatchDoesNotHoldHIDMutex(t *testing.T) {
	d := testDevice(t)
	d.layout.Pages[0].Keys = []KeyConfig{
		{Index: 0, Action: "url", Target: "https://example.com/hello"},
		{Index: 1, Action: "application", Target: "firefox.desktop"},
	}
	var actions []string
	d.inputActionRunner = func(action, target string) error {
		_ = d.GetStatus()
		actions = append(actions, action+":"+target)
		return nil
	}
	d.handleKeyPress(0)
	d.handleKeyPress(1)
	if len(actions) != 2 || actions[0] != "url:https://example.com/hello" || actions[1] != "application:firefox.desktop" {
		t.Fatalf("launch dispatch: %+v", actions)
	}
	if d.GetStatus().ButtonEvents != 2 {
		t.Fatalf("button counter: %+v", d.GetStatus())
	}
}

func TestDeckMacroRunIsNonblockingAndSingleFlight(t *testing.T) {
	d := testDevice(t)
	d.layout.Pages[0].Keys = []KeyConfig{{Index: 0, Action: "macro", Target: "42"}}
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	finished := make(chan struct{})
	d.inputActionRunner = func(action, target string) error {
		if action != "macro" || target != "42" {
			t.Errorf("invalid macro dispatch %q %q", action, target)
		}
		_ = d.GetStatus()
		entered <- struct{}{}
		<-release
		close(finished)
		return nil
	}
	d.handleKeyPress(0)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("macro was not dispatched")
	}
	d.handleKeyPress(0)
	select {
	case <-entered:
		t.Fatal("macro executed twice concurrently")
	default:
	}
	d.mu.RLock()
	running := d.macroRunning
	d.mu.RUnlock()
	if !running {
		t.Fatal("macro not marked active")
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("macro did not finish")
	}
	deadline := time.After(2 * time.Second)
	for {
		d.mu.RLock()
		busy := d.macroRunning
		d.mu.RUnlock()
		if !busy {
			break
		}
		select {
		case <-deadline:
			t.Fatal("macro slot not released")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	d.Stop()
}

func TestMalformedLaunchMacroEventCannotRun(t *testing.T) {
	d := testDevice(t)
	called := false
	d.inputActionRunner = func(string, string) error {
		called = true
		return nil
	}
	for _, key := range []KeyConfig{
		{Index: 0, Action: "url", Target: "file:///etc/shadow"},
		{Index: 0, Action: "application", Target: "/bin/sh"},
		{Index: 0, Action: "macro", Target: "1;touch"},
	} {
		d.layout.Pages[0].Keys = []KeyConfig{key}
		d.handleKeyPress(0)
	}
	if called {
		t.Fatal("unvalidated action dispatched")
	}
	if got := d.GetStatus().ButtonEvents; got != 3 {
		t.Fatalf("event count %d", got)
	}
}

func TestDeckMacroMissingProfileReportsError(t *testing.T) {
	d := testDevice(t)
	d.layout.Pages[0].Keys = []KeyConfig{{Index: 0, Action: "macro", Target: "999999999"}}
	d.handleKeyPress(0)
	deadline := time.After(3 * time.Second)
	for {
		status := d.GetStatus()
		if strings.Contains(status.Status, "Saved macro failed:") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("missing macro was not reported: %q", status.Status)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	d.Stop()
}

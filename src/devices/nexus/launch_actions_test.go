package nexus

import (
	"strings"
	"testing"
)

func TestNexusWebURLValidation(t *testing.T) {
	valid := []string{
		"https://www.example.org/path?q=hi",
		"http://localhost:3000/status",
		"https://example.com/#anchor",
	}
	for _, u := range valid {
		if err := validateNexusURL(u); err != nil {
			t.Errorf("rejected valid URL %q: %v", u, err)
		}
	}
	invalid := []string{
		"", "https://", "mailto:test@example.com", "file:///etc/shadow",
		"javascript:alert(1)", "https://user:password@example.com/",
		"https://example.com/with space", "http://example.com\n--bad",
		"https://example.com/" + strings.Repeat("x", 256),
		"https://example.com\\evil",
	}
	for _, u := range invalid {
		if err := validateNexusURL(u); err == nil {
			t.Errorf("unsafe URL accepted: %q", u)
		}
	}
}

func TestNexusDesktopEntryValidation(t *testing.T) {
	for _, entry := range []string{"firefox.desktop", "org.gnome.Terminal.desktop", "code.desktop", "firefox"} {
		if err := validateNexusDesktopID(entry); err != nil {
			t.Errorf("rejected valid app ID %q: %v", entry, err)
		}
	}
	for _, entry := range []string{"", "..", "../../bin/sh", "/tmp/foo", "evil;echo pwned", "terminal.desktop --argument", "hidden..desktop", strings.Repeat("x", 129)} {
		if err := validateNexusDesktopID(entry); err == nil {
			t.Errorf("unsafe app ID accepted: %q", entry)
		}
	}
}

func TestNexusLauncherRequiresNonRootDesktopSession(t *testing.T) {
	if err := launchPermissionCheck(0, ":0", ""); err == nil {
		t.Fatal("root launcher allowed")
	}
	if err := launchPermissionCheck(1000, "", ""); err == nil {
		t.Fatal("headless launcher allowed")
	}
	if err := launchPermissionCheck(1000, ":1", ""); err != nil {
		t.Fatal(err)
	}
	if err := launchPermissionCheck(1000, "", "wayland-0"); err != nil {
		t.Fatal(err)
	}
}

func TestNexusLaunchActionsValidateOnSave(t *testing.T) {
	config := ScreenConfiguration{Version: 1, ActiveScreenID: "launcher", Screens: []Screen{{
		ID: "launcher", Name: "Launchers", Widgets: []Widget{
			{ID: "url", Type: "button", X: 0, Y: 0, Width: 160, Height: 48, Action: "url", Target: "https://example.com"},
			{ID: "application", Type: "button", X: 160, Y: 0, Width: 160, Height: 48, Action: "application", Target: "firefox.desktop"},
			{ID: "macro", Type: "button", Action: "macro", Target: "1", X: 320, Y: 0, Width: 160, Height: 48},
		},
	}}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	config.Screens[0].Widgets[0].Target = "file:///etc/passwd"
	if err := config.Validate(); err == nil {
		t.Fatal("unsafe URL button accepted")
	}
	config.Screens[0].Widgets[0].Target = "https://example.com"
	config.Screens[0].Widgets[2].Target = "../../evil.desktop"
	if err := config.Validate(); err == nil {
		t.Fatal("invalid saved macro ID accepted")
	}
}

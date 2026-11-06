package nexus

import (
	"os"
	"path/filepath"
	"testing"
)

// Every way of picking a screen on the touchscreen must survive a restart,
// not only swipes.
func TestTouchScreenSelectionIsSaved(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "database", "nexus"), 0o700); err != nil {
		t.Fatal(err)
	}
	oldPwd := pwd
	pwd = root
	t.Cleanup(func() { pwd = oldPwd })

	newDevice := func() *Device {
		return &Device{Serial: "test", customScreens: ScreenConfiguration{
			Version: 1, ActiveScreenID: "a", CyclingMode: "manual", CyclingSeconds: 15,
			Screens: []Screen{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}},
		}}
	}
	saved := func(t *testing.T, d *Device) string {
		t.Helper()
		c, err := LoadScreens(d.customScreenPath())
		if err != nil {
			t.Fatal(err)
		}
		return c.ActiveScreenID
	}

	for _, tc := range []struct {
		name, action, target, want string
	}{
		{"next button", "next", "", "b"},
		{"previous button", "previous", "", "c"},
		{"go to screen button", "screen", "c", "c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newDevice()
			d.executeTouchAction(tc.action, tc.target)
			if got := saved(t, d); got != tc.want {
				t.Fatalf("saved active screen = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("swipe", func(t *testing.T) {
		d := newDevice()
		d.navigateCustomScreen(true)
		if got := saved(t, d); got != "b" {
			t.Fatalf("saved active screen = %q, want %q", got, "b")
		}
	})
}

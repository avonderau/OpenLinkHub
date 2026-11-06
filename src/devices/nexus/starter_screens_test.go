package nexus

import (
	"path/filepath"
	"testing"
)

func TestStockScreensAreOrdinaryEditableLayouts(t *testing.T) {
	directory := filepath.Join("..", "..", "..", "static", "nexus", "templates")
	cfg, err := loadStockStarterScreens(directory)
	if err != nil {
		t.Fatalf("load stock screens: %v", err)
	}
	if len(cfg.Screens) != 5 {
		t.Fatalf("expected 5 stock screens, got %d", len(cfg.Screens))
	}
	if cfg.Screens[0].ID != cfg.ActiveScreenID {
		t.Fatal("active screen is not in the stock layout")
	}
	if cfg.CyclingMode != "manual" || cfg.CyclingSeconds != 15 {
		t.Fatalf("unexpected default cycling options: %+v", cfg)
	}
	hasAnimated := false
	for _, screen := range cfg.Screens {
		if len(screen.Widgets) == 0 {
			t.Fatalf("missing widgets in %s", screen.Name)
		}
		if screen.Background == "preset_pulse" {
			hasAnimated = true
		}
	}
	if !hasAnimated {
		t.Fatal("missing animated stock screen")
	}
	// Once selected, stock screens use the same JSON save/load path as any user screen.
	cfg.Screens[0].Name = "My Custom Layout"
	cfg.Screens[0].Widgets[0].Settings["label"] = "Edited"
	path := filepath.Join(t.TempDir(), "screens.json")
	if err := SaveScreens(path, cfg); err != nil {
		t.Fatalf("save edited screen: %v", err)
	}
	stored, err := LoadScreens(path)
	if err != nil {
		t.Fatalf("load edited screen: %v", err)
	}
	if stored.Screens[0].Name != "My Custom Layout" || stored.Screens[0].Widgets[0].Settings["label"] != "Edited" {
		t.Fatal("stock screen edits were not preserved")
	}
}

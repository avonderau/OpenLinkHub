package nexus

import (
	"path/filepath"
	"testing"
)

func sample() ScreenConfiguration {
	return ScreenConfiguration{Version: 1, ActiveScreenID: "system", Screens: []Screen{{ID: "system", Name: "System", Widgets: []Widget{{ID: "cpu", Type: "metric", Metric: "cpu.temperature", X: 0, Y: 0, Width: 120, Height: 48}}}, {ID: "network", Name: "Network"}}}
}
func TestValidation(t *testing.T) {
	c := sample()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Screens[0].Widgets[0].Width = 641
	if c.Validate() == nil {
		t.Fatal("out of bounds accepted")
	}
}
func TestNavigation(t *testing.T) {
	c := sample()
	if c.NextScreen() != "network" || c.PreviousScreen() != "network" {
		t.Fatal("navigation failed")
	}
}
func TestPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "screens.json")
	c := sample()
	if err := SaveScreens(p, c); err != nil {
		t.Fatal(err)
	}
	got, err := LoadScreens(p)
	if err != nil || got.ActiveScreenID != "system" {
		t.Fatalf("load: %v", err)
	}
}
func TestInvalidMetric(t *testing.T) {
	c := sample()
	c.Screens[0].Widgets[0].Metric = "shell.command"
	if c.Validate() == nil {
		t.Fatal("invalid metric accepted")
	}
}

func TestMacroButtonValidation(t *testing.T) {
	c := sample()
	c.Screens[0].Widgets = []Widget{{ID: "macro-button", Type: "button", Action: "macro", X: 0, Y: 0, Width: 96, Height: 48, Settings: map[string]string{"style": "outline", "backgroundColor": "#123456", "textColor": "#ffffff", "borderColor": "#ff8800"}, Target: "1"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid styled macro button rejected: %v", err)
	}
	c.Screens[0].Widgets[0].Settings["textColor"] = "red"
	if err := c.Validate(); err == nil {
		t.Fatal("invalid hex color accepted")
	}
	c.Screens[0].Widgets[0].Settings["textColor"] = "#ffffff"
	c.Screens[0].Widgets[0].Settings["style"] = "gradient"
	if err := c.Validate(); err == nil {
		t.Fatal("unsupported style accepted")
	}
	c.Screens[0].Widgets[0].Settings["style"] = "flat"
	c.Screens[0].Widgets[0].Target = "bad;echo"
	if err := c.Validate(); err == nil {
		t.Fatal("unsupported shared macro ID accepted")
	}
	c.Screens[0].Widgets[0].Target = "1"
	c.Screens[0].Widgets[0].Steps = []MacroStep{{Type: "shell", Value: "echo unsafe"}}
	if err := c.Validate(); err == nil {
		t.Fatal("legacy inline macros must be rejected")
	}
}

func TestColorValidationForMetricBarAndText(t *testing.T) {
	for _, kind := range []string{"metric", "bar", "text"} {
		t.Run(kind, func(t *testing.T) {
			c := sample()
			widget := Widget{ID: "styled", Type: kind, Metric: "cpu.temperature", X: 0, Y: 0, Width: 120, Height: 48, Settings: map[string]string{"backgroundColor": "#123456", "textColor": "#abcdef", "borderColor": "#ff8800"}}
			c.Screens[0].Widgets = []Widget{widget}
			if err := c.Validate(); err != nil {
				t.Fatalf("valid colors rejected: %v", err)
			}
			c.Screens[0].Widgets[0].Settings["borderColor"] = "red"
			if err := c.Validate(); err == nil {
				t.Fatal("invalid color accepted")
			}
		})
	}
}

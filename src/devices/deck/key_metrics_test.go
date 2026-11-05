package deck

import (
	"image"
	"math"
	"strings"
	"testing"
	"time"

	"OpenLinkHub/src/devices/nexus"
)

func TestDeckMetricLayoutValidation(t *testing.T) {
	d := testDevice(t)
	valid := DefaultLayout(d.Model)
	valid.Pages[0].Keys = []KeyConfig{{Index: 0, Metric: "cpu.temperature", RefreshSeconds: 5, ShowGraph: true}}
	if err := valid.Validate(d.Model); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []KeyConfig{
		{Index: 0, Metric: "invalid.metric", RefreshSeconds: 5},
		{Index: 0, Metric: "cpu.temperature", RefreshSeconds: 1},
		{Index: 0, Metric: "cpu.temperature", RefreshSeconds: 61},
		{Index: 0, RefreshSeconds: 5},
		{Index: 0, ShowGraph: true},
		{Index: 0, MetricStyle: "ring"},
		{Index: 0, Metric: "cpu.utilization", MetricStyle: "unexpected"},
	} {
		layout := DefaultLayout(d.Model)
		layout.Pages[0].Keys = []KeyConfig{bad}
		if err := layout.Validate(d.Model); err == nil {
			t.Errorf("invalid metric key accepted: %+v", bad)
		}
	}
}

func TestDeckMetricRefreshUpdatesOnlyDueKeys(t *testing.T) {
	d := testDevice(t)
	d.layout.Pages[0].Keys = []KeyConfig{
		{Index: 0, Label: "CPU", Metric: "cpu.temperature", RefreshSeconds: 5, ShowGraph: true},
		{Index: 1, Label: "Action", Action: "page-next"},
	}
	h := new(fakeDeckSession)
	if err := d.attachSession(h); err != nil {
		t.Fatal(err)
	}
	initial := countMiraboxOperation(h.reports, "BAT")
	if initial != d.Model.Keys() {
		t.Fatalf("expected full-page initial draw: %d", initial)
	}
	reads := 0
	d.metricReader = func() nexus.MetricSnapshot {
		reads++
		return nexus.MetricSnapshot{"cpu.temperature": float64(35 + reads)}
	}
	now := time.Unix(1700000000, 0)
	d.refreshMetricKeys(now)
	if reads != 1 || countMiraboxOperation(h.reports, "BAT") != initial+1 {
		t.Fatal("due metric should refresh exactly one LCD key")
	}
	if countMiraboxOperation(h.reports, "STP") != 2 {
		t.Fatal("metric refresh should flush the device once")
	}
	d.refreshMetricKeys(now.Add(2 * time.Second))
	if reads != 1 || countMiraboxOperation(h.reports, "BAT") != initial+1 {
		t.Fatal("metric refreshed before its interval elapsed")
	}
	d.refreshMetricKeys(now.Add(5 * time.Second))
	if reads != 2 || countMiraboxOperation(h.reports, "BAT") != initial+2 {
		t.Fatal("metric did not refresh at its configured interval")
	}
	status := d.GetStatus()
	if status.Metrics["cpu.temperature"] != 37 || status.MetricSampledAt != now.Add(5*time.Second).Unix() {
		t.Fatalf("stale status: %+v", status)
	}
	status.Metrics["cpu.temperature"] = 9000
	if d.GetStatus().Metrics["cpu.temperature"] != 37 {
		t.Fatal("status leaked internal metric map")
	}
	if len(d.metricHistory["cpu.temperature"]) != 2 {
		t.Fatal("metric sparkline did not accumulate samples")
	}
	d.Stop()
}

func TestDeckMetricAbsentValuesAndShortHistory(t *testing.T) {
	d := testDevice(t)
	d.layout.Pages[0].Keys = []KeyConfig{{Index: 2, Metric: "fan.rpm", ShowGraph: true, RefreshSeconds: 2}}
	h := new(fakeDeckSession)
	if err := d.attachSession(h); err != nil {
		t.Fatal(err)
	}
	d.metricReader = func() nexus.MetricSnapshot {
		return nexus.MetricSnapshot{"other.unlisted": 123, "fan.rpm": math.NaN()}
	}
	d.refreshMetricKeys(time.Unix(1700000000, 0))
	if _, ok := d.GetStatus().Metrics["fan.rpm"]; ok {
		t.Fatal("non-finite metric was exposed to JSON")
	}
	if samples := d.metricHistory["fan.rpm"]; len(samples) != 1 || !math.IsNaN(samples[0]) {
		t.Fatalf("missing metric must become sparkline gap: %+v", samples)
	}
	d.Stop()
}

func TestDeckMetricRenderAndFormatting(t *testing.T) {
	for _, tc := range []struct {
		metric string
		value  float64
		want   string
	}{
		{"cpu.temperature", 51.5, "52 C"},
		{"memory.utilization", 45.2, "45%"},
		{"memory.used", 1073741824, "1.0 GiB"},
		{"fan.rpm", 1199, "1199 rpm"},
		{"network.rx_rate", 1040000, "1.0 MB/s"},
	} {
		if got := deckMetricNumber(tc.metric, tc.value); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.metric, got, tc.want)
		}
	}
	key := KeyConfig{Metric: "cpu.utilization", ShowGraph: true, Label: "CPU"}
	if deckMetricVisualizationStyle(key) != "line" {
		t.Fatal("legacy showGraph layout must remain a line graph")
	}
	frame := renderDeckMetric(85, key, 48.5, true, []float64{10, 50, 90, 45}, nil)
	if frame.Bounds() != image.Rect(0, 0, 85, 85) {
		t.Fatal("live metric frame is not native key dimensions")
	}
	frame = renderDeckMetric(85, key, 0, false, []float64{math.NaN(), 43}, nil)
	if frame.Bounds().Dx() != 85 {
		t.Fatal("unavailable metric cannot be rendered")
	}
	if !strings.Contains(deckMetricNumber("cpu.clock_mhz", 3300), "MHz") {
		t.Fatal("clock unit missing")
	}
}

func TestMetricDueKeysHonorsPageAndTime(t *testing.T) {
	page := Page{ID: "home", Keys: []KeyConfig{
		{Index: 0, Metric: "cpu.temperature", RefreshSeconds: 10},
		{Index: 1, Metric: "gpu.temperature", RefreshSeconds: 2},
		{Index: 2, Label: "Static"},
	}}
	now := time.Unix(1700000000, 0)
	if len(metricDueKeys(page, nil, now)) != 2 {
		t.Fatal("new metric assignments are not immediately due")
	}
	last := map[string]time.Time{"home/0": now, "home/1": now}
	due := metricDueKeys(page, last, now.Add(3*time.Second))
	if len(due) != 1 || due[0].Index != 1 {
		t.Fatalf("incorrect due keys: %+v", due)
	}
}

func TestDeckMetricVisualizationStylesAndUnavailableReadings(t *testing.T) {
	for _, style := range []string{"value", "line", "bars", "fuel", "ring"} {
		key := KeyConfig{Metric: "cpu.utilization", MetricStyle: style, Label: "CPU"}
		available := renderDeckMetric(85, key, 54, true, []float64{10, 35, math.NaN(), 54, 90}, nil)
		if available.Bounds() != image.Rect(0, 0, 85, 85) {
			t.Fatalf("%s: wrong LCD dimensions: %v", style, available.Bounds())
		}
		unavailable := renderDeckMetric(85, key, 0, false, []float64{math.NaN()}, nil)
		if unavailable.Bounds() != available.Bounds() {
			t.Fatalf("%s: unavailable metric changed LCD dimensions", style)
		}
	}
	if got := deckMetricVisualizationStyle(KeyConfig{MetricStyle: "value", ShowGraph: true}); got != "value" {
		t.Fatalf("explicit choice must override legacy flag: %q", got)
	}
	if got := deckMetricGaugeFraction("cpu.utilization", 50, nil); math.Abs(got-0.5) > 0.001 {
		t.Fatalf("percentage gauge is not half full: %f", got)
	}
	if got := deckMetricGaugeFraction("cpu.temperature", 1000, nil); got != 1 {
		t.Fatalf("temperature gauge must clamp to full scale: %f", got)
	}
	if got := deckMetricGaugeFraction("fan.rpm", math.NaN(), nil); got != 0 {
		t.Fatalf("non-finite reading filled gauge: %f", got)
	}
}

func TestDeckRingGaugeDisplaysValueInsideArc(t *testing.T) {
	key := KeyConfig{Metric: "cpu.utilization", MetricStyle: "ring", Label: "CPU"}
	withValue := renderDeckMetric(85, key, 50, true, []float64{50}, nil)
	withoutValue := renderKeyLabel(85, "").(*image.RGBA)
	drawDeckRingGauge(withoutValue, 0.5, true)
	cx, cy, radius := deckRingGeometry(85)
	if radius < 20 || cy-radius < 17 || cy+radius >= 85 {
		t.Fatalf("ring geometry too small or overlaps label: (%d,%d) radius %d", cx, cy, radius)
	}
	changedCenter := false
	for y := cy-12; y < cy+12; y++ {
		for x := cx-18; x < cx+18; x++ {
			if withValue.At(x, y) != withoutValue.At(x, y) {
				changedCenter = true
			}
		}
	}
	if !changedCenter {
		t.Fatal("ring value was not drawn inside the arc")
	}
	// Unit text stays inside the circle for readings such as GPU power.
	power := KeyConfig{Metric: "gpu.power_w", MetricStyle: "ring", Label: "GPU Power"}
	if got := renderDeckMetric(85, power, 110, true, nil, nil); got.Bounds().Dx() != 85 {
		t.Fatal("ring with unit was not rendered")
	}
	// Unavailable sensors must retain the -- placeholder inside the ring.
	unknown := renderDeckMetric(85, key, 0, false, nil, nil)
	if unknown.Bounds() != withValue.Bounds() {
		t.Fatal("unavailable ring changed pixel bounds")
	}
}

package nexus

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestNexusPreviewOnlyFiniteCachedMetricsAndNullGaps(t *testing.T) {
	now := time.Unix(1000, 0)
	samples := MetricHistory{
		"cpu.utilization": {10, math.NaN(), 30, math.Inf(1)},
		"unknown.metric":  {25},
	}
	data := prepareNexusTelemetryPreview(MetricSnapshot{
		"cpu.utilization": 35,
		"gpu.temperature": math.NaN(),
		"unknown.metric":  100,
	}, samples, now.Add(-time.Second), now)
	if data.SampledAt != now.Add(-time.Second).Unix() {
		t.Fatal("incorrect sample timestamp")
	}
	if data.Metrics["cpu.utilization"] != 35 {
		t.Fatal("valid metric dropped")
	}
	if _, ok := data.Metrics["gpu.temperature"]; ok {
		t.Fatal("NaN metric leaked")
	}
	if _, ok := data.Metrics["unknown.metric"]; ok {
		t.Fatal("unapproved metric leaked")
	}
	points := data.History["cpu.utilization"]
	if len(points) != 4 || points[0] == nil || points[1] != nil || points[2] == nil || points[3] != nil {
		t.Fatalf("invalid gap representation: %v", points)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) == 0 {
		t.Fatal("missing JSON")
	}
}

func TestNexusPreviewDropsStaleSamples(t *testing.T) {
	now := time.Unix(500, 0)
	stale := prepareNexusTelemetryPreview(MetricSnapshot{"cpu.utilization": 12}, MetricHistory{"cpu.utilization": {12}}, now.Add(-11*time.Second), now)
	if stale.SampledAt != 0 || len(stale.Metrics) != 0 || len(stale.History) != 0 {
		t.Fatal("stale cache exposed")
	}
}

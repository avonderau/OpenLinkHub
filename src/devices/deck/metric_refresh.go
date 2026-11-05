package deck

import (
	"math"
	"strconv"
	"time"

	"OpenLinkHub/src/devices/nexus"
)

// metricLoop reads available sensor data at most once each second and only
// when the active page has a metric key whose interval is due.
func (d *Device) metricLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-d.done:
			return
		case now := <-ticker.C:
			d.refreshMetricKeys(now)
		}
	}
}

func (d *Device) refreshMetricKeys(now time.Time) {
	d.mu.RLock()
	if !d.connected || d.Model.Family != "mirabox" {
		d.mu.RUnlock()
		return
	}
	page, ok := d.activeMetricPageLocked()
	if !ok {
		d.mu.RUnlock()
		return
	}
	if len(metricDueKeys(page, d.metricUpdated, now)) == 0 {
		d.mu.RUnlock()
		return
	}
	d.mu.RUnlock()

	var snapshot nexus.MetricSnapshot
	if d.metricReader != nil {
		snapshot = d.metricReader()
	} else {
		snapshot = nexus.CollectHostMetrics()
		for name, reading := range nexus.ReadHostCounterRates(&d.counterSampler, now) {
			snapshot[name] = reading
		}
	}
	safe := copyDeckMetrics(snapshot)

	d.mu.Lock()
	defer d.mu.Unlock()
	select {
	case <-d.done:
		return
	default:
	}
	if d.hidDevice == nil || !d.connected || d.layout.ActivePage != page.ID {
		return
	}
	current, ok := d.activeMetricPageLocked()
	if !ok {
		return
	}
	due := metricDueKeys(current, d.metricUpdated, now)
	if len(due) == 0 {
		return
	}
	d.metricValues = safe
	d.metricSampleAt = now
	if d.metricHistory == nil {
		d.metricHistory = make(map[string][]float64)
	}
	seen := make(map[string]bool)
	for _, key := range due {
		if seen[key.Metric] {
			continue
		}
		seen[key.Metric] = true
		value, present := safe[key.Metric]
		if !present {
			value = math.NaN()
		}
		samples := append(d.metricHistory[key.Metric], value)
		if len(samples) > maxDeckMetricHistory {
			samples = append([]float64(nil), samples[len(samples)-maxDeckMetricHistory:]...)
		}
		d.metricHistory[key.Metric] = samples
	}
	if d.metricUpdated == nil {
		d.metricUpdated = make(map[string]time.Time)
	}
	updatedCount := 0
	for _, key := range due {
		if err := d.drawKeyLocked(key.Index, key); err != nil {
			d.status = "Metric LCD refresh failed: " + err.Error()
			return
		}
		d.metricUpdated[current.ID+"/"+strconv.Itoa(key.Index)] = now
		updatedCount++
	}
	if updatedCount > 0 {
		if err := MiraboxFlush(d.hidDevice, d.Model); err != nil {
			d.status = "Metric LCD refresh failed: " + err.Error()
		} else {
			d.status = "Connected; live metrics refreshed"
		}
	}
}

func (d *Device) activeMetricPageLocked() (Page, bool) {
	for _, page := range d.layout.Pages {
		if page.ID == d.layout.ActivePage {
			return page, true
		}
	}
	return Page{}, false
}

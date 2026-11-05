package nexus

import (
	"os"
	"strings"
	"time"

	"OpenLinkHub/src/config"
	"OpenLinkHub/src/systeminfo"
	"OpenLinkHub/src/temperatures"
)

// CollectHostMetrics is the shared stateless system telemetry reader used by
// Nexus and Deck. It reuses existing bounded Linux and NVIDIA metric sources.
// Network and disk throughput rates are tracked by each display's sampler.
func CollectHostMetrics() MetricSnapshot {
	values := MetricSnapshot{
		"cpu.temperature": float64(temperatures.GetCpuTemperature()),
		"gpu.temperature": float64(temperatures.GetGpuTemperature()),
		"cpu.utilization": systeminfo.GetCpuUtilization(),
		"gpu.utilization": float64(systeminfo.GetGPUUtilization()),
	}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		if used, percentage, ok := parseMemoryMetrics(string(data)); ok {
			values["memory.used"] = used
			values["memory.utilization"] = percentage
		}
	}
	for name, reading := range ReadLinuxExtendedMetrics(defaultLinuxExtendedMetricPaths()) {
		values[name] = reading
	}
	if info := systeminfo.GetInfo(); info != nil && info.GPU != nil {
		index := config.GetConfig().DefaultNvidiaGPU
		if gpu, ok := info.GPU[index]; ok && strings.Contains(strings.ToLower(gpu.Model), "nvidia") {
			for name, reading := range ReadNvidiaMetrics(index) {
				values[name] = reading
			}
		}
	}
	return values
}

// IsSupportedMetric uses the same vetted key registry as Nexus widgets.
func IsSupportedMetric(name string) bool {
	return supportedMetrics[name]
}

// ReadHostCounterRates shares Nexus's existing network/disk counters, but
// leaves prior samples with the caller so displays do not race each other.
func ReadHostCounterRates(sampler *CounterSampler, now time.Time) MetricSnapshot {
	result := MetricSnapshot{}
	if sampler == nil {
		return result
	}
	network, disk := detectCounterDevices()
	if network == "" || disk == "" {
		return result
	}
	counters, err := ReadLinuxCounters(network, disk)
	if err != nil {
		return result
	}
	rates := sampler.Sample(counters, now)
	if rates.Valid {
		result["network.rx_rate"] = rates.NetworkRXBytesPerSecond
		result["network.tx_rate"] = rates.NetworkTXBytesPerSecond
		result["disk.read_rate"] = rates.DiskReadBytesPerSecond
		result["disk.write_rate"] = rates.DiskWriteBytesPerSecond
	}
	return result
}

package nexus

import "sync"

// Nexus device actions use a fixed allowlist; no arbitrary device method,
// command, serial number or uncontrolled fan/pump speed can be supplied.
var nexusHardwareActions = map[string]bool{"rgb-on": true, "rgb-off": true}

var hardwareActionHandler struct {
	sync.RWMutex
	execute func(string)
}

func supportedNexusHardwareAction(action string) bool {
	return nexusHardwareActions[action]
}

// RegisterHardwareActionHandler binds the device-registry implementation.
// Devices without an RGB control method are ignored by the existing registry.
func RegisterHardwareActionHandler(handler func(string)) {
	hardwareActionHandler.Lock()
	hardwareActionHandler.execute = handler
	hardwareActionHandler.Unlock()
}

func executeNexusHardwareAction(action string) bool {
	if !supportedNexusHardwareAction(action) {
		return false
	}
	hardwareActionHandler.RLock()
	handler := hardwareActionHandler.execute
	hardwareActionHandler.RUnlock()
	if handler == nil {
		return false
	}
	handler(action)
	return true
}

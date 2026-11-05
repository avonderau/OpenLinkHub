package nexus

import (
	"errors"
	"os"

	"OpenLinkHub/src/config"
	"OpenLinkHub/src/logger"
)

// ValidateDeckLaunchAction uses the same URL and desktop-entry validation
// as the Nexus touchscreen.
func ValidateDeckLaunchAction(action, target string) error {
	return validateNexusLaunchAction(action, target)
}

// InitializeDeckDesktopLauncher starts the shared desktop bridge on hosts
// where a Deck is connected but a Corsair Nexus is not.
func InitializeDeckDesktopLauncher(root string) error {
	return initializeNexusDesktopLaunchBroker(root)
}

// DeckDesktopLauncherStatus distinguishes HID readiness from an actual
// desktop session. A background service must use an attached local agent.
func DeckDesktopLauncherStatus() (bool, string) {
	if nexusDirectDesktopAllowed(config.IsSystemService(), os.Geteuid(),
		os.Getenv("DISPLAY"), os.Getenv("WAYLAND_DISPLAY")) {
		return true, "Direct local graphical session available"
	}

	broker := nexusLaunchBrokerGlobal.broker
	if broker == nil {
		return false, "Desktop launcher unavailable: check OpenLinkHub service logs"
	}
	broker.mu.Lock()
	connected := broker.client != nil
	broker.mu.Unlock()
	if !connected {
		return false, "Desktop agent disconnected. Run OpenLinkHub --nexus-desktop-agent as your logged-in desktop user."
	}
	return true, "Local desktop agent connected"
}

// QueueDeckLaunchAction sends validated physical-button actions to the
// local desktop launcher; it never runs a shell or targets the WebUI client.
func QueueDeckLaunchAction(action, target string) error {
	if err := validateNexusLaunchAction(action, target); err != nil {
		return err
	}
	if ready, message := DeckDesktopLauncherStatus(); !ready {
		return errors.New(message)
	}
	select {
	case launchSlot <- struct{}{}:
	default:
		return errors.New("desktop launcher busy")
	}
	go func() {
		defer func() { <-launchSlot }()
		if err := dispatchNexusLaunchAction(action, target); err != nil {
			// Do not include the private URL or application target in logs.
			logger.Log(logger.Fields{"action": action, "error": err.Error()}).Warn("Deck desktop launch failed")
		}
	}()
	return nil
}

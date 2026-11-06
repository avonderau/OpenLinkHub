package deck

import (
	"fmt"
	"image"
)

// Family dispatch for the Deck hardware protocols. Device logic (layouts,
// actions, metrics, reconnects) is shared; only the wire format differs.

// initializeHardwareSession prepares a freshly opened handle before input
// is read. Both families start at 50% until the saved brightness is replayed.
func initializeHardwareSession(session hidSession, m Model) error {
	if m.Family == "elgato" {
		if err := ElgatoReset(session, m); err != nil {
			return fmt.Errorf("reset Stream Deck: %w", err)
		}
		if err := ElgatoBrightness(session, m, 50); err != nil {
			return fmt.Errorf("set startup brightness: %w", err)
		}
		return nil
	}
	return initializeMiraboxSession(session, m)
}

func setHardwareBrightness(session hidSession, m Model, percent uint8) error {
	if m.Family == "elgato" {
		return ElgatoBrightness(session, m, percent)
	}
	return MiraboxBrightness(session, m, percent)
}

// writeHardwareKey encodes and sends one key frame. Mirabox queues it until
// finishHardwareFrame; Elgato displays it immediately.
func writeHardwareKey(session hidSession, m Model, index int, frame image.Image) error {
	if m.Family == "elgato" {
		data, err := ElgatoKeyImage(m, index, frame)
		if err != nil {
			return err
		}
		return ElgatoWriteKeyImage(session, m, index, data)
	}
	jpegData, err := MiraboxKeyImage(m, index, frame)
	if err != nil {
		return err
	}
	return MiraboxWriteKeyJPEG(session, m, index, jpegData)
}

func finishHardwareFrame(session hidSession, m Model) error {
	if m.Family == "elgato" {
		return nil
	}
	return MiraboxFlush(session, m)
}

// hardwareKeyPresses returns the keys newly pressed by one input report.
// Mirabox sends one event per report. Elgato sends the full key state, so
// presses are the keys that changed from released to pressed; the caller
// keeps that state between reports.
func hardwareKeyPresses(m Model, report []byte, previous []bool) (presses []int, state []bool) {
	if m.Family == "elgato" {
		states, ok := ElgatoKeyStates(m, report)
		if !ok {
			return nil, previous
		}
		for i, down := range states {
			if down && (i >= len(previous) || !previous[i]) && m.CanPress(i) {
				presses = append(presses, i)
			}
		}
		return presses, states
	}
	key, pressed, ok := MiraboxKeyEvent(m, report)
	if ok && pressed {
		presses = append(presses, key)
	}
	return presses, previous
}

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
func hardwareKeyPresses(m Model, report []byte, previous []bool) (presses []int, state []bool) {
	presses, _, state = hardwareKeyEvents(m, report, previous)
	return presses, state
}

// hardwareKeyEvents returns the keys pressed and released by one input
// report. Mirabox sends one event per report (protocol 1 only presses).
// Elgato sends the full key state, so presses and releases are the keys
// that changed; the caller keeps that state between reports.
func hardwareKeyEvents(m Model, report []byte, previous []bool) (presses, releases []int, state []bool) {
	if m.Family == "elgato" {
		states, ok := ElgatoKeyStates(m, report)
		if !ok {
			return nil, nil, previous
		}
		for i, down := range states {
			was := i < len(previous) && previous[i]
			if !m.CanPress(i) || down == was {
				continue
			}
			if down {
				presses = append(presses, i)
			} else {
				releases = append(releases, i)
			}
		}
		return presses, releases, states
	}
	key, pressed, ok := MiraboxKeyEvent(m, report)
	if ok && pressed {
		presses = append(presses, key)
	} else if ok && m.SupportsKeyRelease() {
		releases = append(releases, key)
	}
	return presses, releases, previous
}

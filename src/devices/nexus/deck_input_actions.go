package nexus

import (
 "fmt"

 "OpenLinkHub/src/inputmanager"
)

// Reuse Nexus's validated shortcut parser and Linux uinput keyboard backend
// for physical Deck keys. These actions emit bounded keyboard/media events.
// They never spawn shells, launch executables, or accept unvalidated key codes.
func ValidateDeckShortcut(target string) error {
 _, err := parseKeyChord(target)
 return err
}

func ExecuteDeckShortcut(target string) error {
 chord, err := parseKeyChord(target)
 if err != nil { return err }
 nexusShortcutMutex.Lock()
 defer nexusShortcutMutex.Unlock()
 sendKeyChord(chord, inputmanager.InputControlKeyboardHold)
 return nil
}

// Only these fixed media-key names can be emitted by Deck widgets.
var deckMediaKeys = map[string]uint16{
 "play-pause":inputmanager.MediaPlayPause,
 "stop":inputmanager.MediaStop,
 "previous-track":inputmanager.MediaPrev,
 "next-track":inputmanager.MediaNext,
 "volume-up":inputmanager.VolumeUp,
 "volume-down":inputmanager.VolumeDown,
 "volume-mute":inputmanager.VolumeMute,
}

func ValidDeckMediaAction(target string) bool {
 _, ok := deckMediaKeys[target]
 return ok
}

func ExecuteDeckMediaAction(target string) error {
 code, ok := deckMediaKeys[target]
 if !ok { return fmt.Errorf("unsupported Deck media action %q",target) }
 inputmanager.InputControlKeyboard(code, false)
 return nil
}

package nexus

import (
 "fmt"

 "OpenLinkHub/src/inputmanager"
)

// Explicit media controls remain available without standalone shortcuts.
var deckMediaKeys=map[string]uint16{
 "play-pause":inputmanager.MediaPlayPause,
 "stop":inputmanager.MediaStop,
 "previous-track":inputmanager.MediaPrev,
 "next-track":inputmanager.MediaNext,
 "volume-up":inputmanager.VolumeUp,
 "volume-down":inputmanager.VolumeDown,
 "volume-mute":inputmanager.VolumeMute,
}

func ValidDeckMediaAction(target string) bool {
 _,ok:=deckMediaKeys[target]
 return ok
}
func ExecuteDeckMediaAction(target string) error {
 code,ok:=deckMediaKeys[target]
 if !ok{return fmt.Errorf("unsupported Deck media action %q",target)}
 nexusKeyboardMutex.Lock()
 defer nexusKeyboardMutex.Unlock()
 inputmanager.InputControlKeyboard(code,false)
 return nil
}

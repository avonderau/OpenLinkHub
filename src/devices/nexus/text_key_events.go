package nexus

import (
 "sync"
 "OpenLinkHub/src/inputmanager"
)

// Shared low-level key emission for printable text and media actions.
// This is not a configurable keyboard-shortcut action.
type textKeyChord struct {
 modifiers []uint16
 key uint16
}

var nexusTextKeyCodes=map[string]uint16{
 "A":inputmanager.KeyA,"B":inputmanager.KeyB,"C":inputmanager.KeyC,
 "D":inputmanager.KeyD,"E":inputmanager.KeyE,"F":inputmanager.KeyF,
 "G":inputmanager.KeyG,"H":inputmanager.KeyH,"I":inputmanager.KeyI,
 "J":inputmanager.KeyJ,"K":inputmanager.KeyK,"L":inputmanager.KeyL,
 "M":inputmanager.KeyM,"N":inputmanager.KeyN,"O":inputmanager.KeyO,
 "P":inputmanager.KeyP,"Q":inputmanager.KeyQ,"R":inputmanager.KeyR,
 "S":inputmanager.KeyS,"T":inputmanager.KeyT,"U":inputmanager.KeyU,
 "V":inputmanager.KeyV,"W":inputmanager.KeyW,"X":inputmanager.KeyX,
 "Y":inputmanager.KeyY,"Z":inputmanager.KeyZ,
 "0":inputmanager.Number0,"1":inputmanager.Number1,
 "2":inputmanager.Number2,"3":inputmanager.Number3,
 "4":inputmanager.Number4,"5":inputmanager.Number5,
 "6":inputmanager.Number6,"7":inputmanager.Number7,
 "8":inputmanager.Number8,"9":inputmanager.Number9,
}

var nexusKeyboardMutex sync.Mutex

func sendTextKeyChord(chord textKeyChord, emit func(uint16,bool)) {
 pressed:=make([]uint16,0,len(chord.modifiers)+1)
 defer func(){
  for i:=len(pressed)-1;i>=0;i--{emit(pressed[i],false)}
 }()
 for _,modifier:=range chord.modifiers {
  pressed=append(pressed,modifier)
  emit(modifier,true)
 }
 pressed=append(pressed,chord.key)
 emit(chord.key,true)
}

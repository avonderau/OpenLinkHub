package nexus

import (
 "fmt"
 "strings"
 "sync"

 "OpenLinkHub/src/inputmanager"
)

// Nexus keyboard actions are a bounded sequence of real virtual-keyboard
// events, never a shell command. They share the existing Linux uinput backend.
type keyChord struct {
 modifiers []uint16
 key uint16
}

var shortcutKeyCodes=func()map[string]uint16{
 keys:=map[string]uint16{
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
  "ENTER":inputmanager.KeyEnter,
  "TAB":inputmanager.KeyTab,
  "SPACE":inputmanager.KeySpace,
  "ESC":inputmanager.KeyEsc,
  "ESCAPE":inputmanager.KeyEsc,
  "BACKSPACE":inputmanager.KeyBack,
  "DELETE":inputmanager.KeyDel,
  "INSERT":inputmanager.KeyIns,
  "HOME":inputmanager.KeyHome,
  "END":inputmanager.KeyEnd,
  "PAGEUP":inputmanager.KeyPgUp,
  "PAGEDOWN":inputmanager.KeyPgDn,
  "UP":inputmanager.KeyUp,
  "DOWN":inputmanager.KeyDown,
  "LEFT":inputmanager.KeyLeft,
  "RIGHT":inputmanager.KeyRight,
 }
 for n:=1;n<=12;n++{keys[fmt.Sprintf("F%d",n)]=inputmanager.KeyF1+uint16(n-1)}
 for n:=13;n<=24;n++{keys[fmt.Sprintf("F%d",n)]=inputmanager.KeyF13+uint16(n-13)}
 return keys
}()

var shortcutModifiers=map[string]uint16{
 "CTRL":inputmanager.KeyLeftCtrl,
 "CONTROL":inputmanager.KeyLeftCtrl,
 "ALT":inputmanager.KeyLeftAlt,
 "SHIFT":inputmanager.KeyLeftShift,
 "SUPER":inputmanager.KeyWindowsKey,
 "META":inputmanager.KeyWindowsKey,
 "WIN":inputmanager.KeyWindowsKey,
}

func parseKeyChord(value string)(keyChord,error){
 var chord keyChord
 if len(value)==0||len(value)>80{return chord,fmt.Errorf("keyboard shortcut must be 1-80 characters")}
 parts:=strings.Split(value,"+")
 if len(parts)>5{return chord,fmt.Errorf("keyboard shortcut has too many keys")}
 seen:=map[uint16]bool{}
 for i,raw:=range parts{
  name:=strings.ToUpper(strings.TrimSpace(raw))
  if name==""{return chord,fmt.Errorf("empty shortcut key")}
  if modifier,ok:=shortcutModifiers[name];ok{
   if i==len(parts)-1{return chord,fmt.Errorf("shortcut must end with a non-modifier key")}
   if seen[modifier]{return chord,fmt.Errorf("duplicate shortcut modifier")}
   seen[modifier]=true
   chord.modifiers=append(chord.modifiers,modifier)
   continue
  }
  if i!=len(parts)-1{return chord,fmt.Errorf("shortcut modifiers must precede the key")}
  code,ok:=shortcutKeyCodes[name]
  if !ok{return chord,fmt.Errorf("unsupported shortcut key %q",name)}
  chord.key=code
 }
 if chord.key==0{return chord,fmt.Errorf("missing shortcut key")}
 return chord,nil
}

var nexusShortcutMutex sync.Mutex

func sendKeyChord(chord keyChord, emit func(uint16,bool)){
 pressed:=make([]uint16,0,len(chord.modifiers)+1)
 // Release all held keys even if a future emitter fails midway.
 defer func(){
  for i:=len(pressed)-1;i>=0;i--{emit(pressed[i],false)}
 }()
 for _,modifier:=range chord.modifiers{
  pressed=append(pressed,modifier)
  emit(modifier,true)
 }
 pressed=append(pressed,chord.key)
 emit(chord.key,true)
}

func executeKeyChord(value string){
 chord,err:=parseKeyChord(value)
 if err!=nil{return}
 nexusShortcutMutex.Lock()
 defer nexusShortcutMutex.Unlock()
 sendKeyChord(chord,inputmanager.InputControlKeyboardHold)
}

package nexus

import (
 "strings"
 "testing"

 "OpenLinkHub/src/inputmanager"
)

func TestParseKeyChord(t *testing.T){
 cases:=[]struct{
  input string
  key uint16
  modifierCount int
 }{
  {"Ctrl+Alt+T",inputmanager.KeyT,2},
  {"Shift+F12",inputmanager.KeyF12,1},
  {"Meta+PageDown",inputmanager.KeyPgDn,1},
  {"A",inputmanager.KeyA,0},
  {"Ctrl+SHIFT+DELETE",inputmanager.KeyDel,2},
 }
 for _,tt:=range cases{
  parsed,err:=parseKeyChord(tt.input)
  if err!=nil{t.Fatalf("%s: %v",tt.input,err)}
  if parsed.key!=tt.key || len(parsed.modifiers)!=tt.modifierCount {t.Fatalf("%s returned unexpected chord %+v",tt.input,parsed)}
 }
}

func TestRejectUnsafeOrMalformedShortcuts(t *testing.T){
 invalid:=[]string{"","Ctrl","Ctrl+","Ctrl+Ctrl+T","T+Ctrl","Alt+UnknownKey","Ctrl+Alt+Shift+Super+F1+F2","Ctrl && rm -rf /","Ctrl+;","Ctrl+Alt+T+X",strings.Repeat("x",81)}
 for _,value:=range invalid{
  if _,err:=parseKeyChord(value);err==nil{t.Errorf("accepted invalid keyboard shortcut %q",value)}
 }
}

func TestShortcutEventReleaseOrder(t *testing.T){
 chord,err:=parseKeyChord("Ctrl+Shift+V")
 if err!=nil{t.Fatal(err)}
 type event struct{key uint16;down bool}
 events:=[]event{}
 sendKeyChord(chord,func(key uint16,down bool){events=append(events,event{key,down})})
 want:=[]event{
  {inputmanager.KeyLeftCtrl,true},{inputmanager.KeyLeftShift,true},{inputmanager.KeyV,true},
  {inputmanager.KeyV,false},{inputmanager.KeyLeftShift,false},{inputmanager.KeyLeftCtrl,false},
 }
 if len(events)!=len(want){t.Fatalf("expected %d key events, got %+v",len(want),events)}
 for i:=range want{if events[i]!=want[i]{t.Errorf("event %d got %+v want %+v",i,events[i],want[i])}}
}

func TestShortcutValidationInScreens(t *testing.T){
 config:=ScreenConfiguration{Version:1,ActiveScreenID:"keys",Screens:[]Screen{{
  ID:"keys",Name:"Keyboard",
  Widgets:[]Widget{{
   ID:"hotkey",Type:"button",X:0,Y:0,Width:100,Height:48,Action:"hotkey",Target:"Ctrl+Alt+T",
  },{
   ID:"macro",Type:"button",X:110,Y:0,Width:100,Height:48,Action:"macro",
   Steps:[]MacroStep{{Type:"hotkey",Value:"Ctrl+Shift+C"},{Type:"delay",DelayMs:200}},
  }},
 }}}
 if err:=config.Validate();err!=nil{t.Fatal(err)}
 config.Screens[0].Widgets[0].Target="Ctrl+Invalid"
 if err:=config.Validate();err==nil{t.Fatal("accepted invalid single shortcut")}
 config.Screens[0].Widgets[0].Target="Ctrl+Alt+T"
 config.Screens[0].Widgets[1].Steps[0].Value="Ctrl+Alt+"
 if err:=config.Validate();err==nil{t.Fatal("accepted invalid macro shortcut")}
}

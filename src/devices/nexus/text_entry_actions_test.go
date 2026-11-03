package nexus

import (
 "strings"
 "testing"

 "OpenLinkHub/src/inputmanager"
)

func TestNexusTextEntryASCIIKeyTranslation(t *testing.T){
 chords,err:=parseNexusTextEntry("Aa0!_ ?")
 if err!=nil{t.Fatal(err)}
 if len(chords)!=7{t.Fatalf("unexpected chord count: %d",len(chords))}
 type expected struct{code uint16;shift bool}
 want:=[]expected{
  {inputmanager.KeyA,true},
  {inputmanager.KeyA,false},
  {inputmanager.Number0,false},
  {inputmanager.Number1,true},
  {inputmanager.KeyMinus,true},
  {inputmanager.KeySpace,false},
  {inputmanager.KeySlash,true},
 }
 for i,key:=range chords{
  shifted:=len(key.modifiers)>0
  if key.key!=want[i].code||shifted!=want[i].shift{
   t.Errorf("key %d got %+v, want %+v",i,key,want[i])
  }
  if shifted && key.modifiers[0]!=inputmanager.KeyLeftShift{t.Errorf("unexpected modifier for key %d",i)}
 }
 if _,err:=parseNexusTextEntry("Hello, world!");err!=nil{t.Fatal(err)}
 if _,err:=parseNexusTextEntry("file://C:\\path");err!=nil{t.Fatal(err)}
}

func TestNexusTextEntryRejectsUnsafeOrUnrepresentableCharacters(t *testing.T){
 invalid:=[]string{"",strings.Repeat("a",81),"New\nline","Tabbed\ttext","\x00hidden","em dash —","☃"}
 for _,value:=range invalid{
  if _,err:=parseNexusTextEntry(value);err==nil{
   t.Errorf("invalid text entry accepted: %q",value)
  }
 }
}

func TestNexusTextEntryButtonAndMacroValidation(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"main",Screens:[]Screen{{
  ID:"main",Name:"Main",Widgets:[]Widget{
   {ID:"text",Type:"button",X:0,Y:0,Width:96,Height:48,Action:"text",Target:"Hello, World!"},
   {ID:"macro",Type:"button",Action:"macro",Target:"1",X:112,Y:0,Width:128,Height:48},
  },
 }}}
 if err:=cfg.Validate();err!=nil{t.Fatalf("text actions should be valid: %v",err)}
 cfg.Screens[0].Widgets[0].Target="bad\nnewline"
 if err:=cfg.Validate();err==nil{t.Fatal("button allowed newline")}
 cfg.Screens[0].Widgets[0].Target="OK"
 cfg.Screens[0].Widgets[1].Target="☃"
 if err:=cfg.Validate();err==nil{t.Fatal("macro accepted invalid shared ID")}
}

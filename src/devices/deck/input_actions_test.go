package deck

import (
 "errors"
 "strings"
 "testing"
)

// No virtual keyboard is created during these tests. We inject a stub after
// the same layout validation the production HID reader uses.
func TestPhysicalDeckKeyboardAndMediaActions(t *testing.T) {
 d:=testDevice(t)
 d.layout.Pages[0].Keys=[]KeyConfig{
  {Index:0,Action:"hotkey",Target:"Ctrl+Alt+T"},
  {Index:1,Action:"media",Target:"play-pause"},
  {Index:2,Action:"media",Target:"volume-mute"},
 }
 if err:=d.layout.Validate(d.Model);err!=nil{t.Fatal(err)}
 var actions []string
 d.inputActionRunner=func(kind,target string)error{
  // Must not run while holding the Deck device mutex.
  _=d.GetStatus()
  actions=append(actions,kind+":"+target)
  return nil
 }
 d.handleKeyPress(0)
 d.handleKeyPress(1)
 d.handleKeyPress(2)
 d.handleKeyPress(3)
 want:=[]string{"hotkey:Ctrl+Alt+T","media:play-pause","media:volume-mute"}
 if len(actions)!=len(want){t.Fatalf("actions=%v, expected %v",actions,want)}
 for i:=range want{if actions[i]!=want[i]{t.Fatalf("action[%d]=%q, want %q",i,actions[i],want[i])}}
 if d.GetStatus().ButtonEvents!=4{t.Fatal("key events should remain visible in diagnostics")}
}

func TestDeckInputActionValidationRejectsArbitraryCommands(t *testing.T){
 d:=testDevice(t)
 cases:=[]KeyConfig{
  {Index:0,Action:"hotkey",Target:""},
  {Index:0,Action:"hotkey",Target:"Ctrl+Alt+$(touch /tmp/unsafe)"},
  {Index:0,Action:"hotkey",Target:"Ctrl+Ctrl+T"},
  {Index:0,Action:"media",Target:"rm -rf /"},
  {Index:0,Action:"media",Target:"not-an-action"},
  {Index:0,Action:"media",Target:"volume-up",TargetPage:"home"},
  {Index:0,Action:"hotkey",Target:"Ctrl+T",TargetPage:"home"},
  {Index:0,Action:"page-next",Target:"Ctrl+T"},
 }
 for _,bad:=range cases{
  l:=DefaultLayout(d.Model)
  l.Pages[0].Keys=[]KeyConfig{bad}
  if err:=l.Validate(d.Model);err==nil{
   t.Fatalf("invalid action was accepted: %+v",bad)
  }
 }
}

func TestDeckInputActionPersistenceAndFailureStatus(t *testing.T){
 d:=testDevice(t)
 layout:=DefaultLayout(d.Model)
 layout.Pages[0].Keys=[]KeyConfig{
  {Index:0,Action:"hotkey",Target:"Super+D"},
  {Index:1,Action:"media",Target:"next-track"},
 }
 if err:=SaveLayout(d.root,d.Serial,d.Model,layout);err!=nil{t.Fatal(err)}
 stored,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil{t.Fatal(err)}
 if len(stored.Pages[0].Keys)!=2 || stored.Pages[0].Keys[0].Target!="Super+D"{
  t.Fatalf("action targets were not persisted: %+v",stored)
 }
 d.layout=stored
 d.inputActionRunner=func(kind,target string)error{return errors.New("input temporarily unavailable")}
 d.handleKeyPress(0)
 if !strings.Contains(d.GetStatus().Status,"Button action failed"){
  t.Fatalf("action failure was not shown: %q",d.GetStatus().Status)
 }
 if d.GetStatus().ActivePage!="home"{t.Fatal("input action changed active page")}
}

func TestMalformedInputActionCannotReachInputBackend(t *testing.T){
 d:=testDevice(t)
 d.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"hotkey",Target:"not-valid++"}}
 called:=false
 d.inputActionRunner=func(string,string)error{called=true;return nil}
 d.handleKeyPress(0)
 if called{t.Fatal("unvalidated input action was executed")}
}

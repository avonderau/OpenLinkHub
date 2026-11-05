package deck

import (
 "errors"
 "strings"
 "testing"
)

func TestPhysicalDeckMediaActions(t *testing.T) {
 d:=testDevice(t)
 d.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"media",Target:"play-pause"},{Index:1,Action:"media",Target:"volume-mute"}}
 if err:=d.layout.Validate(d.Model);err!=nil{t.Fatal(err)}
 var actions []string
 d.inputActionRunner=func(kind,target string)error{
  _=d.GetStatus()
  actions=append(actions,kind+":"+target)
  return nil
 }
 d.handleKeyPress(0);d.handleKeyPress(1);d.handleKeyPress(3)
 if len(actions)!=2||actions[0]!="media:play-pause"||actions[1]!="media:volume-mute"{t.Fatalf("unexpected actions: %v",actions)}
 if d.GetStatus().ButtonEvents!=3{t.Fatal("key events not counted")}
}

func TestRemovedDeckShortcutRejected(t *testing.T) {
 d:=testDevice(t)
 for _,key:=range []KeyConfig{{Index:0,Action:"hotkey",Target:"Ctrl+Alt+T"},{Index:0,Action:"media",Target:"rm -rf /"},{Index:0,Action:"media",Target:"volume-up",TargetPage:"home"}}{
  l:=DefaultLayout(d.Model);l.Pages[0].Keys=[]KeyConfig{key}
  if err:=l.Validate(d.Model);err==nil{t.Fatalf("invalid Deck action accepted: %+v",key)}
 }
 d.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"hotkey",Target:"Super+D"}}
 called:=false
 d.inputActionRunner=func(string,string)error{called=true;return nil}
 d.handleKeyPress(0)
 if called{t.Fatal("removed shortcut action was executed")}
}

func TestDeckMediaPersistenceAndFailureStatus(t *testing.T) {
 d:=testDevice(t)
 layout:=DefaultLayout(d.Model)
 layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"media",Target:"next-track"}}
 if err:=SaveLayout(d.root,d.Serial,d.Model,layout);err!=nil{t.Fatal(err)}
 stored,err:=LoadLayout(d.root,d.Serial,d.Model);if err!=nil{t.Fatal(err)}
 if len(stored.Pages[0].Keys)!=1||stored.Pages[0].Keys[0].Target!="next-track"{t.Fatalf("media not persisted: %+v",stored)}
 d.layout=stored
 d.inputActionRunner=func(kind,target string)error{return errors.New("input unavailable")}
 d.handleKeyPress(0)
 if !strings.Contains(d.GetStatus().Status,"Button action failed"){t.Fatalf("action failure missing: %q",d.GetStatus().Status)}
}

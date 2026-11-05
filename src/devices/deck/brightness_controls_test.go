package deck

import (
 "bytes"
 "strings"
 "testing"
)

func lastBrightnessCommand(reports [][]byte)(byte,bool){
 for i:=len(reports)-1;i>=0;i--{
  report:=reports[i]
  if len(report)>=12&&bytes.Equal(report[6:9],[]byte("LIG")) {
   return report[11],true
  }
 }
 return 0,false
}

func TestDeckBrightnessControlsPersistAndReconnect(t *testing.T){
 d:=testDevice(t)
 d.layout.Pages[0].Keys=[]KeyConfig{
  {Index:0,Action:"brightness-up"},
  {Index:1,Action:"brightness-down"},
  {Index:2,Action:"brightness-set",Target:"75"},
 }
 h:=new(fakeDeckSession)
 if err:=d.attachSession(h);err!=nil{t.Fatal(err)}
 check:=func(index,want int){
  t.Helper()
  d.handleKeyPress(index)
  level,ok:=lastBrightnessCommand(h.reports)
  if !ok||int(level)!=want{t.Fatalf("action %d: HID level %d valid %v, want %d",index,level,ok,want)}
  if d.GetLayout().Brightness!=want{t.Fatalf("in-memory brightness %d, want %d",d.GetLayout().Brightness,want)}
  saved,err:=LoadLayout(d.root,d.Serial,d.Model)
  if err!=nil||saved.Brightness!=want{t.Fatalf("brightness not persisted: %d %v",saved.Brightness,err)}
 }
 check(0,60)
 check(0,70)
 check(1,60)
 check(2,75)
 for i:=0;i<4;i++{check(0,min(100,85+i*10))}
 for i:=0;i<12;i++{d.handleKeyPress(1)}
 if d.GetLayout().Brightness!=10{t.Fatalf("min brightness should clamp at 10, got %d",d.GetLayout().Brightness)}
 d.Stop()

 // A new session initializes at 50%, then restores the saved 10% level.
 again:=testDevice(t)
 saved,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil{t.Fatal(err)}
 again.root=d.root
 again.Serial=d.Serial
 again.layout=saved
 fresh:=new(fakeDeckSession)
 if err:=again.attachSession(fresh);err!=nil{t.Fatal(err)}
 got,ok:=lastBrightnessCommand(fresh.reports)
 if !ok||got!=10{t.Fatalf("reconnect brightness %d valid %v",got,ok)}
 again.Stop()
}

func TestDeckBrightnessValidationAndOutputFailure(t *testing.T){
 d:=testDevice(t)
 valid:=DefaultLayout(d.Model)
 valid.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"brightness-set",Target:"25"}}
 if err:=valid.Validate(d.Model);err!=nil{t.Fatal(err)}
 for _,target:=range []string{"","0","-1","101","junk","1.0"}{
  attempt:=cloneLayout(valid)
  attempt.Pages[0].Keys[0].Target=target
  if err:=attempt.Validate(d.Model);err==nil{t.Errorf("accepted unsafe brightness target %q",target)}
 }
 for _,action:=range []string{"brightness-up","brightness-down"}{
  attempt:=cloneLayout(valid);attempt.Pages[0].Keys[0].Action=action;attempt.Pages[0].Keys[0].Target="50"
  if err:=attempt.Validate(d.Model);err==nil{t.Errorf("accepted target on %s",action)}
 }
 valid.Brightness=101
 if err:=valid.Validate(d.Model);err==nil{t.Fatal("invalid saved brightness accepted")}
 d.layout=DefaultLayout(d.Model)
 d.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"brightness-up"}}
 h:=new(fakeDeckSession)
 if err:=d.attachSession(h);err!=nil{t.Fatal(err)}
 h.failOutput=true
 d.handleKeyPress(0)
 if d.GetLayout().Brightness!=0||!strings.Contains(d.GetStatus().Status,"brightness failed"){
  t.Fatal("failed HID report must not update saved brightness")
 }
 d.Stop()
}

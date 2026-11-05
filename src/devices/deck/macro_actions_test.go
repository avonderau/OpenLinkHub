package deck

import (
 "strings"
 "testing"
 "time"
)

func TestSavedDeckMacroValidationAndPersistence(t *testing.T) {
 d:=testDevice(t)
 l:=DefaultLayout(d.Model)
 l.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"macro",Target:"7",Metric:"cpu.temperature",RefreshSeconds:5,ShowGraph:true}}
 if err:=l.Validate(d.Model);err!=nil{t.Fatal(err)}
 if err:=SaveLayout(d.root,d.Serial,d.Model,l);err!=nil{t.Fatal(err)}
 stored,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil||stored.Pages[0].Keys[0].Target!="7"||stored.Pages[0].Keys[0].Metric!="cpu.temperature" {
  t.Fatalf("macro and metric assignment did not persist: %+v %v",stored,err)
 }
 for _,target:=range []string{"","0","0001","../../etc/passwd","1;rm","1234567890"}{
  invalid:=DefaultLayout(d.Model)
  invalid.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"macro",Target:target}}
  if err:=invalid.Validate(d.Model);err==nil{t.Errorf("invalid macro target %q accepted",target)}
 }
}

func TestDeckMacroDispatchNonblockingAndSingleFlight(t *testing.T){
 d:=testDevice(t)
 d.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"macro",Target:"42"}}
 entered:=make(chan struct{},2)
 release:=make(chan struct{})
 d.inputActionRunner=func(action,target string)error{
  if action!="macro"||target!="42"{t.Errorf("unexpected macro: %q %q",action,target)}
  _=d.GetStatus()
  entered<-struct{}{}
  <-release
  return nil
 }
 d.handleKeyPress(0)
 select{case <-entered:case <-time.After(2*time.Second):t.Fatal("macro did not start")}
 d.handleKeyPress(0)
 select{case <-entered:t.Fatal("macro executed concurrently");default:}
 close(release)
 deadline:=time.After(2*time.Second)
 for{
  d.mu.RLock();running:=d.macroRunning;d.mu.RUnlock()
  if !running{break}
  select{case <-deadline:t.Fatal("macro did not finish");default:time.Sleep(time.Millisecond)}
 }
 d.Stop()
}
func TestMissingDeckMacroReportsFailure(t *testing.T){
 d:=testDevice(t)
 d.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"macro",Target:"999999999"}}
 d.handleKeyPress(0)
 deadline:=time.After(3*time.Second)
 for{
  if strings.Contains(d.GetStatus().Status,"Saved macro failed:"){break}
  select{case <-deadline:t.Fatal("missing macro failure not reported");default:time.Sleep(time.Millisecond)}
 }
 d.Stop()
}

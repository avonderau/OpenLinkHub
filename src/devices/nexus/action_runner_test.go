package nexus

import (
 "testing"
 "time"
)

func TestNexusMacroRunnerExecutesInOrder(t *testing.T){
 steps:=[]MacroStep{{Type:"hotkey",Value:"Ctrl+C"},{Type:"delay",DelayMs:1},{Type:"mouse",Value:"left"},{Type:"next"}}
 var got []string
 runNexusMacroSteps(steps,make(chan struct{}),func(k,v string){got=append(got,k+":"+v)})
 want:=[]string{"hotkey:Ctrl+C","mouse:left","next:"}
 if len(got)!=len(want){t.Fatalf("got steps %v",got)}
 for i:=range want{if got[i]!=want[i]{t.Fatalf("step %d got %q expected %q",i,got[i],want[i])}}
}
func TestNexusMacroRunnerStopsOnDeviceShutdown(t *testing.T){
 done:=make(chan struct{})
 started:=make(chan struct{})
 result:=make(chan struct{})
 count:=0
 go func(){
  runNexusMacroSteps([]MacroStep{{Type:"next"},{Type:"delay",DelayMs:5000},{Type:"previous"}},done,func(action,target string){
   count++;if action=="next"{close(started)}
  })
  close(result)
 }()
 select{case <-started:case <-time.After(time.Second):t.Fatal("macro did not start")}
 close(done)
 select{case <-result:case <-time.After(250*time.Millisecond):t.Fatal("macro did not cancel delay")}
 if count!=1{t.Fatalf("macro executed %d actions after cancellation",count)}
}
func TestNexusMacroRunnerAlreadyCanceled(t *testing.T){
 done:=make(chan struct{});close(done)
 ran:=false
 runNexusMacroSteps([]MacroStep{{Type:"next"}},done,func(_, _ string){ran=true})
 if ran{t.Fatal("macro ran after shutdown")}
}

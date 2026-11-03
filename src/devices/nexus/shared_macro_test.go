package nexus

import (
 "OpenLinkHub/src/macro"
 "context"
 "testing"
 "time"
)

func TestNexusMacroIDValidation(t *testing.T){
 for _,v:=range []string{"1","42","999999999"}{if !validSharedMacroID(v){t.Errorf("rejected %s",v)}}
 for _,v:=range []string{"","0","-1","001","1;reboot","1234567890"," 2","2.5"}{if validSharedMacroID(v){t.Errorf("accepted %s",v)}}
}
func TestNexusSharedMacroValidation(t *testing.T){
 good:=macro.Macro{Id:1,Repeat:1,Actions:map[int]macro.Actions{0:{ActionType:5,ActionDelay:1}}}
 if !validateSharedMacro(good){t.Fatal("valid shared macro rejected")}
 bad:=good
 bad.Repeat=-1
 if validateSharedMacro(bad){t.Fatal("infinite repeat accepted")}
 bad=good
 bad.Actions=map[int]macro.Actions{0:{ActionType:99}}
 if validateSharedMacro(bad){t.Fatal("arbitrary action accepted")}
 bad=good
 bad.Actions=map[int]macro.Actions{1:{ActionType:5}}
 if validateSharedMacro(bad){t.Fatal("noncontiguous sequence accepted")}
}
func TestNexusSharedMacroRespectsCancellation(t *testing.T){
 profile:=macro.Macro{Id:1,Actions:map[int]macro.Actions{0:{ActionType:5,ActionDelay:5000}}}
 ctx,cancel:=context.WithCancel(context.Background())
 cancel()
 start:=time.Now()
 if runSavedMacro(ctx,profile){t.Fatal("cancelled macro executed")}
 if time.Since(start)>time.Second{t.Fatal("cancelled macro blocked")}
}

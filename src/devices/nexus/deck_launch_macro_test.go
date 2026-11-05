package nexus

import (
 "context"
 "testing"
)

func TestDeckLaunchValidationReusesDesktopRules(t *testing.T){
 for _,item:=range []struct{action,target string}{
  {"url","https://example.com/path"},
  {"url","http://localhost:3000/status"},
  {"application","org.gnome.Terminal.desktop"},
 } {
  if err:=ValidateDeckLaunchAction(item.action,item.target);err!=nil{
   t.Errorf("valid action rejected %+v: %v",item,err)
  }
 }
 for _,item:=range []struct{action,target string}{
  {"url","file:///etc/shadow"},
  {"url","https://user:pw@example.com"},
  {"application","../../bin/sh"},
  {"application","code.desktop --command"},
  {"shell","echo hello"},
 }{
  if err:=ValidateDeckLaunchAction(item.action,item.target);err==nil{
   t.Errorf("unsafe action accepted: %+v",item)
  }
  if err:=QueueDeckLaunchAction(item.action,item.target);err==nil{
   t.Errorf("unsafe action queued: %+v",item)
  }
 }
}

func TestDeckSavedMacroValidationUsesNexusBounds(t *testing.T){
 for _,id:=range []string{"1","42","999999999"}{
  if !ValidateDeckSavedMacroID(id){t.Errorf("valid macro ID rejected: %s",id)}
 }
 for _,id:=range []string{"","0","-1","0001","../../../etc/shadow","4;rm","1234567890"}{
  if ValidateDeckSavedMacroID(id){t.Errorf("unsafe macro accepted: %s",id)}
  if err:=RunDeckSavedMacro(context.Background(),id);err==nil{
   t.Errorf("invalid macro should fail: %s",id)
  }
 }
 if nexusSharedMacroMaxDuration.Seconds()>15{t.Fatal("macro limit exceeded")}
}

func TestDeckDesktopLaunchRequiresConnectedAgent(t *testing.T) {
 t.Setenv("DISPLAY","")
 t.Setenv("WAYLAND_DISPLAY","")
 if nexusLaunchBrokerGlobal.broker!=nil {
  t.Skip("a shared desktop broker is already initialized by another test")
 }
 ready,explanation:=DeckDesktopLauncherStatus()
 if ready||explanation==""{t.Fatalf("unexpected desktop readiness: ready=%t, status=%q",ready,explanation)}
 if err:=QueueDeckLaunchAction("url","https://example.com/");err==nil{
  t.Fatal("desktop URL was queued without a connected graphical agent")
 }
 if err:=QueueDeckLaunchAction("application","firefox.desktop");err==nil{
  t.Fatal("application was queued without a connected graphical agent")
 }
}

func TestDesktopLaunchDoesNotUseInheritedServiceDisplay(t *testing.T) {
 cases:=[]struct{
  name string
  service bool
  uid int
  display,wayland string
  allowed bool
 }{
  {name:"service with inherited X11 display",service:true,uid:997,display:":0",allowed:false},
  {name:"service with inherited Wayland display",service:true,uid:997,wayland:"wayland-0",allowed:false},
  {name:"real graphical user",service:false,uid:1000,display:":0",allowed:true},
  {name:"real Wayland desktop",service:false,uid:1000,wayland:"wayland-0",allowed:true},
  {name:"user without desktop",service:false,uid:1000,allowed:false},
  {name:"root graphical process",service:false,uid:0,display:":0",allowed:false},
 }
 for _,tc:=range cases{
  t.Run(tc.name,func(t *testing.T){
   if got:=nexusDirectDesktopAllowed(tc.service,tc.uid,tc.display,tc.wayland);got!=tc.allowed{
    t.Fatalf("allowed=%v, want %v",got,tc.allowed)
   }
  })
 }
}

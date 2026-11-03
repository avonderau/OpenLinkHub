package nexus

import "testing"

func TestHardwareActionsAreAllowlisted(t *testing.T){
 for _,v:=range []string{"rgb-on","rgb-off"}{
  if !supportedNexusHardwareAction(v){t.Fatalf("missing supported action %s",v)}
 }
 for _,v:=range []string{"","reset","fan:100","../../","rgb-on;shutdown"}{
  if supportedNexusHardwareAction(v){t.Fatalf("accepted unsafe action %s",v)}
 }
}
func TestHardwareActionDispatch(t *testing.T){
 var called string
 RegisterHardwareActionHandler(func(action string){called=action})
 defer RegisterHardwareActionHandler(nil)
 if executeNexusHardwareAction("fan:100"){t.Fatal("unexpected dispatch")}
 if called!=""{t.Fatal("unsafe action reached handler")}
 if !executeNexusHardwareAction("rgb-off")||called!="rgb-off"{t.Fatal("expected RGB action")}
}
func TestHardwareScreenValidation(t *testing.T){
 for _,action:=range []string{"rgb-on","rgb-off"}{
  cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"main",Screens:[]Screen{{ID:"main",Name:"Main",Widgets:[]Widget{{ID:"rgb",Type:"button",Action:"device",Target:action,X:0,Y:0,Width:80,Height:40}}}}}
  if err:=cfg.Validate();err!=nil{t.Fatalf("valid action rejected: %v",err)}
 }
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"main",Screens:[]Screen{{ID:"main",Name:"Main",Widgets:[]Widget{{ID:"bad",Type:"button",Action:"device",Target:"fan:100",X:0,Y:0,Width:80,Height:40}}}}}
 if err:=cfg.Validate();err==nil{t.Fatal("accepted unrestricted device command")}
}

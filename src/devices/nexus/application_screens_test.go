package nexus

import (
 "testing"
 "time"
)

func TestNexusApplicationScreenOverridesAndRestores(t *testing.T){
 d:=&Device{customScreens:ScreenConfiguration{
  Version:1,ActiveScreenID:"main",CyclingMode:"automatic",CyclingSeconds:5,
  ApplicationScreens:map[string]string{"game":"gaming","editor":"work"},
  Screens:[]Screen{{ID:"main",Name:"Main"},{ID:"gaming",Name:"Gaming"},{ID:"work",Name:"Work"}},
 }}
 if err:=d.customScreens.Validate();err!=nil{t.Fatal(err)}
 d.SetNexusApplicationActive("unknown",true)
 if d.customScreens.ActiveScreenID!="main"{t.Fatal("unknown app changed screen")}
 d.SetNexusApplicationActive("game",true)
 if d.customScreens.ActiveScreenID!="gaming"{t.Fatal("game did not activate mapped screen")}
 d.advanceAutomaticScreen(time.Now().Add(30*time.Second))
 if d.customScreens.ActiveScreenID!="gaming"{t.Fatal("automatic cycling overrode application")}
 d.SetNexusApplicationActive("editor",true)
 if d.customScreens.ActiveScreenID!="work"{t.Fatal("newest application did not take precedence")}
 d.SetNexusApplicationActive("editor",false)
 if d.customScreens.ActiveScreenID!="gaming"{t.Fatal("previous active app not restored")}
 d.SetNexusApplicationActive("game",false)
 if d.customScreens.ActiveScreenID!="main"{t.Fatal("default screen not restored")}
}
func TestNexusApplicationBindingsValidation(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"main",Screens:[]Screen{{ID:"main",Name:"Main"}},ApplicationScreens:map[string]string{"my.game":"main"}}
 if err:=cfg.Validate();err!=nil{t.Fatal(err)}
 for _,mapping:=range []map[string]string{{"../unsafe":"main"},{"game":"absent"},{"":"main"}}{
  cfg.ApplicationScreens=mapping
  if err:=cfg.Validate();err==nil{t.Fatalf("invalid mapping accepted: %v",mapping)}
 }
}

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
 if DeckMacroMaxDuration().Seconds()>15{t.Fatal("macro limit exceeded")}
}

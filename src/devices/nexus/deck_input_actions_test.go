package nexus

import "testing"

func TestDeckInputValidationSharesNexusParser(t *testing.T){
 for _,ok:=range []string{"Ctrl+Alt+T","Super+D","Shift+F1","Ctrl+Shift+Esc"}{
  if err:=ValidateDeckShortcut(ok);err!=nil{t.Errorf("%s: %v",ok,err)}
 }
 for _,bad:=range []string{"","Ctrl+","Ctrl+Ctrl+T","sh -c echo","Ctrl+$(echo hi)"}{
  if err:=ValidateDeckShortcut(bad);err==nil{t.Errorf("accepted invalid shortcut: %q",bad)}
  if err:=ExecuteDeckShortcut(bad);err==nil{t.Errorf("executed invalid shortcut: %q",bad)}
 }
}
func TestDeckMediaActionAllowlist(t *testing.T){
 for _,action:=range []string{"play-pause","stop","previous-track","next-track","volume-up","volume-down","volume-mute"}{
  if !ValidDeckMediaAction(action){t.Errorf("missing action %s",action)}
 }
 for _,action:=range []string{"launch","volume-max","https://example.com","; echo hi",""}{
  if ValidDeckMediaAction(action){t.Errorf("accepted invalid media action %q",action)}
  if err:=ExecuteDeckMediaAction(action);err==nil{t.Errorf("executed invalid media action %q",action)}
 }
}

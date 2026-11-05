package nexus

import "testing"

func TestDeckMediaActionAllowlist(t *testing.T){
 for _,action:=range []string{"play-pause","stop","previous-track","next-track","volume-up","volume-down","volume-mute"}{
  if !ValidDeckMediaAction(action){t.Errorf("missing action %s",action)}
 }
 for _,action:=range []string{"launch","volume-max","https://example.com","; echo hi",""}{
  if ValidDeckMediaAction(action){t.Errorf("accepted invalid media action %q",action)}
  if err:=ExecuteDeckMediaAction(action);err==nil{t.Errorf("executed invalid media action %q",action)}
 }
}

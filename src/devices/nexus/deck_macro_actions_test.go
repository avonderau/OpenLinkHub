package nexus

import (
 "context"
 "testing"
)
func TestDeckSavedMacroIDValidation(t *testing.T){
 for _,id:=range []string{"1","42","999999999"}{
  if !ValidateDeckSavedMacroID(id){t.Errorf("valid macro ID rejected %s",id)}
 }
 for _,id:=range []string{"","0","0001","-1","../../bad","1;rm","1234567890"}{
  if ValidateDeckSavedMacroID(id){t.Errorf("invalid macro ID accepted %s",id)}
  if err:=RunDeckSavedMacro(context.Background(),id);err==nil{t.Errorf("invalid macro ID dispatched %s",id)}
 }
 if err:=RunDeckSavedMacro(nil,"1");err==nil{t.Fatal("missing cancellation context accepted")}
}

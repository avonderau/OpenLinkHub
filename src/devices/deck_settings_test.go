package devices

import (
 "encoding/json"
 "testing"

 "OpenLinkHub/src/devices/deck"
)

func TestSupportedDeviceSettingsIncludesDeckModels(t *testing.T){
 raw,err:=json.Marshal(GetSupportedDevices())
 if err!=nil{t.Fatal(err)}
 var rows []struct{
  ProductId uint16
  VendorId uint16
  DeviceKey string
  Family string
  Name string
  Enabled bool
 }
 if err:=json.Unmarshal(raw,&rows);err!=nil{t.Fatal(err)}
 byKey:=make(map[string]int)
 for _,row:=range rows{
  if row.DeviceKey==""{t.Fatalf("missing vendor-qualified device key for %q",row.Name)}
  byKey[row.DeviceKey]++
 }
 for _,m:=range deck.SupportedModels(){
  key:=deck.DeviceKey(m.VendorID,m.ProductID)
  if byKey[key]!=1{t.Errorf("expected one Settings row for %s, got %d",key,byKey[key])}
  matched:=false
  for _,row:=range rows{
   if row.DeviceKey!=key{continue}
   matched=true
   if row.Family!="deck" || row.ProductId!=m.ProductID || row.VendorId!=m.VendorID || row.Name!=m.Name{
    t.Errorf("invalid Settings row for %s: %+v",key,row)
   }
  }
  if !matched{t.Errorf("missing Settings row: %s",key)}
 }
 corsairCount:=0
 for _,row:=range rows {if row.Family=="corsair"{corsairCount++}}
 if corsairCount==0{t.Fatal("original Corsair Settings list lost")}
}

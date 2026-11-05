package deck

import "testing"

func TestLookupModelRequiresVendorAndProduct(t *testing.T) {
 if _,ok:=LookupModel(0x1b1c,0x6670);ok {t.Fatal("wrong vendor accepted")}
 if _,ok:=LookupModel(0x5548,0x0080);ok {t.Fatal("wrong product accepted")}
 cases:=[]struct{vid,pid uint16; keys int; family string}{
  {0x5548,0x6670,18,"mirabox"},
  {0x6603,0x1014,18,"mirabox"},
  {0x6603,0x1005,18,"mirabox"},
  {0x0fd9,0x0063,6,"elgato"},
  {0x0fd9,0x0080,15,"elgato"},
  {0x0fd9,0x008f,32,"elgato"},
 }
 for _,tc:=range cases {
  m,ok:=LookupModel(tc.vid,tc.pid)
  if !ok || m.Keys()!=tc.keys || m.Family!=tc.family {
   t.Errorf("model %04x:%04x = %+v, %v",tc.vid,tc.pid,m,ok)
  }
 }
}
func TestMiraboxHardwareCapabilities(t *testing.T) {
 old,_:=LookupModel(0x5548,0x6670)
 next,_:=LookupModel(0x6603,0x1014)
 if old.SupportsKeyRelease() {t.Fatal("v1 release incorrectly enabled")}
 if !next.SupportsKeyRelease(){t.Fatal("v3 release missing")}
 if old.NativeImagePixels(5)!=85 {t.Fatal("v1 image size")}
 if next.NativeImagePixels(5)!=82 || next.NativeImagePixels(4)!=95 {t.Fatal("v3 image size")}
}

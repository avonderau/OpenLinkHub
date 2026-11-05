package deck

import (
 "errors"
 "strings"
 "testing"
 "time"

 "github.com/sstallion/go-hid"
)

type fakeDeckSession struct {
 reports [][]byte
 closed bool
 failOutput bool
}

func(s *fakeDeckSession)Write(data []byte)(int,error){
 if s.failOutput{return 0,errors.New("simulated image write failure")}
 s.reports=append(s.reports,append([]byte(nil),data...))
 return len(data),nil
}
func(s *fakeDeckSession)ReadWithTimeout(_ []byte,_ time.Duration)(int,error){
 return 0,hid.ErrTimeout
}
func(s *fakeDeckSession)Close()error{s.closed=true;return nil}

func testDevice(t *testing.T)*Device{
 t.Helper()
 m,ok:=LookupModel(0x5548,0x6670)
 if !ok{t.Fatal("missing HSV293S model")}
 return &Device{
  Serial:"deck-5548-6670-test",Product:m.Name,Model:m,
  root:t.TempDir(),layout:DefaultLayout(m),done:make(chan struct{}),
  pressedKey:-1,
 }
}

func countMiraboxOperation(reports [][]byte,operation string)int{
 count:=0
 for _,report:=range reports{
  if len(report)>=9 && string(report[6:9])==operation && string(report[1:4])=="CRT"{count++}
 }
 return count
}

func TestDeviceAttachReplaysAllKeysAndFlushesOnce(t *testing.T){
 d:=testDevice(t)
 h:=new(fakeDeckSession)
 if err:=d.attachSession(h);err!=nil{t.Fatal(err)}
 status:=d.GetStatus()
 if !status.Connected || !status.KeyOutputSupported{t.Fatalf("not connected: %+v",status)}
 if count:=countMiraboxOperation(h.reports,"BAT");count!=d.Model.Keys(){
  t.Fatalf("expected %d key images, got %d",d.Model.Keys(),count)
 }
 if count:=countMiraboxOperation(h.reports,"STP");count!=1{
  t.Fatalf("expected one image commit, got %d",count)
 }
 if !strings.Contains(status.Status,"applied"){t.Errorf("unexpected status: %q",status.Status)}
 d.Stop()
 if !h.closed || d.GetStatus().Connected{t.Fatal("stop must close HID handle and mark disconnected")}
}

func TestOfflineLayoutQueuedForReconnect(t *testing.T){
 d:=testDevice(t)
 next:=DefaultLayout(d.Model)
 next.Pages[0].Keys=[]KeyConfig{{Index:0,Label:"CPU"}}
 result,err:=d.UpdateLayout(next)
 if err!=nil{t.Fatal(err)}
 if !result.Saved || result.Applied{t.Fatalf("offline result: %+v",result)}
 if !strings.Contains(result.Message,"reconnect"){t.Fatalf("missing offline message: %+v",result)}
 fromDisk,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil || len(fromDisk.Pages[0].Keys)!=1{t.Fatalf("not persisted: %+v %v",fromDisk,err)}
 h:=new(fakeDeckSession)
 if err:=d.attachSession(h);err!=nil{t.Fatal(err)}
 if countMiraboxOperation(h.reports,"BAT")!=d.Model.Keys(){t.Fatal("reconnect did not restore all key images")}
 result,err=d.UpdateLayout(next)
 if err!=nil||!result.Saved||!result.Applied{t.Fatalf("online result: %+v %v",result,err)}
 if countMiraboxOperation(h.reports,"STP")!=2{t.Fatal("each page update must flush exactly once")}
 d.Stop()
}

func TestDeviceDisplayFailureDoesNotDiscardSavedLayout(t *testing.T){
 d:=testDevice(t)
 h:=new(fakeDeckSession)
 if err:=d.attachSession(h);err!=nil{t.Fatal(err)}
 h.failOutput=true
 updated:=DefaultLayout(d.Model)
 updated.Pages[0].Keys=[]KeyConfig{{Index:3,Label:"Test"}}
 result,err:=d.UpdateLayout(updated)
 if err!=nil{t.Fatal(err)}
 if !result.Saved || result.Applied{t.Fatalf("display failure incorrectly reported: %+v",result)}
 if !strings.Contains(result.Message,"display update failed"){t.Fatal("missing failure warning")}
 got,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil||got.Pages[0].Keys[0].Label!="Test"{t.Fatalf("configuration lost: %+v %v",got,err)}
 d.Stop()
}

func TestReconnectCandidateFiltersIdentityAndHIDUsage(t *testing.T){
 d:=testDevice(t)
 good:=&hid.DeviceInfo{VendorID:0x5548,ProductID:0x6670,Path:"/dev/hidraw99",UsagePage:0xffa0,Usage:1}
 if !d.matchesReconnectCandidate(good){t.Fatal("valid replacement device rejected")}
 bad:=*good
 bad.UsagePage=0xff00
 if d.matchesReconnectCandidate(&bad){t.Fatal("different HID usage accepted")}
 bad=*good
 bad.VendorID=0x0fd9
 if d.matchesReconnectCandidate(&bad){t.Fatal("different vendor accepted")}
 bad=*good
 bad.Path=""
 if d.matchesReconnectCandidate(&bad){t.Fatal("empty path accepted")}
}

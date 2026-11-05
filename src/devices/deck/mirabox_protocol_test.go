package deck

import (
 "bytes"
 "errors"
 "image"
 "testing"
)

type reportRecorder struct{ reports [][]byte; short bool; fail bool }
func (r *reportRecorder)Write(b []byte)(int,error){
 if r.fail{return 0,errors.New("write failed")}
 r.reports=append(r.reports,append([]byte(nil),b...))
 if r.short{return len(b)-1,nil}
 return len(b),nil
}
func TestMiraboxReportLengthAndBrightness(t *testing.T){
 for _,id:=range []struct{vid,pid uint16;size int}{{0x5548,0x6670,513},{0x6603,0x1014,1025}}{
  m,_:=LookupModel(id.vid,id.pid)
  r:=new(reportRecorder)
  if err:=MiraboxInitialize(r,m);err!=nil{t.Fatal(err)}
  if err:=MiraboxBrightness(r,m,73);err!=nil{t.Fatal(err)}
  if len(r.reports)!=3{t.Fatalf("expected three reports, got %d",len(r.reports))}
  for _,report:=range r.reports{if len(report)!=id.size{t.Errorf("report length %d != %d",len(report),id.size)}}
  if !bytes.Equal(r.reports[2][:12],[]byte{0,0x43,0x52,0x54,0,0,'L','I','G',0,0,73}){t.Fatal("brightness command")}
  if err:=MiraboxBrightness(r,m,101);err==nil{t.Fatal("accepted illegal brightness")}
 }
}
func TestMiraboxImageTransferAndMapping(t *testing.T){
 m,_:=LookupModel(0x5548,0x6670)
 jpg,err:=MiraboxKeyImage(m,0,image.NewRGBA(image.Rect(0,0,85,85)))
 if err!=nil{t.Fatal(err)}
 r:=new(reportRecorder)
 if err:=MiraboxSetKeyJPEG(r,m,0,jpg);err!=nil{t.Fatal(err)}
 if len(r.reports)<3{t.Fatalf("expected command, body, commit: %d",len(r.reports))}
 if !bytes.Equal(r.reports[0][6:9],[]byte("BAT")){t.Fatal("missing BAT")}
 if r.reports[0][13]!=1{t.Fatal("first key should be device key 1")}
 if !bytes.Equal(r.reports[len(r.reports)-1][6:9],[]byte("STP")){t.Fatal("missing flush")}
 for _,report:=range r.reports{if len(report)!=513{t.Fatal("bad page size")}}
 if err:=MiraboxSetKeyJPEG(r,m,18,jpg);!errors.Is(err,errInvalidKey){t.Fatal("invalid index accepted")}
 if err:=MiraboxSetKeyJPEG(r,m,0,[]byte("not-jpg"));err==nil{t.Fatal("invalid jpeg accepted")}
}
func TestMiraboxShortWritesAndPressMapping(t *testing.T){
 m,_:=LookupModel(0x5548,0x6670)
 r:=&reportRecorder{short:true}
 if err:=MiraboxBrightness(r,m,55);err==nil{t.Fatal("short write must fail")}
 data:=make([]byte,512)
 copy(data,"ACK")
 data[9]=1
 key,pressed,ok:=MiraboxKeyEvent(m,data)
 if !ok || !pressed || key!=4{t.Fatalf("first raw key = %d %v %v",key,pressed,ok)}
 data[9]=0
 if _,_,ok:=MiraboxKeyEvent(m,data);ok{t.Fatal("accepted key zero")}
 next,_:=LookupModel(0x6603,0x1014)
 data[9]=1;data[10]=0
 _,pressed,ok=MiraboxKeyEvent(next,data)
 if !ok||pressed{t.Fatal("v3 release not recognized")}
 data[10]=1
 _,pressed,ok=MiraboxKeyEvent(next,data)
 if !ok||!pressed{t.Fatal("v3 press not recognized")}
}

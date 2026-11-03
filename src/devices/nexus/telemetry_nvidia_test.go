package nexus

import "testing"

func TestParseNvidiaMetrics(t *testing.T){
 result:=parseNvidiaMetrics("2048, 8192, 1650, 124.5\n")
 if result["gpu.vram.used_mb"]!=2048||result["gpu.vram.total_mb"]!=8192||result["gpu.vram.utilization"]!=25{
  t.Fatalf("bad VRAM results: %+v",result)
 }
 if result["gpu.clock_mhz"]!=1650||result["gpu.power_w"]!=124.5{t.Fatalf("bad clock or power: %+v",result)}
}

func TestNvidiaOptionalAndInvalidReadings(t *testing.T){
 cases:=[]struct{
  line string
  wantVRAM bool
  wantClock bool
  wantPower bool
 }{
  {"N/A, N/A, 1200, N/A",false,true,false},
  {"9999, 8192, 30000, 12000",false,false,false},
  {"0, 8192, N/A, 0",true,false,true},
  {"garbage",false,false,false},
  {"1200, 8192, 1500, 100",true,true,true},
 }
 for _,tc:=range cases{
  found:=parseNvidiaMetrics(tc.line)
  _,vram:=found["gpu.vram.utilization"]
  _,clock:=found["gpu.clock_mhz"]
  _,power:=found["gpu.power_w"]
  if vram!=tc.wantVRAM||clock!=tc.wantClock||power!=tc.wantPower{t.Errorf("%q: %+v",tc.line,found)}
 }
 for _,index:=range []int{-1,16,999}{
  if got:=ReadNvidiaMetrics(index);len(got)!=0{t.Fatalf("invalid index %d returned metrics",index)}
 }
}

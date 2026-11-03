package nexus

import (
 "math"
 "os"
 "path/filepath"
 "testing"
)

func writeKernelFixture(t *testing.T,path,contents string){
 t.Helper()
 if err:=os.MkdirAll(filepath.Dir(path),0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(path,[]byte(contents),0644);err!=nil{t.Fatal(err)}
}

func TestOptionalLinuxMetricsWithSysfsFixtures(t *testing.T){
 base:=t.TempDir()
 paths:=LinuxExtendedMetricPaths{
  CPUClock:filepath.Join(base,"cpu","scaling_cur_freq"),
  DiskRoot:base,
  Hwmon:filepath.Join(base,"hwmon"),
  DRM:filepath.Join(base,"drm"),
 }
 writeKernelFixture(t,paths.CPUClock,"3200000\n")
 writeKernelFixture(t,filepath.Join(paths.Hwmon,"hwmon0","fan1_input"),"1100\n")
 writeKernelFixture(t,filepath.Join(paths.Hwmon,"hwmon0","fan1_label"),"CPU Fan\n")
 writeKernelFixture(t,filepath.Join(paths.Hwmon,"hwmon1","fan1_input"),"2750\n")
 writeKernelFixture(t,filepath.Join(paths.Hwmon,"hwmon1","fan1_label"),"AIO Pump\n")
 gpu:=filepath.Join(paths.DRM,"card0","device")
 writeKernelFixture(t,filepath.Join(gpu,"mem_info_vram_total"),"4294967296\n")
 writeKernelFixture(t,filepath.Join(gpu,"mem_info_vram_used"),"1073741824\n")
 values:=ReadLinuxExtendedMetrics(paths)
 expected:=map[string]float64{
  "cpu.clock_mhz":3200,
  "fan.rpm":1100,
  "pump.rpm":2750,
  "gpu.vram.used_mb":1024,
  "gpu.vram.utilization":25,
 }
 for name,want:=range expected{
  value,ok:=values[name]
  if !ok||math.Abs(value-want)>0.01{t.Fatalf("%s expected %v got %v (available: %v)",name,want,value,values)}
 }
 if _,ok:=values["disk.utilization"];!ok{t.Fatal("root filesystem percent not recorded")}
 if _,ok:=values["disk.free_gb"];!ok{t.Fatal("root filesystem free space not recorded")}
}

func TestOptionalLinuxMetricsFailClosedOnMissingOrInvalidSensors(t *testing.T){
 root:=t.TempDir()
 paths:=LinuxExtendedMetricPaths{
  CPUClock:filepath.Join(root,"missing"),
  DiskRoot:filepath.Join(root,"missing-disk"),
  Hwmon:filepath.Join(root,"hwmon"),
  DRM:filepath.Join(root,"drm"),
 }
 fan:=filepath.Join(paths.Hwmon,"hwmon0","fan1_input")
 writeKernelFixture(t,fan,"invalid")
 vram:=filepath.Join(paths.DRM,"card0","device")
 writeKernelFixture(t,filepath.Join(vram,"mem_info_vram_total"),"100")
 writeKernelFixture(t,filepath.Join(vram,"mem_info_vram_used"),"200")
 readings:=ReadLinuxExtendedMetrics(paths)
 if len(readings)!=0{t.Fatalf("invalid sensors must be unavailable rather than fabricated: %v",readings)}
 writeKernelFixture(t,fan,"0")
 if _,ok:=ReadLinuxExtendedMetrics(paths)["fan.rpm"];ok{t.Fatal("zero RPM was reported")}
}

func TestReadKernelMetricBounds(t *testing.T){
 root:=t.TempDir()
 file:=filepath.Join(root,"metric")
 writeKernelFixture(t,file,"4294967296\n")
 if value,err:=readKernelUint(file);err!=nil||value!=4294967296{t.Fatalf("uint64 parse: %v %v",value,err)}
 writeKernelFixture(t,file,string(make([]byte,129)))
 if _,err:=readKernelUint(file);err==nil{t.Fatal("unbounded kernel text accepted")}
}

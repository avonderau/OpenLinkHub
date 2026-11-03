package nexus

import (
 "errors"
 "os"
 "path/filepath"
 "strconv"
 "strings"
 "syscall"
)

// LinuxExtendedMetricPaths permits fixture-driven tests. Production paths
// resolve exclusively to kernel-owned sysfs and the root filesystem.
type LinuxExtendedMetricPaths struct {
 CPUClock string
 DiskRoot string
 Hwmon string
 DRM string
}

func defaultLinuxExtendedMetricPaths()LinuxExtendedMetricPaths{
 return LinuxExtendedMetricPaths{
  CPUClock:"/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq",
  DiskRoot:"/",
  Hwmon:"/sys/class/hwmon",
  DRM:"/sys/class/drm",
 }
}

func readKernelUint(path string)(uint64,error){
 raw,err:=os.ReadFile(path)
 if err!=nil{return 0,err}
 if len(raw)>128{return 0,errors.New("unexpectedly large kernel metric")}
 return strconv.ParseUint(strings.TrimSpace(string(raw)),10,64)
}

func readLinuxFanMetrics(root string)MetricSnapshot{
 result:=MetricSnapshot{}
 entries,err:=filepath.Glob(filepath.Join(root,"hwmon*","fan*_input"))
 if err!=nil{return result}
 // Choose one actual, positive RPM, never sum unrelated devices. A labelled
 // pump remains separate from generic fan readings.
 for _,name:=range entries{
  rpm,err:=readKernelUint(name)
  if err!=nil||rpm==0||rpm>100000{continue}
  labelPath:=strings.TrimSuffix(name,"_input")+"_label"
  label,err:=os.ReadFile(labelPath)
  pump:=err==nil&&strings.Contains(strings.ToLower(string(label)),"pump")
  key:="fan.rpm"
  if pump{key="pump.rpm"}
  if _,seen:=result[key];!seen{result[key]=float64(rpm)}
  if len(result)==2{break}
 }
 return result
}

func readLinuxAMDGpuVRAM(root string)MetricSnapshot{
 result:=MetricSnapshot{}
 paths,err:=filepath.Glob(filepath.Join(root,"card[0-9]*","device","mem_info_vram_total"))
 if err!=nil{return result}
 for _,totalPath:=range paths{
  total,err:=readKernelUint(totalPath)
  if err!=nil||total==0{continue}
  used,err:=readKernelUint(filepath.Join(filepath.Dir(totalPath),"mem_info_vram_used"))
  if err!=nil||used>total{continue}
  result["gpu.vram.used_mb"]=float64(used)/(1024*1024)
  result["gpu.vram.utilization"]=100*float64(used)/float64(total)
  break
 }
 return result
}

// ReadLinuxExtendedMetrics reads inexpensive optional kernel metrics. Missing
// devices simply omit keys and render as "--"; no external processes run.
func ReadLinuxExtendedMetrics(paths LinuxExtendedMetricPaths) MetricSnapshot{
 readings:=MetricSnapshot{}
 if clock,err:=readKernelUint(paths.CPUClock);err==nil&&clock>=100000&&clock<=10000000{
  readings["cpu.clock_mhz"]=float64(clock)/1000 // scaling_cur_freq is kHz
 }
 if paths.DiskRoot!=""{
  var stat syscall.Statfs_t
  if err:=syscall.Statfs(paths.DiskRoot,&stat);err==nil&&stat.Blocks>0{
   used:=stat.Blocks-stat.Bfree
   readings["disk.utilization"]=100*float64(used)/float64(stat.Blocks)
   readings["disk.free_gb"]=float64(stat.Bavail)*float64(stat.Bsize)/1e9
  }
 }
 for key,value:=range readLinuxFanMetrics(paths.Hwmon){readings[key]=value}
 for key,value:=range readLinuxAMDGpuVRAM(paths.DRM){readings[key]=value}
 return readings
}

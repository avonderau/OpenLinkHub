package nexus

import (
 "OpenLinkHub/src/systeminfo"
 "OpenLinkHub/src/temperatures"
 "fmt"
 "os"
 "path/filepath"
 "strings"
)
func defaultCustomScreens() ScreenConfiguration {
 return ScreenConfiguration{Version:1,ActiveScreenID:"system",Screens:[]Screen{
 {ID:"system",Name:"System",Widgets:[]Widget{
 {ID:"cpu",Type:"metric",Metric:"cpu.temperature",X:75,Y:0,Width:150,Height:48,Settings:map[string]string{"label":"CPU","unit":" C"}},
 {ID:"gpu",Type:"metric",Metric:"gpu.temperature",X:230,Y:0,Width:150,Height:48,Settings:map[string]string{"label":"GPU","unit":" C"}},
 {ID:"ram",Type:"bar",Metric:"memory.utilization",X:385,Y:0,Width:175,Height:48,Settings:map[string]string{"label":"RAM","unit":"%"}},
 }},
 {ID:"network",Name:"Network",Widgets:[]Widget{
 {ID:"rx",Type:"metric",Metric:"network.rx_rate",X:75,Y:0,Width:235,Height:48,Settings:map[string]string{"label":"RX bytes/s"}},
 {ID:"tx",Type:"metric",Metric:"network.tx_rate",X:320,Y:0,Width:235,Height:48,Settings:map[string]string{"label":"TX bytes/s"}},
 }},
 {ID:"storage",Name:"Storage",Widgets:[]Widget{
 {ID:"read",Type:"metric",Metric:"disk.read_rate",X:75,Y:0,Width:235,Height:48,Settings:map[string]string{"label":"READ bytes/s"}},
 {ID:"write",Type:"metric",Metric:"disk.write_rate",X:320,Y:0,Width:235,Height:48,Settings:map[string]string{"label":"WRITE bytes/s"}},
 }},
 }}
}
func (d *Device) customScreenPath() string {
 return filepath.Join(pwd,"database","nexus",d.Serial+"-screens.json")
}
func (d *Device) loadCustomScreens() {
 c,err:=LoadScreens(d.customScreenPath())
 if err!=nil {c=defaultCustomScreens()}
 d.customScreens=c
 d.LCDModes["custom-screens"]="Custom Screens"
 if d.LCDProfiles!=nil {
  if d.LCDProfiles.Profiles==nil {d.LCDProfiles.Profiles=map[string]Profile{}}
  d.LCDProfiles.Profiles["custom-screens"]=Profile{Name:"Custom Screens"}
 }
}
func (d *Device) navigateCustomScreen(next bool) {
 d.mutex.Lock()
 defer d.mutex.Unlock()
 if len(d.customScreens.Screens)==0{return}
 if next {d.customScreens.ActiveScreenID=d.customScreens.NextScreen()} else {d.customScreens.ActiveScreenID=d.customScreens.PreviousScreen()}
 _=SaveScreens(d.customScreenPath(),d.customScreens)
}
func (d *Device) renderCustomScreen() []byte {
 d.mutex.Lock()
 c:=d.customScreens
 d.mutex.Unlock()
 if len(c.Screens)==0{return nil}
 values:=MetricSnapshot{
 "cpu.temperature":float64(temperatures.GetCpuTemperature()),
 "gpu.temperature":float64(temperatures.GetGpuTemperature()),
 "cpu.utilization":systeminfo.GetCpuUtilization(),
 "gpu.utilization":float64(systeminfo.GetGPUUtilization()),
 }
 if mem,err:=os.ReadFile("/proc/meminfo");err==nil {
  if used,pct,ok:=parseMemoryMetrics(string(mem));ok {
   values["memory.used"]=used
   values["memory.utilization"]=pct
  }
 }
 for _,screen:=range c.Screens {
  if screen.ID==c.ActiveScreenID {
   frame,err:=RenderScreen(screen,values)
   if err!=nil{return nil}
   return renderImageToBytes(frame)
  }
 }
 return nil
}
func parseMemoryMetrics(content string)(float64,float64,bool){
 var total,available uint64
 for _,line:=range strings.Split(content,"\n"){
  var key string;var value uint64
  if _,err:=fmt.Sscanf(line,"%s %d",&key,&value);err!=nil{continue}
  if key=="MemTotal:"{total=value}
  if key=="MemAvailable:"{available=value}
 }
 if total==0||available>total{return 0,0,false}
 return float64(total-available)*1024,100*float64(total-available)/float64(total),true
}

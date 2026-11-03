package nexus

import (
 "OpenLinkHub/src/inputmanager"
 "OpenLinkHub/src/systeminfo"
 "OpenLinkHub/src/temperatures"
 "fmt"
 "image"
 "os"
 "path/filepath"
 "strings"
 "time"
)
func defaultCustomScreens() ScreenConfiguration {
 // On a fresh device, use the same fully editable JSON screens offered in the editor.
 if stock,err:=stockStarterScreens();err==nil{return stock}
 return ScreenConfiguration{Version:1,ActiveScreenID:"system",CyclingMode:"manual",CyclingSeconds:15,Screens:[]Screen{
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
 if d.DeviceProfile!=nil && d.DeviceProfile.LCDMode!="custom-screens" {
  d.DeviceProfile.LCDMode="custom-screens"
  d.DeviceProfile.DynamicMode=false
  d.Keyboard=false
  d.saveDeviceProfile()
 }
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
 if d.customScreens.CyclingMode=="off" || d.customScreens.CyclingMode=="automatic" {return}
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
 if network,disk:=detectCounterDevices();network!=""&&disk!="" {
  if counters,err:=ReadLinuxCounters(network,disk);err==nil {
   d.mutex.Lock()
   rates:=d.customSampler.Sample(counters,time.Now())
   d.mutex.Unlock()
   if rates.Valid {
    values["network.rx_rate"]=rates.NetworkRXBytesPerSecond
    values["network.tx_rate"]=rates.NetworkTXBytesPerSecond
    values["disk.read_rate"]=rates.DiskReadBytesPerSecond
    values["disk.write_rate"]=rates.DiskWriteBytesPerSecond
   }
  }
 }
 // Record 60 one-second samples per supported metric for sparkline widgets.
 d.mutex.Lock()
 d.customSampler.history.add(values,time.Now())
 history:=d.customSampler.history.snapshot()
 d.mutex.Unlock()
 for _,screen:=range c.Screens {
  if screen.ID==c.ActiveScreenID {
   var background image.Image
   if screen.Background!="" {
    background=PresetNexusBackground(screen.Background,time.Now())
    if background==nil {background,_=ReadBackgroundAt(filepath.Join(pwd,"database","nexus","backgrounds"),screen.Background,time.Now())}
   }
   frame,err:=RenderScreenWithBackgroundAndIconsHistory(screen,values,background,filepath.Join(pwd,"database","nexus","icons"),history)
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

func (d *Device) GetCustomScreens() ScreenConfiguration {
 d.mutex.Lock()
 defer d.mutex.Unlock()
 return cloneScreenConfiguration(d.customScreens)
}
func cloneScreenConfiguration(c ScreenConfiguration) ScreenConfiguration {
 out:=c
 out.Screens=make([]Screen,len(c.Screens))
 for i,s:=range c.Screens {
  out.Screens[i]=s
  out.Screens[i].Widgets=make([]Widget,len(s.Widgets))
  for j,w:=range s.Widgets {
   out.Screens[i].Widgets[j]=w
   out.Screens[i].Widgets[j].Steps=append([]MacroStep(nil),w.Steps...)
   if w.Settings!=nil {
    out.Screens[i].Widgets[j].Settings=make(map[string]string,len(w.Settings))
    for k,v:=range w.Settings {out.Screens[i].Widgets[j].Settings[k]=v}
   }
  }
 }
 return out
}
func (d *Device) SetCustomScreens(c ScreenConfiguration) error {
 if err:=c.Validate();err!=nil{return err}
 d.mutex.Lock()
 defer d.mutex.Unlock()
 if err:=SaveScreens(d.customScreenPath(),c);err!=nil{return err}
 d.customScreens=cloneScreenConfiguration(c)
 return nil
}
func (d *Device) SaveCustomIcon(id string,content []byte)(string,error){
 return SaveCustomIcon(filepath.Join(pwd,"database","nexus","icons"),id,content)
}
func (d *Device) CustomIconPath(id string)(string,error){
 if !safeID.MatchString(id){return "",fmt.Errorf("invalid icon ID")}
 path:=filepath.Join(pwd,"database","nexus","icons",id+".png")
 if _,err:=ReadCustomIcon(filepath.Dir(path),id);err!=nil{return "",err}
 return path,nil
}
func (d *Device) CustomBackgroundPath(id string)(string,error){
 return backgroundFilePath(filepath.Join(pwd,"database","nexus","backgrounds"),id)
}
func (d *Device) SaveCustomBackground(id string,content []byte)(string,error){
 return SaveBackground(filepath.Join(pwd,"database","nexus","backgrounds"),id,content)
}

func (d *Device) advanceAutomaticScreen(now time.Time) {
 d.mutex.Lock()
 defer d.mutex.Unlock()
 c:=&d.customScreens
 if c.CyclingMode!="automatic" || len(c.Screens)<2 || c.CyclingSeconds<5 {d.lastCycle=now;return}
 if d.lastCycle.IsZero() {d.lastCycle=now;return}
 if now.Sub(d.lastCycle)<time.Duration(c.CyclingSeconds)*time.Second{return}
 c.ActiveScreenID=c.NextScreen()
 d.lastCycle=now
}

func (d *Device) handleCustomTouch(position uint16) {
 d.mutex.Lock()
 cfg:=cloneScreenConfiguration(d.customScreens)
 d.mutex.Unlock()
 for _,screen:=range cfg.Screens {
  if screen.ID!=cfg.ActiveScreenID {continue}
  // HID reports a horizontal touch position; select the widget at the
  // horizontal coordinate, preferring the last (topmost) widget.
  for i:=len(screen.Widgets)-1;i>=0;i-- {
   w:=screen.Widgets[i]
   if int(position)<w.X || int(position)>=w.X+w.Width {continue}
   if w.Type=="button" {if w.Action=="macro" {d.runTouchMacro(w.Steps)} else {d.executeTouchAction(w.Action,w.Target)};return}
   if w.Type=="macro" {d.runTouchMacro(w.Steps);return}
  }
  break
 }
 if cfg.CyclingMode=="manual" {
  if position<70 {d.navigateCustomScreen(false)}
  if position>570 {d.navigateCustomScreen(true)}
 }
}
func (d *Device) executeTouchAction(action,target string) {
 switch action {
 case "next":d.navigateScreenAction(true)
 case "previous":d.navigateScreenAction(false)
 case "screen":
  d.mutex.Lock()
  for _,s:=range d.customScreens.Screens {if s.ID==target {d.customScreens.ActiveScreenID=target;break}}
  d.lastCycle=time.Now()
  d.mutex.Unlock()
 case "hotkey":executeKeyChord(target)
 case "mouse":executeNexusMouseAction(target)
 case "media":
  codes:=map[string]uint16{"volume-up":1,"volume-down":2,"stop":4,"previous-track":5,"play-pause":6,"next-track":7}
  if code:=codes[target];code!=0 {inputmanager.InputControlKeyboard(code,false)}
 }
}
func (d *Device) navigateScreenAction(next bool) {
 d.mutex.Lock()
 defer d.mutex.Unlock()
 if len(d.customScreens.Screens)==0{return}
 if next {d.customScreens.ActiveScreenID=d.customScreens.NextScreen()}else{d.customScreens.ActiveScreenID=d.customScreens.PreviousScreen()}
 d.lastCycle=time.Now()
}
func (d *Device) runTouchMacro(steps []MacroStep) {
 d.mutex.Lock()
 if d.macroRunning {d.mutex.Unlock();return}
 d.macroRunning=true
 d.mutex.Unlock()
 go func(){
  defer func(){d.mutex.Lock();d.macroRunning=false;d.mutex.Unlock()}()
  runNexusMacroSteps(steps,d.lcdRefreshChan,d.executeTouchAction)
 }()
}

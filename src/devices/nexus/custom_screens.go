package nexus

import (
 "OpenLinkHub/src/config"
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
func (d *Device) collectCustomTelemetry(now time.Time) MetricSnapshot {
 values:=MetricSnapshot{
  "cpu.temperature":float64(temperatures.GetCpuTemperature()),
  "gpu.temperature":float64(temperatures.GetGpuTemperature()),
  "cpu.utilization":systeminfo.GetCpuUtilization(),
  "gpu.utilization":float64(systeminfo.GetGPUUtilization()),
 }
 if mem,err:=os.ReadFile("/proc/meminfo");err==nil{
  if used,pct,ok:=parseMemoryMetrics(string(mem));ok{
   values["memory.used"]=used
   values["memory.utilization"]=pct
  }
 }
 if network,disk:=detectCounterDevices();network!=""&&disk!=""{
  if counters,err:=ReadLinuxCounters(network,disk);err==nil{
   d.mutex.Lock()
   rates:=d.customSampler.Sample(counters,now)
   d.mutex.Unlock()
   if rates.Valid{
    values["network.rx_rate"]=rates.NetworkRXBytesPerSecond
    values["network.tx_rate"]=rates.NetworkTXBytesPerSecond
    values["disk.read_rate"]=rates.DiskReadBytesPerSecond
    values["disk.write_rate"]=rates.DiskWriteBytesPerSecond
   }
  }
 }
 for name,reading:=range ReadLinuxExtendedMetrics(defaultLinuxExtendedMetricPaths()){
  values[name]=reading
 }
 // NVIDIA's optional clock, power and VRAM data is not available through
 // the generic AMD DRM sysfs provider; sample a bounded driver query only
 // for a configured NVIDIA GPU, no faster than once per telemetry snapshot.
 if info:=systeminfo.GetInfo();info!=nil&&info.GPU!=nil{
  index:=config.GetConfig().DefaultNvidiaGPU
  if gpu,ok:=info.GPU[index];ok&&strings.Contains(strings.ToLower(gpu.Model),"nvidia"){
   for name,reading:=range ReadNvidiaMetrics(index){values[name]=reading}
  }
 }
 return values
}

func (d *Device) currentCustomTelemetry(now time.Time)(MetricSnapshot,MetricHistory){
 d.mutex.Lock()
 values,history,cached:=d.customSampler.Cached(now)
 d.mutex.Unlock()
 if cached{return values,history}
 // Read potentially expensive GPU and CPU metrics without holding d.mutex,
 // so HID touch processing and screen changes are never blocked by sampling.
 values=d.collectCustomTelemetry(now)
 d.mutex.Lock()
 history=d.customSampler.Store(now,values)
 d.mutex.Unlock()
 return values,history
}

func (d *Device) renderCustomScreen() []byte {
 d.mutex.Lock()
 c:=d.customScreens
 pressed:=""
 if time.Now().Before(d.pressedWidgetUntil){pressed=d.pressedWidgetID}else{d.pressedWidgetID=""}
 d.mutex.Unlock()
 if len(c.Screens)==0{return nil}
 now:=time.Now()
 values,history:=d.currentCustomTelemetry(now)
 for _,screen:=range c.Screens{
  if screen.ID!=c.ActiveScreenID{continue}
  var background image.Image
  if screen.Background!=""{
   background=PresetNexusBackground(screen.Background,now)
   if background==nil {
    background,_=ReadBackgroundAt(filepath.Join(pwd,"database","nexus","backgrounds"),screen.Background,now)
   }
  }
  frame,err:=RenderScreenWithBackgroundAndIconsHistoryPressed(screen,values,background,
   filepath.Join(pwd,"database","nexus","icons"),history,pressed)
  if err!=nil{return nil}
  applyNexusSoftwareBrightness(frame,c.Brightness)
  return renderImageToBytes(frame)
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
 if c.ApplicationScreens!=nil{
  out.ApplicationScreens=make(map[string]string,len(c.ApplicationScreens))
  for id,screen:=range c.ApplicationScreens{out.ApplicationScreens[id]=screen}
 }
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
 d.recomputeNexusApplicationScreenLocked()
 return nil
}
func (d *Device) SaveCustomIcon(id string,content []byte)(string,error){
 return SaveCustomIcon(filepath.Join(pwd,"database","nexus","icons"),id,content)
}
func (d *Device) CustomIconPath(id string)(string,error){
 if !safeID.MatchString(id){return "",fmt.Errorf("invalid icon ID")}
 dir:=filepath.Join(pwd,"database","nexus","icons")
 path,err:=customIconFilePath(dir,id)
 if err!=nil{return "",err}
 if _,err=ReadCustomIcon(dir,id);err!=nil{return "",err}
 return path,nil
}
func (d *Device) CustomBackgroundPath(id string)(string,error){
 return backgroundFilePath(filepath.Join(pwd,"database","nexus","backgrounds"),id)
}
func (d *Device) SaveCustomBackground(id string,content []byte)(string,error){
 return SaveBackground(filepath.Join(pwd,"database","nexus","backgrounds"),id,content)
}

// SetNexusApplicationActive is called by the existing appmonitor dispatcher
// for configured applications only; saved layout state is not modified.
func (d *Device) SetNexusApplicationActive(id string,active bool){
 if !safeNexusAppID.MatchString(id){return}
 d.mutex.Lock()
 defer d.mutex.Unlock()
 if _,mapped:=d.customScreens.ApplicationScreens[id];!mapped{return}
 if active{
  if len(d.activeNexusApps)==0{d.preApplicationScreen=d.customScreens.ActiveScreenID}
  for i,v:=range d.activeNexusApps {if v==id{d.activeNexusApps=append(d.activeNexusApps[:i],d.activeNexusApps[i+1:]...);break}}
  d.activeNexusApps=append(d.activeNexusApps,id)
 }else{
  for i,v:=range d.activeNexusApps {if v==id{d.activeNexusApps=append(d.activeNexusApps[:i],d.activeNexusApps[i+1:]...);break}}
 }
 d.recomputeNexusApplicationScreenLocked()
}
func (d *Device) recomputeNexusApplicationScreenLocked(){
 for i:=len(d.activeNexusApps)-1;i>=0;i--{
  target,ok:=d.customScreens.ApplicationScreens[d.activeNexusApps[i]]
  if !ok{continue}
  for _,screen:=range d.customScreens.Screens{
   if screen.ID==target{d.customScreens.ActiveScreenID=target;d.lastCycle=time.Now();return}
  }
 }
 if len(d.activeNexusApps)==0&&d.preApplicationScreen!=""{
  for _,screen:=range d.customScreens.Screens{
   if screen.ID==d.preApplicationScreen{d.customScreens.ActiveScreenID=d.preApplicationScreen;break}
  }
  d.preApplicationScreen=""
  d.lastCycle=time.Now()
 }
}
func (d *Device) advanceAutomaticScreen(now time.Time) {
 d.mutex.Lock()
 defer d.mutex.Unlock()
 c:=&d.customScreens
 if len(d.activeNexusApps)>0{d.lastCycle=now;return}
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
   if w.Type=="button" {
    d.mutex.Lock()
    d.pressedWidgetID=w.ID
    d.pressedWidgetUntil=time.Now().Add(260*time.Millisecond)
    if d.lcdTimer!=nil{d.lcdTimer.Reset(1*time.Millisecond)}
    d.mutex.Unlock()
    if w.Action=="macro" {d.runTouchMacro(w.Steps)} else {d.executeTouchAction(w.Action,w.Target)}
    return
   }
   if w.Type=="macro" {
    d.mutex.Lock()
    d.pressedWidgetID=w.ID
    d.pressedWidgetUntil=time.Now().Add(260*time.Millisecond)
    if d.lcdTimer!=nil{d.lcdTimer.Reset(1*time.Millisecond)}
    d.mutex.Unlock()
    d.runTouchMacro(w.Steps);return
   }
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
 case "text":executeNexusTextEntry(target)
 case "url","application":executeNexusLaunchAction(action,target)
 case "device": executeNexusHardwareAction(target)
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

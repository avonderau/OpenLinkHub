package nexus

import (
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
 "strconv"
 "strings"
)

const (
 ScreenWidth = 640
 ScreenHeight = 48
 MaxScreens = 256
 MaxWidgets = 64
 MaxTotalWidgets = 4096
 MaxScreenConfigBytes = 4*1024*1024
)
var safeNexusAppID=regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var supportedWidgets = map[string]bool{"metric":true,"text":true,"bar":true,"graph":true,"clock":true,"image":true,"button":true}
var supportedMetrics = map[string]bool{
 "cpu.temperature":true,"cpu.utilization":true,
 "gpu.temperature":true,"gpu.utilization":true,
 "memory.used":true,"memory.utilization":true,
 "network.rx_rate":true,"network.tx_rate":true,
 "disk.read_rate":true,"disk.write_rate":true,
 "cpu.clock_mhz":true,"cpu.load_1m":true,"cpu.load_5m":true,"cpu.load_15m":true,"disk.utilization":true,"disk.free_gb":true,"fan.rpm":true,"pump.rpm":true,"gpu.vram.used_mb":true,"gpu.vram.utilization":true,"gpu.vram.total_mb":true,"gpu.clock_mhz":true,"gpu.power_w":true,
}
type MacroStep struct {
 Type string `json:"type"`
 Value string `json:"value,omitempty"`
 DelayMs int `json:"delayMs,omitempty"`
}
type Widget struct {
 ID string `json:"id"`
 Type string `json:"type"`
 Metric string `json:"metric,omitempty"`
 X int `json:"x"`
 Y int `json:"y"`
 Width int `json:"width"`
 Height int `json:"height"`
 Settings map[string]string `json:"settings,omitempty"`
 Action string `json:"action,omitempty"`
 Target string `json:"target,omitempty"`
 Steps []MacroStep `json:"steps,omitempty"`
}
type Screen struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Widgets []Widget `json:"widgets"`
 Background string `json:"background,omitempty"`
}
type ScreenConfiguration struct {
 Version int `json:"version"`
 ActiveScreenID string `json:"activeScreenId"`
 Screens []Screen `json:"screens"`
 CyclingMode string `json:"cyclingMode,omitempty"`
 CyclingSeconds int `json:"cyclingSeconds,omitempty"`
 Brightness int `json:"brightness,omitempty"`
 ApplicationScreens map[string]string `json:"applicationScreens,omitempty"`
}
func (c ScreenConfiguration) Validate() error {
 if c.Version != 1 {return errors.New("unsupported screen configuration version")}
 if c.CyclingMode!="" && c.CyclingMode!="manual" && c.CyclingMode!="automatic" && c.CyclingMode!="off" {return errors.New("invalid cycling mode")}
 if c.CyclingMode=="automatic" && (c.CyclingSeconds<5 || c.CyclingSeconds>3600) {return errors.New("cycling interval must be 5 to 3600 seconds")}
 if c.Brightness!=0 && (c.Brightness<20 || c.Brightness>100){return errors.New("display brightness must be 20 to 100 percent")}
 if len(c.Screens)<1 || len(c.Screens)>MaxScreens {return errors.New("invalid screen count")}
 if len(c.ApplicationScreens)>64{return errors.New("too many application screen bindings")}
 seen:=map[string]bool{}
 totalWidgets:=0
 for _,s:=range c.Screens {
  totalWidgets+=len(s.Widgets)
  if totalWidgets>MaxTotalWidgets{return errors.New("too many widgets across screens")}
  if !safeID.MatchString(s.ID)||seen[s.ID] {return fmt.Errorf("invalid or duplicate screen ID: %q",s.ID)}
  seen[s.ID]=true
  if len(s.Name)==0||len(s.Name)>80||len(s.Widgets)>MaxWidgets {return fmt.Errorf("invalid screen %q",s.ID)}
  if s.Background!="" && (!safeID.MatchString(s.Background) || len(s.Background)>64) {return fmt.Errorf("invalid background ID for screen %q",s.ID)}
  widgets:=map[string]bool{}
  for _,w:=range s.Widgets {
   if !safeID.MatchString(w.ID)||widgets[w.ID] {return fmt.Errorf("invalid or duplicate widget ID: %q",w.ID)}
   widgets[w.ID]=true
   if !supportedWidgets[w.Type] {return fmt.Errorf("unsupported widget type: %q",w.Type)}
   if (w.Type=="metric"||w.Type=="bar"||w.Type=="graph")&&!supportedMetrics[w.Metric] {return fmt.Errorf("unsupported metric: %q",w.Metric)}
   if source:=w.Settings["metricSource"];source!="" {
    if (w.Type!="metric"&&w.Type!="bar"&&w.Type!="graph")||!ValidMetricSource(w.Metric,source){
     return fmt.Errorf("invalid source for metric %q",w.Metric)
    }
   }
   if w.X<0||w.Y<0||w.Width<=0||w.Height<=0||w.X>ScreenWidth-w.Width||w.Y>ScreenHeight-w.Height {return fmt.Errorf("widget %q outside display",w.ID)}
   if w.Type=="button" {
    if w.Action!="next" && w.Action!="previous" && w.Action!="screen" && w.Action!="media" && w.Action!="macro" && w.Action!="hotkey" && w.Action!="mouse" && w.Action!="url" && w.Action!="application" && w.Action!="device" && w.Action!="text" {return fmt.Errorf("invalid button action for %q",w.ID)}
    if w.Action=="screen" && !safeID.MatchString(w.Target) {return fmt.Errorf("invalid button target for %q",w.ID)}
    if w.Action=="media" && !supportedMediaAction(w.Target) {return fmt.Errorf("invalid media action for %q",w.ID)}
    if w.Action=="device" && !supportedNexusHardwareAction(w.Target) {return fmt.Errorf("invalid device action for %q",w.ID)}
    if w.Action=="hotkey" {if _,err:=parseKeyChord(w.Target);err!=nil{return fmt.Errorf("invalid shortcut for %q: %w",w.ID,err)}}
    if w.Action=="mouse" {if _,err:=parseNexusMouseAction(w.Target);err!=nil{return fmt.Errorf("invalid mouse action for %q: %w",w.ID,err)}}
    if w.Action=="text" {if _,err:=parseNexusTextEntry(w.Target);err!=nil{return fmt.Errorf("invalid text entry for %q: %w",w.ID,err)}}
    if w.Action=="url" || w.Action=="application" {
     if err:=validateNexusLaunchAction(w.Action,w.Target);err!=nil{return fmt.Errorf("invalid launcher for %q: %w",w.ID,err)}
    }
   }
   if len(w.Steps)!=0 {return fmt.Errorf("inline Nexus macros are no longer supported for widget %q",w.ID)}
   if w.Type=="button" && w.Action=="macro" {
    if !validSharedMacroID(w.Target){return fmt.Errorf("invalid shared macro ID for %q",w.ID)}
   }
   if w.Type=="button" || w.Type=="metric" || w.Type=="bar" || w.Type=="graph" || w.Type=="text" || w.Type=="clock" || w.Type=="image" {
    style:=w.Settings["style"]
    if style!="" && style!="solid" && style!="outline" && style!="flat" {return fmt.Errorf("invalid widget style %q",style)}
    for _,key:=range []string{"cornerRadius","opacity"}{
     val:=w.Settings[key];if val=="" {continue}
     number,err:=strconv.Atoi(val);maximum:=24;if key=="opacity"{maximum=100}
     if err!=nil||number<0||number>maximum{return fmt.Errorf("invalid %s for widget %q",key,w.ID)}
    }
    if direction:=w.Settings["gradientDirection"];direction!=""&&direction!="none"&&direction!="horizontal"&&direction!="vertical"{return fmt.Errorf("invalid widget gradient direction for %q",w.ID)}
    for _,key:=range []string{"backgroundColor","gradientColor","textColor","borderColor"}{
     val:=w.Settings[key]
     if val!="" && !validHexColor(val) {return fmt.Errorf("invalid %s for widget %q",key,w.ID)}
    }
   }
   if v:=w.Settings["customIcon"];v!="" {
    if w.Type!="button" && w.Type!="image"{return fmt.Errorf("custom icons are only supported on buttons and images")}
    if !safeID.MatchString(v){return fmt.Errorf("invalid custom icon ID for widget %q",w.ID)}
   }
   if w.Type=="clock" && !validNexusClockFormat(w.Settings["clockFormat"]) {return fmt.Errorf("invalid clock format for %q",w.ID)}
   if !validNexusFontSize(w.Settings["fontSize"]) || !validNexusTextAlign(w.Settings["textAlign"]) || !validNexusVerticalAlign(w.Settings["verticalAlign"]) || !validNexusTextShadow(w.Settings["textShadow"]) {return fmt.Errorf("invalid typography for widget %q",w.ID)}
   if len(w.Settings)>16 {return errors.New("too many widget settings")}
   for k,v:=range w.Settings {if len(k)>64||len(v)>256 {return errors.New("widget setting too long")}}
  }
 }
 for _,screen:=range c.Screens {for _,w:=range screen.Widgets {
  if w.Type=="button" && w.Action=="screen" && !seen[w.Target] {return fmt.Errorf("unknown button screen target %q",w.Target)}
 }}
 for appID,screenID:=range c.ApplicationScreens {
  if !safeNexusAppID.MatchString(appID)||!seen[screenID]{return fmt.Errorf("invalid application screen binding %q",appID)}
 }
 if !seen[c.ActiveScreenID] {return errors.New("active screen does not exist")}
 return nil
}
func (c ScreenConfiguration) NextScreen() string {
 for i,s:=range c.Screens {if s.ID==c.ActiveScreenID {return c.Screens[(i+1)%len(c.Screens)].ID}}
 return c.Screens[0].ID
}
func (c ScreenConfiguration) PreviousScreen() string {
 for i,s:=range c.Screens {if s.ID==c.ActiveScreenID {return c.Screens[(i+len(c.Screens)-1)%len(c.Screens)].ID}}
 return c.Screens[0].ID
}
func LoadScreens(path string)(ScreenConfiguration,error){
 var c ScreenConfiguration
 b,err:=os.ReadFile(path);if err!=nil{return c,err}
 if len(b)>MaxScreenConfigBytes{return c,errors.New("configuration too large")}
 if err=json.Unmarshal(b,&c);err!=nil{return c,err}
 return c,c.Validate()
}
func SaveScreens(path string,c ScreenConfiguration)error{
 if err:=c.Validate();err!=nil{return err}
 b,err:=json.MarshalIndent(c,"","  ");if err!=nil{return err}
 if len(b)>MaxScreenConfigBytes{return errors.New("configuration too large")}
 dir:=filepath.Dir(path)
 f,err:=os.CreateTemp(dir,".nexus-screens-*.tmp");if err!=nil{return err}
 defer os.Remove(f.Name())
 if err=f.Chmod(0600);err!=nil{f.Close();return err}
 if _,err=f.Write(b);err!=nil{f.Close();return err}
 if err=f.Sync();err!=nil{f.Close();return err}
 if err=f.Close();err!=nil{return err}
 return os.Rename(f.Name(),path)
}

func supportedMediaAction(action string) bool {
 switch action {case "volume-up","volume-down","play-pause","stop","previous-track","next-track":return true}
 return false
}

func validHexColor(v string)bool{
 if len(v)!=7 || v[0]!='#' {return false}
 return strings.IndexFunc(v[1:],func(r rune)bool{return !(r>='0'&&r<='9'||r>='a'&&r<='f'||r>='A'&&r<='F')})==-1
}

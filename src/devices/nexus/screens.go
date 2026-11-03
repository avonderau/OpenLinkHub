package nexus

import (
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
)

const (
 ScreenWidth = 640
 ScreenHeight = 48
 MaxScreens = 32
 MaxWidgets = 64
)
var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var supportedWidgets = map[string]bool{"metric":true,"text":true,"bar":true}
var supportedMetrics = map[string]bool{
 "cpu.temperature":true,"cpu.utilization":true,
 "gpu.temperature":true,"gpu.utilization":true,
 "memory.used":true,"memory.utilization":true,
 "network.rx_rate":true,"network.tx_rate":true,
 "disk.read_rate":true,"disk.write_rate":true,
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
}
func (c ScreenConfiguration) Validate() error {
 if c.Version != 1 {return errors.New("unsupported screen configuration version")}
 if c.CyclingMode!="" && c.CyclingMode!="manual" && c.CyclingMode!="automatic" && c.CyclingMode!="off" {return errors.New("invalid cycling mode")}
 if c.CyclingMode=="automatic" && (c.CyclingSeconds<5 || c.CyclingSeconds>3600) {return errors.New("cycling interval must be 5 to 3600 seconds")}
 if len(c.Screens)<1 || len(c.Screens)>MaxScreens {return errors.New("invalid screen count")}
 seen:=map[string]bool{}
 for _,s:=range c.Screens {
  if !safeID.MatchString(s.ID)||seen[s.ID] {return fmt.Errorf("invalid or duplicate screen ID: %q",s.ID)}
  seen[s.ID]=true
  if len(s.Name)==0||len(s.Name)>80||len(s.Widgets)>MaxWidgets {return fmt.Errorf("invalid screen %q",s.ID)}
  if s.Background!="" && (!safeID.MatchString(s.Background) || len(s.Background)>64) {return fmt.Errorf("invalid background ID for screen %q",s.ID)}
  widgets:=map[string]bool{}
  for _,w:=range s.Widgets {
   if !safeID.MatchString(w.ID)||widgets[w.ID] {return fmt.Errorf("invalid or duplicate widget ID: %q",w.ID)}
   widgets[w.ID]=true
   if !supportedWidgets[w.Type] {return fmt.Errorf("unsupported widget type: %q",w.Type)}
   if w.Type!="text"&&!supportedMetrics[w.Metric] {return fmt.Errorf("unsupported metric: %q",w.Metric)}
   if w.X<0||w.Y<0||w.Width<=0||w.Height<=0||w.X>ScreenWidth-w.Width||w.Y>ScreenHeight-w.Height {return fmt.Errorf("widget %q outside display",w.ID)}
   if len(w.Settings)>16 {return errors.New("too many widget settings")}
   for k,v:=range w.Settings {if len(k)>64||len(v)>256 {return errors.New("widget setting too long")}}
  }
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
 if len(b)>1024*1024{return c,errors.New("configuration too large")}
 if err=json.Unmarshal(b,&c);err!=nil{return c,err}
 return c,c.Validate()
}
func SaveScreens(path string,c ScreenConfiguration)error{
 if err:=c.Validate();err!=nil{return err}
 b,err:=json.MarshalIndent(c,"","  ");if err!=nil{return err}
 dir:=filepath.Dir(path)
 f,err:=os.CreateTemp(dir,".nexus-screens-*.tmp");if err!=nil{return err}
 defer os.Remove(f.Name())
 if err=f.Chmod(0600);err!=nil{f.Close();return err}
 if _,err=f.Write(b);err!=nil{f.Close();return err}
 if err=f.Sync();err!=nil{f.Close();return err}
 if err=f.Close();err!=nil{return err}
 return os.Rename(f.Name(),path)
}

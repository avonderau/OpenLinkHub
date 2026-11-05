package deck

import (
 "crypto/sha256"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"

 "OpenLinkHub/src/devices/nexus"
)

const MaxDeckPages=16
const MaxDeckLayoutBytes=256*1024
var safePageID=regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)
var deckMetricColorPattern=regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type KeyConfig struct {
 Index int `json:"index"`
 Label string `json:"label"`
 Action string `json:"action,omitempty"`
 TargetPage string `json:"targetPage,omitempty"`
 Target string `json:"target,omitempty"`
 Icon string `json:"icon,omitempty"`
 Metric string `json:"metric,omitempty"`
 RefreshSeconds int `json:"refreshSeconds,omitempty"`
 ShowGraph bool `json:"showGraph,omitempty"` // Kept for older saved layouts.
 MetricStyle string `json:"metricStyle,omitempty"`
 MetricColor string `json:"metricColor,omitempty"`
}
type Page struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Keys []KeyConfig `json:"keys"`
}
type Layout struct {
 Version int `json:"version"`
 ActivePage string `json:"activePage"`
 Pages []Page `json:"pages"`
}

func DefaultLayout(m Model)Layout {
 return Layout{Version:1,ActivePage:"home",Pages:[]Page{{ID:"home",Name:"Home",Keys:[]KeyConfig{}}}}
}

func (l Layout)Validate(m Model)error{
 if m.Keys()<=0 {return errors.New("unknown deck model")}
 if l.Version!=1 {return errors.New("unsupported layout version")}
 if len(l.Pages)<1||len(l.Pages)>MaxDeckPages{return errors.New("invalid page count")}
 ids:=make(map[string]bool,len(l.Pages))
 for _,page:=range l.Pages {
  if !safePageID.MatchString(page.ID)||ids[page.ID] {return fmt.Errorf("invalid or duplicate page ID: %q",page.ID)}
  ids[page.ID]=true
  if len(page.Name)<1||len(page.Name)>64{return errors.New("invalid page name")}
  if len(page.Keys)>m.Keys(){return errors.New("too many assigned keys")}
  seen:=make(map[int]bool,len(page.Keys))
  for _,key:=range page.Keys{
   if key.Index<0||key.Index>=m.Keys()||seen[key.Index] {return fmt.Errorf("invalid or duplicate key index: %d",key.Index)}
   seen[key.Index]=true
   if len(key.Label)>48 {return fmt.Errorf("key %d label too long",key.Index)}
   if key.Icon!="" && !deckIconID.MatchString(key.Icon) {
    return fmt.Errorf("key %d: invalid image reference",key.Index)
   }
   if key.Metric!="" {
    if key.MetricColor!="" && !deckMetricColorPattern.MatchString(key.MetricColor) {
     return fmt.Errorf("key %d: invalid metric visualization color",key.Index)
    }
    if key.MetricStyle!="" && key.MetricStyle!="value" && key.MetricStyle!="line" && key.MetricStyle!="bars" && key.MetricStyle!="fuel" && key.MetricStyle!="ring" {
     return fmt.Errorf("key %d: unsupported metric visualization %q",key.Index,key.MetricStyle)
    }
    if !nexus.IsSupportedMetric(key.Metric) {
     return fmt.Errorf("key %d: unsupported metric",key.Index)
    }
    if key.RefreshSeconds!=0 && (key.RefreshSeconds<2||key.RefreshSeconds>60) {
     return fmt.Errorf("key %d: refresh interval must be 2-60 seconds",key.Index)
    }
   } else if key.RefreshSeconds!=0||key.ShowGraph||key.MetricStyle!=""||key.MetricColor!="" {
    return fmt.Errorf("key %d: metric settings without metric",key.Index)
   }
   for _,char:=range key.Label{
    if char<32||char==127{return errors.New("key labels cannot contain control characters")}
   }
   switch key.Action {
   case "", "none", "page-next", "page-previous":
    if key.TargetPage!="" || key.Target!="" {return fmt.Errorf("key %d: unexpected action target for %q",key.Index,key.Action)}
   case "page-go":
    if !safePageID.MatchString(key.TargetPage) || key.Target!="" {return fmt.Errorf("key %d: invalid page navigation target",key.Index)}
   case "hotkey":
    if key.TargetPage!="" {return fmt.Errorf("key %d: unexpected hotkey page",key.Index)}
    if err:=nexus.ValidateDeckShortcut(key.Target);err!=nil{return fmt.Errorf("key %d: %w",key.Index,err)}
   case "media":
    if key.TargetPage!="" || !nexus.ValidDeckMediaAction(key.Target){
     return fmt.Errorf("key %d: invalid media action",key.Index)
    }
   case "macro":
    if key.TargetPage!="" || !nexus.ValidateDeckSavedMacroID(key.Target) {
     return fmt.Errorf("key %d: invalid saved macro ID",key.Index)
    }
   default:
    return fmt.Errorf("key %d: unsupported action %q",key.Index,key.Action)
   }
  }
 }
 for _,page:=range l.Pages{
  for _,key:=range page.Keys{
   if key.Action=="page-go" && !ids[key.TargetPage] {
    return fmt.Errorf("key %d: target page %q does not exist",key.Index,key.TargetPage)
   }
  }
 }
 if !ids[l.ActivePage]{return errors.New("active page does not exist")}
 return nil
}

func layoutFilename(root,id string)string{
 digest:=sha256.Sum256([]byte(id))
 return filepath.Join(root,"database","deck",fmt.Sprintf("%x.json",digest[:12]))
}

func LoadLayout(root,id string,m Model)(Layout,error){
 path:=layoutFilename(root,id)
 data,err:=os.ReadFile(path)
 if errors.Is(err,os.ErrNotExist){return DefaultLayout(m),nil}
 if err!=nil{return Layout{},err}
 if len(data)>MaxDeckLayoutBytes {return Layout{},errors.New("layout file too large")}
 var layout Layout
 if err:=json.Unmarshal(data,&layout);err!=nil{return Layout{},err}
 if err:=layout.Validate(m);err!=nil{return Layout{},err}
 return layout,nil
}

func SaveLayout(root,id string,m Model,layout Layout)error{
 if err:=layout.Validate(m);err!=nil{return err}
 content,err:=json.MarshalIndent(layout,"","  ")
 if err!=nil{return err}
 if len(content)>MaxDeckLayoutBytes{return errors.New("layout exceeds size limit")}
 path:=layoutFilename(root,id)
 if err:=os.MkdirAll(filepath.Dir(path),0750);err!=nil{return err}
 file,err:=os.CreateTemp(filepath.Dir(path),".deck-layout-*")
 if err!=nil{return err}
 defer os.Remove(file.Name())
 if err:=file.Chmod(0600);err!=nil{file.Close();return err}
 if _,err:=file.Write(content);err!=nil{file.Close();return err}
 if err:=file.Sync();err!=nil{file.Close();return err}
 if err:=file.Close();err!=nil{return err}
 return os.Rename(file.Name(),path)
}

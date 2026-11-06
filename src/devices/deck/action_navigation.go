package deck

import (
 "context"
 "fmt"
 "strconv"

 "OpenLinkHub/src/devices/nexus"
)

// keyDisplayLabel supplies readable default LCD text for built-in actions.
func keyDisplayLabel(key KeyConfig) string {
 if key.Label!="" {return key.Label}
 switch key.Action {
 case "page-next":return "Next Page"
 case "page-previous":return "Prev Page"
 case "page-go":return "Go to Page"
 case "folder-open":return "Folder"
 case "folder-back":return "Back"
 case "brightness-up":return "Brightness +"
 case "brightness-down":return "Brightness -"
 case "brightness-set":return "Brightness "+key.Target+"%"
 case "hotkey":return "Hotkey"
 case "macro":return "Run Macro"
 case "url":return "Open Website"
 case "application":return "Launch App"
 case "media":return nexus.DefaultMediaLabel(key.Target)
 }
 return ""
}

// Keyboard shortcuts and media keys use the existing bounded Nexus input backend.
func executeDeckInputAction(kind,target string)error{
 switch kind{
 case "hotkey":return nexus.ExecuteDeckShortcut(target)
 case "media":return nexus.ExecuteDeckMediaAction(target)
 default:return fmt.Errorf("unsupported Deck input action %q",kind)
 }
}

// handleKeyPress records every physical event, handles internal page changes,
// and dispatches keyboard/media/macro output AFTER releasing the device mutex.
// No external input handler can hold the HID layout lock.
func(d *Device)handleKeyPress(index int){
 action,target,run := d.selectKeyAction(index)
 if action==""{return}
 if action=="macro"{
  d.runDeckMacroAsync(target,run)
  return
 }
 if action=="url"||action=="application"{
  d.runDeckLaunchAsync(action,target,run)
  return
 }
 if run==nil{run=executeDeckInputAction}
 if err:=run(action,target);err!=nil{
  d.mu.Lock()
  d.status=fmt.Sprintf("Button action failed: %v",err)
  d.mu.Unlock()
 }
}

// A launch may block for up to four seconds while xdg-open/gtk-launch runs.
// Dispatch only from physical key presses and never hold the HID mutex during
// the desktop action. Avoid overlapping launches on the same Deck.
func (d *Device) runDeckLaunchAsync(action,target string,runner func(string,string)error){
 d.mu.Lock()
 if d.launchRunning{d.mu.Unlock();return}
 select{case <-d.done:d.mu.Unlock();return;default:}
 d.launchRunning=true
 d.mu.Unlock()
 go func(){
  defer func(){d.mu.Lock();d.launchRunning=false;d.mu.Unlock()}()
  select{case <-d.done:return;default:}
  if runner==nil{runner=nexus.ExecuteDeckLaunchAction}
  if err:=runner(action,target);err!=nil{
   d.mu.Lock()
   d.status=fmt.Sprintf("Desktop launch failed: %v",err)
   d.mu.Unlock()
  }
 }()
}

// A saved macro can run for up to 15 seconds. Never block the HID reader,
// allow concurrent macros from the same device, or leave a macro running
// after that device is stopped. The Nexus engine validates a fresh snapshot
// of the shared OpenLinkHub macro at execution time.
func(d *Device)runDeckMacroAsync(target string,runner func(string,string)error){
 d.mu.Lock()
 if d.macroRunning {
  d.mu.Unlock()
  return
 }
 select{case <-d.done:d.mu.Unlock();return;default:}
 d.macroRunning=true
 d.mu.Unlock()
 go func(){
  defer func(){
   d.mu.Lock()
   d.macroRunning=false
   d.mu.Unlock()
  }()
  ctx,cancel:=context.WithCancel(context.Background())
  defer cancel()
  go func(){
   select{case <-d.done:cancel();case <-ctx.Done():}
  }()
  var err error
  if runner!=nil{
   err=runner("macro",target) // injected test runner; real HID uses Nexus macro engine
  }else{
   err=nexus.RunDeckSavedMacro(ctx,target)
  }
  if err!=nil{
   d.mu.Lock()
   d.status=fmt.Sprintf("Saved macro failed: %v",err)
   d.mu.Unlock()
  }
 }()
}

// selectKeyAction holds d.mu while reading/changing layouts. An input action
// is only returned after its saved configuration has been validated.
func(d *Device)selectKeyAction(index int)(string,string,func(string,string)error){
 d.mu.Lock()
 defer d.mu.Unlock()
 // LCD positions 6/12/18 are passive on Mirabox hardware. They
 // must never dispatch commands even if stale in-memory data assigns one.
 if !d.Model.CanPress(index) {return "","",nil}
 d.eventCount++
 d.pressedKey=index
 current:=-1
 for i:=range d.layout.Pages{
  if d.layout.Pages[i].ID==d.layout.ActivePage {current=i;break}
 }
 if current<0{return "","",nil}
 var chosen *KeyConfig
 for i:=range d.layout.Pages[current].Keys{
  if d.layout.Pages[current].Keys[i].Index==index{
   chosen=&d.layout.Pages[current].Keys[i]
   break
  }
 }
 if chosen==nil{return "","",nil}
 if chosen.Action=="brightness-up"||chosen.Action=="brightness-down"||chosen.Action=="brightness-set" {
  if d.Model.Family!="mirabox" || d.hidDevice==nil || !d.connected {return "","",nil}
  if chosen.TargetPage!="" {return "","",nil}
  level:=d.layout.Brightness
  if level==0 {level=50}
  switch chosen.Action {
  case "brightness-up":
   if chosen.Target!=""{return "","",nil}
   level+=10
  case "brightness-down":
   if chosen.Target!=""{return "","",nil}
   level-=10
  case "brightness-set":
   requested,err:=strconv.Atoi(chosen.Target)
   if err!=nil || requested<10 || requested>100 {return "","",nil}
   level=requested
  }
  if level<10{level=10}
  if level>100{level=100}
  if err:=MiraboxBrightness(d.hidDevice,d.Model,uint8(level));err!=nil {
   d.status=fmt.Sprintf("Deck brightness failed: %v",err)
   return "","",nil
  }
  updated:=cloneLayout(d.layout)
  updated.Brightness=level
  if err:=SaveLayout(d.root,d.Serial,d.Model,updated);err!=nil{
   d.status=fmt.Sprintf("Brightness updated but save failed: %v",err)
   return "","",nil
  }
  d.layout=updated
  d.status=fmt.Sprintf("Deck brightness %d%%",level)
  return "","",nil
 }
 if chosen.Action=="hotkey"||chosen.Action=="media"||chosen.Action=="macro"||chosen.Action=="url"||chosen.Action=="application" {
  // Layouts are validated when loaded/saved. Revalidate the HID action
  // before dispatch, so malformed memory or stale settings cannot run.
  if chosen.TargetPage!="" {return "","",nil}
  switch chosen.Action {
  case "hotkey":
   if nexus.ValidateDeckShortcut(chosen.Target)!=nil {return "","",nil}
  case "media":
   if !nexus.ValidDeckMediaAction(chosen.Target){return "","",nil}
  case "macro":
   if !nexus.ValidateDeckSavedMacroID(chosen.Target){return "","",nil}
  case "url","application":
   if nexus.ValidateDeckLaunchAction(chosen.Action,chosen.Target)!=nil{return "","",nil}
  }
  return chosen.Action,chosen.Target,d.inputActionRunner
 }

 next:=current
 switch chosen.Action{
 case "page-next":next=(current+1)%len(d.layout.Pages)
 case "page-previous":next=(current+len(d.layout.Pages)-1)%len(d.layout.Pages)
 case "page-go":
  for i:=range d.layout.Pages{
   if d.layout.Pages[i].ID==chosen.TargetPage{next=i;break}
  }
 case "folder-open":
  for i:=range d.layout.Pages{
   if d.layout.Pages[i].ID==chosen.TargetPage && d.layout.Pages[i].ParentPage==d.layout.Pages[current].ID {
    next=i;break
   }
  }
 case "folder-back":
  if chosen.Target!="" || chosen.TargetPage!="" {return "","",nil}
  parent:=d.layout.Pages[current].ParentPage
  if parent=="" {return "","",nil}
  for i:=range d.layout.Pages{
   if d.layout.Pages[i].ID==parent{next=i;break}
  }
 default:return "","",nil
 }
 if next==current{return "","",nil}
 updated:=cloneLayout(d.layout)
 updated.ActivePage=updated.Pages[next].ID
 if err:=SaveLayout(d.root,d.Serial,d.Model,updated);err!=nil{
  d.status=fmt.Sprintf("Page navigation save failed: %v",err)
  return "","",nil
 }
 d.layout=updated
 if d.hidDevice!=nil && d.connected && d.Model.Family=="mirabox"{
  if err:=d.drawActivePageLocked();err!=nil{
   d.status=fmt.Sprintf("Page changed; LCD refresh failed: %v",err)
  }else{
   d.status="Connected; page "+updated.Pages[next].Name+" displayed"
  }
 }
 return "","",nil
}

package deck

import (
 "context"
 "fmt"

 "OpenLinkHub/src/devices/nexus"
)

// keyDisplayLabel supplies readable default LCD text for built-in actions.
func keyDisplayLabel(key KeyConfig) string {
 if key.Label!="" {return key.Label}
 switch key.Action {
 case "page-next":return "Next Page"
 case "page-previous":return "Prev Page"
 case "page-go":return "Go to Page"
 case "hotkey":return "Hotkey"
 case "url":return "Open URL"
 case "application":return "Launch App"
 case "macro":return "Run Macro"
 case "media":
  switch key.Target {
  case "play-pause":return "Play / Pause"
  case "previous-track":return "Previous"
  case "next-track":return "Next Track"
  case "volume-up":return "Volume +"
  case "volume-down":return "Volume -"
  case "volume-mute":return "Mute"
  case "stop":return "Stop"
  }
 }
 return ""
}

// executeDeckInputAction delegates to Nexus's allowlisted keyboard backend.
// This cannot spawn programs or execute arbitrary command strings.
func executeDeckInputAction(kind,target string)error{
 switch kind{
 case "hotkey":return nexus.ExecuteDeckShortcut(target)
 case "media":return nexus.ExecuteDeckMediaAction(target)
 case "url", "application":return nexus.QueueDeckLaunchAction(kind,target)
 default:return fmt.Errorf("unsupported Deck input action %q",kind)
 }
}

// handleKeyPress records every physical event, handles internal page changes,
// and dispatches keyboard/media output AFTER releasing the device mutex.
// No external input handler can hold the HID layout lock.
func(d *Device)handleKeyPress(index int){
 action,target,run := d.selectKeyAction(index)
 if action==""{return}
 if action=="macro"{
  d.runDeckMacroAsync(target,run)
  return
 }
 if run==nil{run=executeDeckInputAction}
 if err:=run(action,target);err!=nil{
  d.mu.Lock()
  d.status=fmt.Sprintf("Button action failed: %v",err)
  d.mu.Unlock()
 }
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
 d.eventCount++
 d.pressedKey=index

 if index<0||index>=d.Model.Keys(){return "","",nil}
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
 if chosen.Action=="hotkey"||chosen.Action=="media"||
   chosen.Action=="url"||chosen.Action=="application"||chosen.Action=="macro" {
  // Layouts are validated when loaded/saved. Revalidate the HID action
  // before dispatch, so malformed memory or stale settings cannot run.
  if chosen.TargetPage!="" {return "","",nil}
  switch chosen.Action {
  case "media":
   if !nexus.ValidDeckMediaAction(chosen.Target){return "","",nil}
  case "hotkey":
   if nexus.ValidateDeckShortcut(chosen.Target)!=nil{return "","",nil}
  case "url", "application":
   if nexus.ValidateDeckLaunchAction(chosen.Action,chosen.Target)!=nil{return "","",nil}
  case "macro":
   if !nexus.ValidateDeckSavedMacroID(chosen.Target){return "","",nil}
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

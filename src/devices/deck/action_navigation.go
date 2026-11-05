package deck

import (
 "fmt"

 "OpenLinkHub/src/logger"
)

// keyDisplayLabel supplies readable default LCD text for navigation actions.
func keyDisplayLabel(key KeyConfig)string{
 if key.Label!="" {return key.Label}
 switch key.Action {
 case "page-next":return "Next Page"
 case "page-previous":return "Prev Page"
 case "page-go":return "Go to Page"
 default:return ""
 }
}

// handleKeyPress dispatches the first safe, native Deck actions. Page actions
// run inside the device process and never start user programs as the
// privileged OpenLinkHub system service. Caller must not hold d.mu.
func(d *Device)handleKeyPress(index int){
 d.mu.Lock()
 defer d.mu.Unlock()
 d.eventCount++
 d.pressedKey=index

 if index<0 || index>=d.Model.Keys(){return}
 current:=-1
 for i:=range d.layout.Pages {
  if d.layout.Pages[i].ID==d.layout.ActivePage {current=i;break}
 }
 if current<0{return}

 var chosen *KeyConfig
 for i:=range d.layout.Pages[current].Keys{
  if d.layout.Pages[current].Keys[i].Index==index{
   chosen=&d.layout.Pages[current].Keys[i]
   break
  }
 }
 if chosen==nil{return}
 next:=current
 switch chosen.Action {
 case "page-next":
  next=(current+1)%len(d.layout.Pages)
 case "page-previous":
  next=(current+len(d.layout.Pages)-1)%len(d.layout.Pages)
 case "page-go":
  for i:=range d.layout.Pages {
   if d.layout.Pages[i].ID==chosen.TargetPage{next=i;break}
  }
 default:
  return
 }
 if next==current{return}

 // Persist the selected page before applying its images. Reconnection
 // restores the same page and unsuccessful disk writes do not change state.
 updated:=cloneLayout(d.layout)
 updated.ActivePage=updated.Pages[next].ID
 if err:=SaveLayout(d.root,d.Serial,d.Model,updated);err!=nil{
  d.status=fmt.Sprintf("Page navigation save failed: %v",err)
  logger.Log(logger.Fields{"serial":d.Serial,"error":err}).Warn("Deck page navigation persistence failed")
  return
 }
 d.layout=updated
 if d.hidDevice!=nil && d.connected && d.Model.Family=="mirabox"{
  if err:=d.drawActivePageLocked();err!=nil{
   d.status=fmt.Sprintf("Page changed; LCD refresh failed: %v",err)
   logger.Log(logger.Fields{"serial":d.Serial,"error":err}).Warn("Deck page navigation LCD update failed")
  }else{
   d.status="Connected; page "+updated.Pages[next].Name+" displayed"
  }
 }
}

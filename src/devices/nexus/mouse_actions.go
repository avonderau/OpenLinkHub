package nexus

import (
 "fmt"
 "OpenLinkHub/src/inputmanager"
)

// A mouse action is a single, bounded virtual-mouse command. No cursor
// teleportation, arbitrary movement scripts or unbounded repeats are allowed.
type nexusMouseAction struct {
 button uint16
 scroll int // vertical: -1 down, 1 up; horizontal: -2 right, 2 left
 count int
}

func parseNexusMouseAction(value string)(nexusMouseAction,error){
 switch value {
 case "left":return nexusMouseAction{button:inputmanager.BtnLeft,count:1},nil
 case "double-left":return nexusMouseAction{button:inputmanager.BtnLeft,count:2},nil
 case "right":return nexusMouseAction{button:inputmanager.BtnRight,count:1},nil
 case "middle":return nexusMouseAction{button:inputmanager.BtnMiddle,count:1},nil
 case "back":return nexusMouseAction{button:inputmanager.BtnBack,count:1},nil
 case "forward":return nexusMouseAction{button:inputmanager.BtnForward,count:1},nil
 case "scroll-up":return nexusMouseAction{scroll:1,count:1},nil
 case "scroll-down":return nexusMouseAction{scroll:-1,count:1},nil
 case "scroll-left":return nexusMouseAction{scroll:2,count:1},nil
 case "scroll-right":return nexusMouseAction{scroll:-2,count:1},nil
 default:return nexusMouseAction{},fmt.Errorf("unsupported mouse action %q",value)
 }
}

func executeNexusMouseAction(value string){
 action,err:=parseNexusMouseAction(value)
 if err!=nil{return}
 for i:=0;i<action.count;i++{
  if action.button!=0{inputmanager.InputControlMouse(action.button);continue}
  switch action.scroll {
  case 1:inputmanager.InputControlScroll(true)
  case -1:inputmanager.InputControlScroll(false)
  case 2:inputmanager.InputControlScrollHorizontal(true)
  case -2:inputmanager.InputControlScrollHorizontal(false)
  }
 }
}

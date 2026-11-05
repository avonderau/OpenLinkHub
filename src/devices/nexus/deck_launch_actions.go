package nexus

import (
 "errors"

 "OpenLinkHub/src/logger"
)

// ValidateDeckLaunchAction uses the exact same URL / desktop-entry rules
// as the Nexus touchscreen. Arbitrary paths and shell commands are forbidden.
func ValidateDeckLaunchAction(action,target string)error{
 return validateNexusLaunchAction(action,target)
}

// InitializeDeckDesktopLauncher makes the existing opt-in desktop agent
// available when the host has a Deck but no Corsair Nexus attached.
// The global broker is initialized at most once across both device types.
func InitializeDeckDesktopLauncher(root string)error{
 return initializeNexusDesktopLaunchBroker(root)
}

// QueueDeckLaunchAction sends a validated action to the *local* graphical
// desktop or opt-in Unix socket agent, never to a remote WebUI browser.
// Nonblocking, with a bounded queue shared with the Nexus touchscreen.
func QueueDeckLaunchAction(action,target string)error{
 if err:=validateNexusLaunchAction(action,target);err!=nil{return err}
 select{
 case launchSlot<-struct{}{}:
 default:return errors.New("desktop launcher busy")
 }
 go func(){
  defer func(){<-launchSlot}()
  if err:=dispatchNexusLaunchAction(action,target);err!=nil{
   // Never log private URL or application target values.
   logger.Log(logger.Fields{"action":action,"error":err.Error()}).Warn("Deck desktop launch failed")
  }
 }()
 return nil
}

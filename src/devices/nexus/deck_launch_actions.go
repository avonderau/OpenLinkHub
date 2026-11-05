package nexus

import (
 "errors"
 "os"

 "OpenLinkHub/src/config"
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

// DeckDesktopLauncherStatus reports whether this daemon can hand desktop
// actions to its local user session. A separate agent is required for a
// headless or service-account daemon; the web browser cannot substitute.
func DeckDesktopLauncherStatus()(bool,string){
 if nexusDirectDesktopAllowed(config.IsSystemService(),os.Geteuid(),os.Getenv("DISPLAY"),os.Getenv("WAYLAND_DISPLAY")){
  return true,"Direct local graphical session available"
 }
 broker:=nexusLaunchBrokerGlobal.broker
 if broker==nil{
  return false,"Desktop launcher unavailable: check OpenLinkHub service logs"
 }
 broker.mu.Lock()
 connected:=broker.client!=nil
 broker.mu.Unlock()
 if !connected{
  return false,"Desktop agent disconnected. Run OpenLinkHub --nexus-desktop-agent as your logged-in desktop user."
 }
 return true,"Local desktop agent connected"
}

// QueueDeckLaunchAction sends a validated action to the *local* graphical
// desktop or opt-in Unix socket agent, never to a remote WebUI browser.
// Nonblocking, with a bounded queue shared with the Nexus touchscreen.
func QueueDeckLaunchAction(action,target string)error{
 if err:=validateNexusLaunchAction(action,target);err!=nil{return err}
 if ready,message:=DeckDesktopLauncherStatus();!ready{return errors.New(message)}
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

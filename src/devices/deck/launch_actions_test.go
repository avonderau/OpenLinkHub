package deck

import (
 "errors"
 "strings"
 "testing"
 "time"
)

func TestDeckLaunchTargetsValidateAndPersist(t *testing.T){
 device:=testDevice(t)
 valid:=[]KeyConfig{
  {Index:0,Action:"url",Target:"https://example.com/project?q=1"},
  {Index:1,Action:"application",Target:"org.gnome.Terminal.desktop"},
 }
 layout:=DefaultLayout(device.Model)
 layout.Pages[0].Keys=valid
 if err:=layout.Validate(device.Model);err!=nil{t.Fatalf("valid launch configuration rejected: %v",err)}
 if err:=SaveLayout(device.root,device.Serial,device.Model,layout);err!=nil{t.Fatal(err)}
 loaded,err:=LoadLayout(device.root,device.Serial,device.Model)
 if err!=nil{t.Fatal(err)}
 if len(loaded.Pages[0].Keys)!=2||loaded.Pages[0].Keys[0].Target!=valid[0].Target||loaded.Pages[0].Keys[1].Target!=valid[1].Target{
  t.Fatalf("launch actions not persisted: %+v",loaded.Pages[0].Keys)
 }
}

func TestDeckLaunchTargetsRejectShellAndUntrustedSchemes(t *testing.T){
 device:=testDevice(t)
 invalid:=[]KeyConfig{
  {Index:0,Action:"url",Target:"file:///etc/passwd"},
  {Index:0,Action:"url",Target:"javascript:alert(1)"},
  {Index:0,Action:"url",Target:"https://user:password@example.com"},
  {Index:0,Action:"url",Target:"https://example.com",TargetPage:"home"},
  {Index:0,Action:"url",Target:""},
  {Index:0,Action:"application",Target:"../bin/sh"},
  {Index:0,Action:"application",Target:"/usr/bin/firefox"},
  {Index:0,Action:"application",Target:"firefox.desktop; echo bad"},
  {Index:0,Action:"application",Target:""},
  {Index:0,Action:"application",Target:"firefox.desktop",TargetPage:"home"},
 }
 for _,key:=range invalid{
  layout:=DefaultLayout(device.Model)
  layout.Pages[0].Keys=[]KeyConfig{key}
  if err:=layout.Validate(device.Model);err==nil{t.Fatalf("malicious launcher accepted: %+v",key)}
 }
}

func TestPhysicalDeckLaunchIsAsynchronousAndValidated(t *testing.T){
 device:=testDevice(t)
 device.layout.Pages[0].Keys=[]KeyConfig{
  {Index:0,Action:"url",Target:"https://example.com"},
  {Index:1,Action:"application",Target:"firefox.desktop"},
 }
 entered:=make(chan string,2)
 release:=make(chan struct{})
 device.inputActionRunner=func(kind,target string)error{
  // The desktop action runs outside the Deck's HID mutex.
  _=device.GetStatus()
  entered<-kind+":"+target
  <-release
  return nil
 }
 device.handleKeyPress(0)
 select{
 case got:=<-entered:
  if got!="url:https://example.com"{t.Fatalf("unexpected physical launch: %s",got)}
 case <-time.After(time.Second):t.Fatal("desktop launch never dispatched")
 }
 // The second key must be ignored while a launch is still pending;
 // the HID listener must not wait for the first desktop process.
 start:=time.Now()
 device.handleKeyPress(1)
 if time.Since(start)>300*time.Millisecond{t.Fatal("physical HID key blocked on desktop launch")}
 select{
 case got:=<-entered:t.Fatalf("overlapping launch started: %s",got)
 case <-time.After(20*time.Millisecond):
 }
 close(release)
 deadline:=time.After(time.Second)
 for{
  device.mu.RLock();running:=device.launchRunning;device.mu.RUnlock()
  if !running{break}
  select{case <-deadline:t.Fatal("launch worker did not finish");default:time.Sleep(time.Millisecond)}
 }
 device.handleKeyPress(1)
 select{
 case got:=<-entered:
  if got!="application:firefox.desktop"{t.Fatalf("unexpected desktop app launch: %s",got)}
 case <-time.After(time.Second):t.Fatal("desktop app launch never dispatched")
 }
}

func TestMalformedDeckLauncherCannotReachBackend(t *testing.T){
 device:=testDevice(t)
 device.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"url",Target:"file:///etc/passwd"}}
 entered:=make(chan struct{},1)
 device.inputActionRunner=func(string,string)error{entered<-struct{}{};return nil}
 device.handleKeyPress(0)
 select{
 case <-entered:t.Fatal("malformed layout reached launch backend")
 case <-time.After(30*time.Millisecond):
 }
}

func TestDeckLauncherFailureReportedWithoutBlocking(t *testing.T){
 device:=testDevice(t)
 device.layout.Pages[0].Keys=[]KeyConfig{{Index:0,Action:"url",Target:"https://example.com"}}
 started:=make(chan struct{},1)
 device.inputActionRunner=func(string,string)error{
  started<-struct{}{}
  return errors.New("no desktop agent connected")
 }
 device.handleKeyPress(0)
 select{case <-started:case <-time.After(time.Second):t.Fatal("launcher did not run")}
 deadline:=time.After(time.Second)
 for {
  if strings.Contains(device.GetStatus().Status,"Desktop launch failed"){break}
  select{case <-deadline:t.Fatal("launcher failure missing from status");default:time.Sleep(time.Millisecond)}
 }
}

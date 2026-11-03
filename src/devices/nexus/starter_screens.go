package nexus

import (
 "fmt"
 "path/filepath"
)

// stockStarterScreenNames lists example layouts shipped with OpenLinkHub.
// The JSON files have the same format as the user's saved screen configuration.
var stockStarterScreenNames=[]string{
 "system-overview",
 "performance-bars",
 "network-and-storage",
 "media-deck",
 "navigation",
}

// loadStockStarterScreens reads editable example screens without introducing
// any special template-only model or persistent template state.
func loadStockStarterScreens(dir string)(ScreenConfiguration,error){
 c:=ScreenConfiguration{Version:1,CyclingMode:"manual",CyclingSeconds:15}
 for _,name:=range stockStarterScreenNames{
  sample,err:=LoadScreens(filepath.Join(dir,name+".json"))
  if err!=nil{return c,fmt.Errorf("starter screen %s: %w",name,err)}
  if len(sample.Screens)!=1{return c,fmt.Errorf("starter screen %s must contain one screen",name)}
  c.Screens=append(c.Screens,sample.Screens[0])
 }
 c.ActiveScreenID=c.Screens[0].ID
 if err:=c.Validate();err!=nil{return c,fmt.Errorf("invalid stock screens: %w",err)}
 return c,nil
}

// stockStarterScreens is only used for the first layout on a device.
// An existing saved configuration is always authoritative.
func stockStarterScreens()(ScreenConfiguration,error){
 dirs:=[]string{
  filepath.Join(pwd,"static","nexus","templates"),
  filepath.Join("static","nexus","templates"),
  filepath.Join("..","..","..","static","nexus","templates"),
 }
 var err error
 for _,dir:=range dirs{
  var c ScreenConfiguration
  c,err=loadStockStarterScreens(dir)
  if err==nil{return c,nil}
 }
 return ScreenConfiguration{},err
}

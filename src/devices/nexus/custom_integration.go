package nexus

import (
 "encoding/binary"
 "time"
 "path/filepath"
)

// Custom Nexus hooks live separately from the upstream device implementation.
// Keep calls from nexus.go minimal so future upstream changes are easier to merge.
func (d *Device) refreshCustomLCD() {
 d.advanceAutomaticScreen(time.Now())
 // Animate uploaded GIFs at a bounded 5 FPS; preserve the upstream 1 Hz
 // update rate for every other screen and all other LCD modes.
 d.mutex.Lock()
 activeBackground:=""
 var activeWidgets []Widget
 for _,screen:=range d.customScreens.Screens{
  if screen.ID==d.customScreens.ActiveScreenID{
   activeBackground=screen.Background
   activeWidgets=screen.Widgets
   break
  }
 }
 d.mutex.Unlock()
 d.mutex.Lock()
 feedbackActive:=d.pressedWidgetID!=""&&time.Now().Before(d.pressedWidgetUntil)
 d.mutex.Unlock()
 animated:=IsAnimatedBackground(filepath.Join(pwd,"database","nexus","backgrounds"),activeBackground)
 if !animated{
  dir:=filepath.Join(pwd,"database","nexus","icons")
  for _,widget:=range activeWidgets{
   id:=widget.Settings["customIcon"]
   if id!=""&&IsAnimatedCustomIcon(dir,id){animated=true;break}
  }
 }
 refresh:=time.Duration(lcdRefreshInterval)*time.Millisecond
 if animated{refresh=200*time.Millisecond}
 if feedbackActive{refresh=100*time.Millisecond}
 d.lcdTimer.Reset(refresh)
 if frame:=d.renderCustomScreen();frame!=nil {d.transfer(frame)}
}

// All touchscreen actions are dispatched after release. Executing buttons
// immediately on touch-down prevents swiping across crowded control panels
// (especially media, macro, and shortcut screens).

// HID provides a single horizontal position and active flag; move reports
// update a per-listener gesture state without spawning goroutines.
func (d *Device) processCustomTouchReport(data []byte,state *touchGestureState){
 if len(data)<6||state==nil{return}
 active:=data[5]
 if active==0 {
  tap,swipe,shouldTap:=state.finish(time.Now())
  if swipe!=0 {
   d.mutex.Lock()
   manual:=d.customScreens.CyclingMode=="manual"
   d.mutex.Unlock()
   if manual{d.navigateCustomScreen(swipe>0)}
  }else if shouldTap {
   d.handleCustomTouch(tap)
  }
  return
 }
 if active!=1||len(data)<8{return}
 position:=binary.LittleEndian.Uint16(data[6:8])
 if position>=ScreenWidth{return}
 if !state.pressed{
  state.begin(position,time.Now())
  return
 }
 state.update(position)
}

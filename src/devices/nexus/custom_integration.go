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
 for _,screen:=range d.customScreens.Screens{
  if screen.ID==d.customScreens.ActiveScreenID{activeBackground=screen.Background;break}
 }
 d.mutex.Unlock()
 refresh:=time.Duration(lcdRefreshInterval)*time.Millisecond
 if IsAnimatedBackground(filepath.Join(pwd,"database","nexus","backgrounds"),activeBackground){
  refresh=200*time.Millisecond
 }
 d.lcdTimer.Reset(refresh)
 if frame:=d.renderCustomScreen();frame!=nil {d.transfer(frame)}
}

// Touch coordinates are reported horizontally by the Nexus HID interface.
// The caller retains its existing press/release debounce state.
func (d *Device) processCustomTouchReport(data []byte,blocked *bool) {
 if len(data)<8{return}
 active:=data[5]
 if active==0 {*blocked=false;return}
 if active!=1||*blocked{return}
 *blocked=true
 position:=binary.LittleEndian.Uint16(data[6:8])
 d.handleCustomTouch(position)
}

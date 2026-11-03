package nexus

import (
 "encoding/binary"
 "time"
)

// Custom Nexus hooks live separately from the upstream device implementation.
// Keep calls from nexus.go minimal so future upstream changes are easier to merge.
func (d *Device) refreshCustomLCD() {
 d.advanceAutomaticScreen(time.Now())
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

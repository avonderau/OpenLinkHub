package nexus

import "time"

const (
 nexusSwipeDistance=96 // pixels out of the physical 640-pixel width
 nexusTapTimeout=1200*time.Millisecond
)

// touchGestureState belongs to one HID listener, not the entire process.
// Keeping it per listener prevents taps and swipes from different devices
// from interfering with one another.
type touchGestureState struct {
 pressed bool
 start uint16
 last uint16
 since time.Time
}
func (s *touchGestureState) begin(position uint16,now time.Time){
 *s=touchGestureState{
  pressed:true,
  start:position,
  last:position,
  since:now,
 }
}
func (s *touchGestureState) update(position uint16){
 if s.pressed{s.last=position}
}

// finish classifies swipes before taps. A swipe can follow a long hold,
// but a stationary long hold is cancelled rather than activating a button.
// No action may fire from a touch-down event on any custom screen.
func (s *touchGestureState) finish(now time.Time)(tap uint16,swipe int,shouldTap bool){
 if !s.pressed{return 0,0,false}
 start,last,since:=s.start,s.last,s.since
 *s=touchGestureState{}
 delta:=int(last)-int(start)
 if delta<=-nexusSwipeDistance{return 0,1,false}
 if delta>=nexusSwipeDistance{return 0,-1,false}
 elapsed:=now.Sub(since)
 if elapsed<0||elapsed>nexusTapTimeout{return 0,0,false}
 return start,0,true
}

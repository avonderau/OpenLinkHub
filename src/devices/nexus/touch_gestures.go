package nexus

import "time"

const (
 nexusSwipeDistance=96 // pixels out of the physical 640-pixel width
 nexusSwipeTimeout=1200*time.Millisecond
)

// touchGestureState belongs to one HID listener, not the entire process.
// Keeping it per listener prevents taps and swipes from different devices
// from interfering with one another.
type touchGestureState struct {
 pressed bool
 handled bool
 start uint16
 last uint16
 since time.Time
}
func (s *touchGestureState) begin(position uint16,now time.Time,handled bool){
 *s=touchGestureState{
  pressed:true,
  handled:handled,
  start:position,
  last:position,
  since:now,
 }
}
func (s *touchGestureState) update(position uint16){
 if s.pressed{s.last=position}
}

// finish returns either a pending tap or a swipe direction. Leftward swipes
// advance to the next screen; rightward swipes go to the previous one.
// A button acted on at press time never fires again on release.
func (s *touchGestureState) finish(now time.Time)(tap uint16,swipe int,shouldTap bool){
 if !s.pressed{return 0,0,false}
 start,last,handled,since:=s.start,s.last,s.handled,s.since
 *s=touchGestureState{}
 if handled{return 0,0,false}
 elapsed:=now.Sub(since)
 if elapsed>=0&&elapsed<=nexusSwipeTimeout{
  delta:=int(last)-int(start)
  if delta<=-nexusSwipeDistance{return 0,1,false}
  if delta>=nexusSwipeDistance{return 0,-1,false}
 }
 return start,0,true
}

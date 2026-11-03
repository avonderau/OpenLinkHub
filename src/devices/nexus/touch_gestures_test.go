package nexus

import (
 "encoding/binary"
 "testing"
 "time"
)

func TestNexusGestureClassification(t *testing.T){
 now:=time.Unix(500,0)
 var state touchGestureState
 state.begin(380,now,false)
 state.update(230)
 _,direction,tap:=state.finish(now.Add(300*time.Millisecond))
 if direction!=1||tap{t.Fatalf("left swipe expected next, got %d/%t",direction,tap)}
 state.begin(160,now,false)
 state.update(320)
 _,direction,tap=state.finish(now.Add(400*time.Millisecond))
 if direction!= -1||tap{t.Fatalf("right swipe expected previous, got %d/%t",direction,tap)}
 state.begin(300,now,false)
 state.update(250)
 position,direction,tap:=state.finish(now.Add(100*time.Millisecond))
 if !tap||direction!=0||position!=300{t.Fatalf("short move should tap start: %d/%d/%t",position,direction,tap)}
 state.begin(360,now,false)
 state.update(120)
 position,direction,tap=state.finish(now.Add(3*time.Second))
 if !tap||direction!=0||position!=360{t.Fatal("slow drag should not swipe")}
 state.begin(200,now,true)
 state.update(0)
 _,direction,tap=state.finish(now.Add(100*time.Millisecond))
 if tap||direction!=0{t.Fatal("interactive touch triggered twice")}
 _,direction,tap=state.finish(now.Add(time.Second))
 if tap||direction!=0{t.Fatal("duplicate release triggered action")}
}

func TestNexusTouchReportsDispatchSwipeOnBlankAreas(t *testing.T){
 d:=&Device{customScreens:ScreenConfiguration{
  Version:1,CyclingMode:"manual",ActiveScreenID:"one",
  Screens:[]Screen{{ID:"one",Name:"One",Widgets:[]Widget{}},{ID:"two",Name:"Two",Widgets:[]Widget{}}},
 }}
 var state touchGestureState
 send:=func(active byte,x uint16){
  data:=make([]byte,8);data[5]=active
  binary.LittleEndian.PutUint16(data[6:8],x)
  d.processCustomTouchReport(data,&state)
 }
 send(1,400)
 if d.customScreens.ActiveScreenID!="one"{t.Fatal("blank-area press navigated before release")}
 send(1,250)
 send(0,0)
 if d.customScreens.ActiveScreenID!="two"{t.Fatal("left swipe did not go to next screen")}
 send(1,180);send(1,340);send(0,0)
 if d.customScreens.ActiveScreenID!="one"{t.Fatal("right swipe did not go to previous screen")}
 // Edge taps remain functional, but dispatch once on release.
 send(1,620)
 if d.customScreens.ActiveScreenID!="one"{t.Fatal("edge tap triggered before release")}
 send(0,0)
 if d.customScreens.ActiveScreenID!="two"{t.Fatal("right edge tap stopped working")}
 d.customScreens.CyclingMode="off"
 send(1,360);send(1,180);send(0,0)
 if d.customScreens.ActiveScreenID!="two"{t.Fatal("swipe bypassed fixed-screen mode")}
}

func TestNexusButtonTapFiresImmediatelyAndOnlyOnce(t *testing.T){
 d:=&Device{customScreens:ScreenConfiguration{
  Version:1,CyclingMode:"manual",ActiveScreenID:"one",
  Screens:[]Screen{
   {ID:"one",Name:"One",Widgets:[]Widget{{ID:"next",Type:"button",X:80,Y:0,Width:160,Height:48,Action:"next"}}},
   {ID:"two",Name:"Two",Widgets:[]Widget{}},
   {ID:"three",Name:"Three",Widgets:[]Widget{}},
  },
 }}
 var state touchGestureState
 press:=make([]byte,8);press[5]=1;binary.LittleEndian.PutUint16(press[6:8],120)
 d.processCustomTouchReport(press,&state)
 if d.customScreens.ActiveScreenID!="two"{t.Fatal("button press not immediate")}
 move:=make([]byte,8);move[5]=1;binary.LittleEndian.PutUint16(move[6:8],360)
 d.processCustomTouchReport(move,&state)
 release:=make([]byte,6)
 d.processCustomTouchReport(release,&state)
 if d.customScreens.ActiveScreenID!="two"{t.Fatal("button press triggered a swipe or duplicate on release")}
}

func TestNexusTouchReportsRejectOutOfBoundsPosition(t *testing.T){
 d:=&Device{customScreens:ScreenConfiguration{
  Version:1,CyclingMode:"manual",ActiveScreenID:"first",
  Screens:[]Screen{{ID:"first",Name:"First"},{ID:"other",Name:"Other"}},
 }}
 var state touchGestureState
 bad:=make([]byte,8);bad[5]=1;binary.LittleEndian.PutUint16(bad[6:8],65535)
 d.processCustomTouchReport(bad,&state)
 if state.pressed{t.Fatal("invalid HID coordinate accepted")}
 d.processCustomTouchReport([]byte{0,0,0,0,0,0},&state)
 if d.customScreens.ActiveScreenID!="first"{t.Fatal("invalid coordinate navigated screen")}
}

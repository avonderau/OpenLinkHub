package nexus

import (
 "testing"
 "time"
 "image/color"
)

func TestClockFormats(t *testing.T){
 now:=time.Date(2026,11,2,15,4,0,0,time.UTC)
 cases:=map[string]string{"":"15:04","24h":"15:04","12h":"3:04 PM","date":"Mon Nov 2","datetime":"Nov 2 15:04"}
 for format,want:=range cases{
  got:=formatNexusClock(map[string]string{"clockFormat":format},now)
  if got!=want{t.Errorf("format %q returned %q, expected %q",format,got,want)}
  if !validNexusClockFormat(format){t.Errorf("rejected valid clock format %q",format)}
 }
 if validNexusClockFormat("os.execute"){t.Fatal("accepted unknown clock format")}
}

func TestClockWidgetConfigAndRender(t *testing.T){
 screen:=Screen{ID:"clock",Name:"Clock",Widgets:[]Widget{{
  ID:"date",Type:"clock",X:0,Y:0,Width:200,Height:48,
  Settings:map[string]string{"label":"TODAY","clockFormat":"datetime","textColor":"#ffffff","style":"outline"},
 }}}
 config:=ScreenConfiguration{Version:1,ActiveScreenID:"clock",Screens:[]Screen{screen}}
 if err:=config.Validate();err!=nil{t.Fatal(err)}
 image,err:=RenderScreen(screen,nil)
 if err!=nil{t.Fatal(err)}
 base:=color.RGBA{12,19,27,255}
 changed:=false
 for y:=8;y<28;y++{
  for x:=3;x<190;x++{
   if image.RGBAAt(x,y)!=base{changed=true;break}
  }
  if changed{break}
 }
 if !changed{t.Fatal("clock text was not rendered")}
 config.Screens[0].Widgets[0].Settings["clockFormat"]="unsupported"
 if err:=config.Validate();err==nil{t.Fatal("invalid clock format accepted")}
}

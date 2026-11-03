package nexus

import (
 "image/color"
 "testing"
)

func TestNexusVerticalTextAlignValidation(t *testing.T) {
 for _,value:=range []string{"","top","middle","bottom"} {
  if !validNexusVerticalAlign(value) {t.Errorf("rejected %q",value)}
 }
 for _,value:=range []string{"center","baseline","../../foo"} {
  if validNexusVerticalAlign(value) {t.Errorf("accepted invalid %q",value)}
 }
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"test",Screens:[]Screen{{
  ID:"test",Name:"Test",Widgets:[]Widget{{
   ID:"text",Type:"text",X:0,Y:0,Width:200,Height:48,
   Settings:map[string]string{"text":"Alignment","verticalAlign":"middle"},
  }},
 }}}
 if err:=cfg.Validate();err!=nil{t.Fatal(err)}
 cfg.Screens[0].Widgets[0].Settings["verticalAlign"]="sideways"
 if err:=cfg.Validate();err==nil{t.Fatal("invalid alignment accepted")}
}

func TestVerticalTextChangesPositionAndPreservesWidgetBounds(t *testing.T) {
 w:=Widget{ID:"test",Type:"text",X:16,Y:0,Width:200,Height:48,Settings:map[string]string{
  "text":"TEST","fontSize":"normal","verticalAlign":"top",
 }}
 screen:=Screen{ID:"test",Name:"Test",Widgets:[]Widget{w}}
 top,err:=RenderScreen(screen,nil);if err!=nil{t.Fatal(err)}
 screen.Widgets[0].Settings["verticalAlign"]="bottom"
 bottom,err:=RenderScreen(screen,nil);if err!=nil{t.Fatal(err)}
 differences:=0
 for y:=0;y<48;y++{
  for x:=16;x<216;x++{
   if top.RGBAAt(x,y)!=bottom.RGBAAt(x,y){differences++}
  }
 }
 if differences==0{t.Fatal("vertical alignment did not change text pixels")}
 if top.RGBAAt(15,10)!=color.RGBA{12,19,27,255}{t.Fatal("rendered outside widget")}
}

package nexus

import (
 "image"
 "image/color"
 "testing"
)

func TestMediaIconsDrawWithinButton(t *testing.T) {
 icons:=[]string{"previous-track","play-pause","next-track","stop","volume-down","volume-up","next","previous"}
 for _,name:=range icons {
  t.Run(name,func(t *testing.T){
   frame:=image.NewRGBA(image.Rect(0,0,ScreenWidth,ScreenHeight))
   bounds:=image.Rect(100,0,200,48)
   drawMediaIcon(frame,bounds,name,color.RGBA{225,245,255,255})
   lit:=0
   for y:=0;y<ScreenHeight;y++{
    for x:=0;x<ScreenWidth;x++{
     c:=frame.RGBAAt(x,y)
     if c!= (color.RGBA{}) {
      if !image.Pt(x,y).In(bounds) {t.Fatalf("icon %s drew outside bounds at %d,%d",name,x,y)}
      lit++
     }
    }
   }
   if lit==0 {t.Fatalf("icon %s rendered no pixels",name)}
  })
 }
}

func TestMediaButtonIconAppearsOnRenderedScreen(t *testing.T) {
 screen:=Screen{ID:"media",Name:"Media",Widgets:[]Widget{{
  ID:"play",Type:"button",X:0,Y:0,Width:100,Height:48,
  Action:"media",Target:"play-pause",Settings:map[string]string{"icon":"play-pause"},
 }}}
 image,err:=RenderScreen(screen,nil)
 if err!=nil {t.Fatal(err)}
 background:=color.RGBA{27,41,53,255}
 found:=false
 for y:=4;y<40;y++ {for x:=15;x<80;x++ {if image.RGBAAt(x,y)!=background {found=true}}}
 if !found {t.Fatal("media control icon was not drawn")}
}

func TestStyledMacroButtonRender(t *testing.T){
 screen:=Screen{ID:"styled",Name:"Styled",Widgets:[]Widget{{
  ID:"action",Type:"button",X:0,Y:0,Width:96,Height:48,Action:"macro",
  Steps:[]MacroStep{{Type:"media",Value:"play-pause"}},
  Settings:map[string]string{"icon":"play-pause","backgroundColor":"#123456","textColor":"#aabbcc","borderColor":"#ff9900","style":"solid"},
 }}}
 result,err:=RenderScreen(screen,nil)
 if err!=nil {t.Fatal(err)}
 if got:=result.RGBAAt(20,2);got!=(color.RGBA{0x12,0x34,0x56,255}){t.Fatalf("background color %v",got)}
 if got:=result.RGBAAt(0,0);got!=(color.RGBA{0xff,0x99,0,255}){t.Fatalf("border color %v",got)}
}

package nexus

import (
 "bytes"
 "image"
 "image/color"
 "image/png"
 "path/filepath"
 "testing"
)

func TestImageWidgetLoadsCustomArtwork(t *testing.T){
 iconDir:=filepath.Join(t.TempDir(),"icons")
 source:=image.NewRGBA(image.Rect(0,0,16,16))
 for y:=0;y<16;y++{for x:=0;x<16;x++{source.SetRGBA(x,y,color.RGBA{240,25,45,255})}}
 var buf bytes.Buffer
 if err:=png.Encode(&buf,source);err!=nil{t.Fatal(err)}
 if _,err:=SaveCustomIcon(iconDir,"image_test",buf.Bytes());err!=nil{t.Fatal(err)}
 screen:=Screen{ID:"screen",Name:"Image",Widgets:[]Widget{{
  ID:"art",Type:"image",X:8,Y:0,Width:96,Height:48,
  Settings:map[string]string{"customIcon":"image_test","style":"flat"},
 }}}
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"screen",Screens:[]Screen{screen}}
 if err:=cfg.Validate();err!=nil{t.Fatal(err)}
 rendered,err:=RenderScreenWithBackgroundAndIcons(screen,nil,nil,iconDir)
 if err!=nil{t.Fatal(err)}
 pixel:=rendered.RGBAAt(56,24)
 if pixel.R<150||pixel.G>100{t.Fatalf("image widget was not drawn: %v",pixel)}
 if rendered.RGBAAt(4,24)!=(color.RGBA{12,19,27,255}){t.Fatal("image exceeded widget bounds")}
}

func TestImageWidgetRejectsUnsafeReferences(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"screen",Screens:[]Screen{{
  ID:"screen",Name:"Image",Widgets:[]Widget{{
   ID:"art",Type:"image",X:0,Y:0,Width:64,Height:48,
   Settings:map[string]string{"customIcon":"../../secret"},
  }},
 }}}
 if err:=cfg.Validate();err==nil{t.Fatal("unsafe image asset ID accepted")}
}

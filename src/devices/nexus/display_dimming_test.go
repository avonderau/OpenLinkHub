package nexus

import (
 "image"
 "image/color"
 "testing"
)

func TestNexusSoftwareBrightnessScalesRGBOnly(t *testing.T){
 frame:=image.NewRGBA(image.Rect(0,0,2,1))
 frame.SetRGBA(0,0,color.RGBA{200,100,50,255})
 frame.SetRGBA(1,0,color.RGBA{100,200,220,128})
 applyNexusSoftwareBrightness(frame,40)
 expected:=color.RGBA{80,40,20,255}
 if got:=frame.RGBAAt(0,0);got!=expected{t.Fatalf("wrong dimmed pixel %v expected %v",got,expected)}
 if got:=frame.RGBAAt(1,0);got.A!=128{t.Fatalf("alpha changed %v",got)}
}

func TestNexusSoftwareBrightnessDefaultsAndRangeValidation(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"main",Screens:[]Screen{{
  ID:"main",Name:"Main",Widgets:[]Widget{},
 }}}
 for _,v:=range []int{0,20,40,80,100}{
  cfg.Brightness=v
  if err:=cfg.Validate();err!=nil{t.Fatalf("brightness %d rejected: %v",v,err)}
 }
 for _,v:=range []int{-1,1,19,101,250}{
  cfg.Brightness=v
  if err:=cfg.Validate();err==nil{t.Fatalf("invalid brightness %d accepted",v)}
 }
 frame:=image.NewRGBA(image.Rect(0,0,1,1))
 frame.SetRGBA(0,0,color.RGBA{100,150,200,255})
 before:=frame.RGBAAt(0,0)
 applyNexusSoftwareBrightness(frame,0)
 if frame.RGBAAt(0,0)!=before{t.Fatal("missing brightness dimmed legacy pixels")}
 applyNexusSoftwareBrightness(frame,100)
 if frame.RGBAAt(0,0)!=before{t.Fatal("full brightness modified pixels")}
}

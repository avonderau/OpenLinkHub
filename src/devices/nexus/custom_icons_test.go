package nexus

import (
 "bytes"
 "image"
 "image/color"
 "image/draw"
 "image/png"
 "path/filepath"
 "testing"
)

func testIconPNG(t *testing.T)[]byte{
 t.Helper()
 img:=image.NewRGBA(image.Rect(0,0,64,64))
 draw.Draw(img,img.Bounds(),image.NewUniform(color.RGBA{235,50,20,255}),image.Point{},draw.Src)
 var b bytes.Buffer
 if err:=png.Encode(&b,img);err!=nil{t.Fatal(err)}
 return b.Bytes()
}

func TestCustomIconStorage(t *testing.T){
 dir:=t.TempDir()
 contents:=testIconPNG(t)
 path,err:=SaveCustomIcon(dir,"volume_custom",contents)
 if err!=nil{t.Fatal(err)}
 if path!=filepath.Join(dir,"volume_custom.png"){t.Fatalf("unexpected path %q",path)}
 icon,err:=ReadCustomIcon(dir,"volume_custom")
 if err!=nil{t.Fatal(err)}
 if icon.Bounds().Dx()!=32||icon.Bounds().Dy()!=32{t.Fatalf("icon not resized: %v",icon.Bounds())}
 if _,err:=SaveCustomIcon(dir,"../escape",contents);err==nil{t.Fatal("traversal icon ID accepted")}
 if _,err:=SaveCustomIcon(dir,"empty",nil);err==nil{t.Fatal("empty icon accepted")}
 if _,err:=SaveCustomIcon(dir,"huge",make([]byte,maxCustomIconBytes+1));err==nil{t.Fatal("oversized icon accepted")}
 if _,err:=ReadCustomIcon(dir,"../escape");err==nil{t.Fatal("traversal read accepted")}
}

func TestCustomIconRendersInNexusButton(t *testing.T){
 dir:=t.TempDir()
 if _,err:=SaveCustomIcon(dir,"red",testIconPNG(t));err!=nil{t.Fatal(err)}
 screen:=Screen{ID:"icons",Name:"Icons",Widgets:[]Widget{{
  ID:"custom",Type:"button",Action:"next",X:0,Y:0,Width:100,Height:48,
  Settings:map[string]string{"customIcon":"red","icon":"custom","label":""},
 }}}
 frame,err:=RenderScreenWithBackgroundAndIcons(screen,nil,nil,dir)
 if err!=nil{t.Fatal(err)}
 pixel:=frame.RGBAAt(50,17)
 if pixel.R<200 || pixel.G>100{t.Fatalf("custom artwork not drawn: %v",pixel)}
 fallback,err:=RenderScreen(screen,nil)
 if err!=nil{t.Fatal(err)}
 if fallback.RGBAAt(50,17)==pixel{t.Fatal("custom artwork appeared without supplied icon directory")}
}

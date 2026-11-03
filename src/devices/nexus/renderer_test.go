package nexus

import ("image/color";"testing")
func TestRenderScreen(t *testing.T){
 s:=Screen{ID:"system",Name:"System",Widgets:[]Widget{{ID:"cpu",Type:"bar",Metric:"cpu.utilization",X:0,Y:0,Width:160,Height:48,Settings:map[string]string{"label":"CPU","unit":"%"}}}}
 img,err:=RenderScreen(s,MetricSnapshot{"cpu.utilization":54.2})
 if err!=nil{t.Fatal(err)}
 if img.Bounds().Dx()!=640||img.Bounds().Dy()!=48{t.Fatal("unexpected framebuffer dimensions")}
 if img.At(10,44)==(color.RGBA{12,19,27,255}){t.Fatal("widget not drawn")}
}
func TestRenderScreenRejectsOutOfBounds(t *testing.T){
 s:=Screen{ID:"system",Name:"System",Widgets:[]Widget{{ID:"bad",Type:"metric",Metric:"cpu.temperature",X:639,Y:0,Width:4,Height:48}}}
 if _,err:=RenderScreen(s,nil);err==nil{t.Fatal("invalid widget accepted")}
}
func TestRenderScreenMissingMetric(t *testing.T){
 s:=Screen{ID:"system",Name:"System",Widgets:[]Widget{{ID:"missing",Type:"metric",Metric:"gpu.temperature",X:0,Y:0,Width:100,Height:48}}}
 if _,err:=RenderScreen(s,nil);err!=nil{t.Fatal(err)}
}

func TestRenderColoredMetricBarAndText(t *testing.T){
 for _,kind:=range []string{"metric","bar","text"} {
  t.Run(kind,func(t *testing.T){
   w:=Widget{ID:"colored",Type:kind,X:0,Y:0,Width:120,Height:48,Settings:map[string]string{"backgroundColor":"#123456","textColor":"#aabbcc","borderColor":"#ff8800","text":"Text"}}
   if kind=="metric"||kind=="bar" {w.Metric="cpu.temperature"}
   screen:=Screen{ID:"colored",Name:"Colored",Widgets:[]Widget{w}}
   img,err:=RenderScreen(screen,MetricSnapshot{"cpu.temperature":42})
   if err!=nil{t.Fatal(err)}
   if got:=img.RGBAAt(20,2);got!=(color.RGBA{0x12,0x34,0x56,255}){t.Fatalf("background: %v",got)}
   if got:=img.RGBAAt(0,0);got!=(color.RGBA{0xff,0x88,0,255}){t.Fatalf("border: %v",got)}
  })
 }
}

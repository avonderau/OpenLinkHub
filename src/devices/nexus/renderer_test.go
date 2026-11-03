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

package nexus

import (
 "fmt"
 "image"
 "image/color"
 "image/draw"
 "image/jpeg"
 "image/png"
 "os"
 "path/filepath"
 "io"
 _ "image/gif"
 "math"
 "strings"

 "golang.org/x/image/font"
 xdraw "golang.org/x/image/draw"
 "golang.org/x/image/font/basicfont"
 "golang.org/x/image/math/fixed"
)

// MetricSnapshot is an immutable snapshot supplied by the device's telemetry
// collector. Missing metrics are intentionally displayed as unavailable.
type MetricSnapshot map[string]float64

// RenderScreen draws a validated screen to the Nexus's native 640x48 RGBA
// framebuffer. It does not interact with HID and can be tested independently.
func RenderScreen(screen Screen, values MetricSnapshot) (*image.RGBA,error) {
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:screen.ID,Screens:[]Screen{screen}}
 if err:=cfg.Validate();err!=nil{return nil,err}
 canvas:=image.NewRGBA(image.Rect(0,0,ScreenWidth,ScreenHeight))
 draw.Draw(canvas,canvas.Bounds(),image.NewUniform(color.RGBA{12,19,27,255}),image.Point{},draw.Src)
 for _,w:=range screen.Widgets {
  bounds:=image.Rect(w.X,w.Y,w.X+w.Width,w.Y+w.Height)
  draw.Draw(canvas,bounds,image.NewUniform(color.RGBA{27,41,53,255}),image.Point{},draw.Src)
  label:=w.Settings["label"]
  if label=="" {label=w.Metric}
  if w.Type=="text" {label=w.Settings["text"]}
  if w.Type=="button" || w.Type=="macro" {if label=="" {label="Touch"} }
  if (w.Type=="button" || w.Type=="macro") && w.Settings["icon"]!="" && w.Settings["icon"]!="none" {
   drawMediaIcon(canvas,bounds,w.Settings["icon"])
   if label!="" {writeWidgetText(canvas,w.X+3,w.Y+w.Height-3,clipText(label,w.Width),color.RGBA{235,245,252,255})}
   continue
  }
  label=clipText(label,w.Width)
  writeWidgetText(canvas,w.X+3,w.Y+11,label,color.RGBA{172,196,213,255})
  if w.Type=="text" || w.Type=="button" || w.Type=="macro" {continue}
  value,ok:=values[w.Metric]
  shown:="--"
  if ok&&!math.IsNaN(value)&&!math.IsInf(value,0) {shown=fmt.Sprintf("%.1f",value)}
  if unit:=w.Settings["unit"];unit!=""&&shown!="--" {shown+=unit}
  writeWidgetText(canvas,w.X+3,w.Y+24,clipText(shown,w.Width),color.RGBA{255,255,255,255})
  if w.Type=="bar"&&ok&&!math.IsNaN(value)&&!math.IsInf(value,0) {
   pct:=math.Max(0,math.Min(100,value))
   barY:=w.Y+w.Height-7
   if barY>=w.Y+25 {
    background:=image.Rect(w.X+3,barY,w.X+w.Width-3,barY+3)
    draw.Draw(canvas,background,image.NewUniform(color.RGBA{65,79,91,255}),image.Point{},draw.Src)
    fill:=image.Rect(background.Min.X,barY,background.Min.X+int(float64(background.Dx())*pct/100),barY+3)
    draw.Draw(canvas,fill,image.NewUniform(color.RGBA{92,213,175,255}),image.Point{},draw.Src)
   }
  }
 }
 return canvas,nil
}
func writeWidgetText(dst *image.RGBA,x,y int,s string,c color.Color){
 drawer:=font.Drawer{Dst:dst,Src:image.NewUniform(c),Face:basicfont.Face7x13,Dot:fixed.P(x,y)}
 drawer.DrawString(s)
}
func clipText(s string,width int)string{
 limit:=(width-6)/7
 if limit<=0{return ""}
 r:=[]rune(strings.ToValidUTF8(s,"?"))
 if len(r)>limit{return string(r[:limit])}
 return string(r)
}

// RenderScreenWithBackground composites a trusted, decoded image behind widgets.
// Images are scaled to the exact Nexus framebuffer dimensions.
func RenderScreenWithBackground(screen Screen, values MetricSnapshot, background image.Image)(*image.RGBA,error){
 frame,err:=RenderScreen(screen,values)
 if err!=nil||background==nil{return frame,err}
 base:=image.NewRGBA(frame.Bounds())
 xdraw.CatmullRom.Scale(base,base.Bounds(),background,background.Bounds(),draw.Src,nil)
 for _,w:=range screen.Widgets {
  rect:=image.Rect(w.X,w.Y,w.X+w.Width,w.Y+w.Height)
  draw.Draw(base,rect,frame,rect.Min,draw.Over)
 }
 return base,nil
}
// ReadBackground accepts only validated background IDs and bounded PNG/JPEG
// files from the application's private background directory.
func ReadBackground(dir,id string)(image.Image,error){
 if !safeID.MatchString(id){return nil,fmt.Errorf("invalid background ID")}
 for _,ext:=range []string{".png",".jpg",".jpeg"} {
  file,err:=os.Open(filepath.Join(dir,id+ext));if err!=nil{continue}
  stat,err:=file.Stat();if err!=nil||stat.Size()>4*1024*1024 {file.Close();return nil,fmt.Errorf("background exceeds size limit")}
  cfg,format,err:=image.DecodeConfig(io.LimitReader(file,4*1024*1024))
  if err!=nil||cfg.Width<1||cfg.Height<1||cfg.Width>4096||cfg.Height>4096 {file.Close();return nil,fmt.Errorf("invalid background dimensions")}
  if _,err=file.Seek(0,0);err!=nil{file.Close();return nil,err}
  var img image.Image
  if format=="png" {img,err=png.Decode(file)} else if format=="jpeg" {img,err=jpeg.Decode(file)} else {err=fmt.Errorf("unsupported image format")}
  file.Close()
  return img,err
 }
 return nil,os.ErrNotExist
}

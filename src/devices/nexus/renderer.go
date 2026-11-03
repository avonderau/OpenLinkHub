package nexus

import (
 "fmt"
 "image"
 "image/color"
 "image/draw"
 "math"
 "strings"
 "strconv"
 "time"

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
 return renderNexusScreen(screen,values,"",nil,nil)
}

func renderNexusScreen(screen Screen,values MetricSnapshot,iconDir string,background image.Image,history MetricHistory)(*image.RGBA,error){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:screen.ID,Screens:[]Screen{screen}}
 if err:=cfg.Validate();err!=nil{return nil,err}
 canvas:=image.NewRGBA(image.Rect(0,0,ScreenWidth,ScreenHeight))
 if background!=nil {
  xdraw.CatmullRom.Scale(canvas,canvas.Bounds(),background,background.Bounds(),draw.Src,nil)
 }else{
  draw.Draw(canvas,canvas.Bounds(),image.NewUniform(color.RGBA{12,19,27,255}),image.Point{},draw.Src)
 }
 for _,w:=range screen.Widgets {
  bounds:=image.Rect(w.X,w.Y,w.X+w.Width,w.Y+w.Height)
  bg:=nexusWidgetColor(w.Settings,"backgroundColor",color.RGBA{27,41,53,255})
  ink:=nexusWidgetColor(w.Settings,"textColor",color.RGBA{235,245,252,255})
  border:=nexusWidgetColor(w.Settings,"borderColor",color.RGBA{128,189,217,255})
  style:=w.Settings["style"]
  if style=="" {style="solid"}
  // Styles are supported for all widget types. Flat and outline leave the background visible.
  if style!="flat" && style!="outline" {draw.Draw(canvas,bounds,image.NewUniform(bg),image.Point{},draw.Src)}
  if style=="outline" || (style=="solid" && w.Settings["borderColor"]!="") {drawNexusButtonBorder(canvas,bounds,border)}
  label:=w.Settings["label"]
  if label=="" {label=w.Metric}
  if w.Type=="text" {label=w.Settings["text"]}
  if w.Type=="clock" {
   if label!="" {writeWidgetText(canvas,w.X+3,w.Y+12,clipText(label,w.Width),ink)}
   writeWidgetText(canvas,w.X+3,w.Y+27,clipText(formatNexusClock(w.Settings,time.Now()),w.Width),ink)
   continue
  }
  if w.Type=="button" || w.Type=="macro" {if label=="" && w.Settings["icon"]=="" && w.Settings["customIcon"]=="" {label="Touch"} }
  if (w.Type=="button" || w.Type=="macro") && w.Settings["customIcon"]!="" && iconDir!="" {
   icon,err:=ReadCustomIcon(iconDir,w.Settings["customIcon"])
   if err==nil {
    size:=32
    if max:=bounds.Dx()-4;max<size{size=max}
    if max:=bounds.Dy()-4;max<size{size=max}
    if size>0 {
     cx:=(bounds.Min.X+bounds.Max.X)/2; cy:=(bounds.Min.Y+bounds.Max.Y)/2
     if label!="" && bounds.Dy()>30 {cy-=5}
     destination:=image.Rect(cx-size/2,cy-size/2,cx+(size+1)/2,cy+(size+1)/2).Intersect(bounds)
     xdraw.CatmullRom.Scale(canvas,destination,icon,icon.Bounds(),draw.Over,nil)
    }
    if label!="" {writeWidgetText(canvas,w.X+3,w.Y+w.Height-3,clipText(label,w.Width),ink)}
    continue
   }
  }
  if (w.Type=="button" || w.Type=="macro") && w.Settings["icon"]!="" && w.Settings["icon"]!="none" && w.Settings["icon"]!="custom" {
   drawMediaIcon(canvas,bounds,w.Settings["icon"],ink)
   if label!="" {writeWidgetText(canvas,w.X+3,w.Y+w.Height-3,clipText(label,w.Width),ink)}
   continue
  }
  label=clipText(label,w.Width)
  writeWidgetText(canvas,w.X+3,w.Y+11,label,ink)
  if w.Type=="graph"{
   drawNexusHistoryGraph(canvas,bounds,w.Metric,history[w.Metric],ink)
   continue
  }
  if w.Type=="text" || w.Type=="button" || w.Type=="macro" {continue}
  value,ok:=values[w.Metric]
  shown:="--"
  if ok&&!math.IsNaN(value)&&!math.IsInf(value,0) {shown=fmt.Sprintf("%.1f",value)}
  if unit:=w.Settings["unit"];unit!=""&&shown!="--" {shown+=unit}
  writeWidgetText(canvas,w.X+3,w.Y+24,clipText(shown,w.Width),ink)
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
 return RenderScreenWithBackgroundAndIcons(screen,values,background,"")
}

func RenderScreenWithBackgroundAndIcons(screen Screen,values MetricSnapshot,background image.Image,iconDir string)(*image.RGBA,error){
 return renderNexusScreen(screen,values,iconDir,background,nil)
}
// ReadBackground returns the current frame for either a static or animated
// background, using the shared bounded image cache.
func RenderScreenWithBackgroundAndIconsHistory(screen Screen,values MetricSnapshot,background image.Image,iconDir string,history MetricHistory)(*image.RGBA,error){
 return renderNexusScreen(screen,values,iconDir,background,history)
}

func ReadBackground(dir,id string)(image.Image,error){
 return ReadBackgroundAt(dir,id,time.Now())
}

func nexusWidgetColor(settings map[string]string,key string,fallback color.RGBA)color.RGBA{
 value:=settings[key]
 if !validHexColor(value) {return fallback}
 n,err:=strconv.ParseUint(value[1:],16,24)
 if err!=nil {return fallback}
 return color.RGBA{uint8(n>>16),uint8(n>>8),uint8(n),255}
}
func drawNexusButtonBorder(img *image.RGBA,r image.Rectangle,c color.RGBA){
 if r.Empty(){return}
 for x:=r.Min.X;x<r.Max.X;x++ {img.Set(x,r.Min.Y,c);img.Set(x,r.Max.Y-1,c)}
 for y:=r.Min.Y;y<r.Max.Y;y++ {img.Set(r.Min.X,y,c);img.Set(r.Max.X-1,y,c)}
}

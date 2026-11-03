package nexus

import (
 "image"
 "image/color"
 "math"
 "time"
)

// PresetNexusBackground provides built-in artwork with no external assets.
// The pulse preset is animated from the device's regular framebuffer refresh.
func PresetNexusBackground(id string, now time.Time) image.Image {
 if id!="preset_aurora"&&id!="preset_neon"&&id!="preset_circuit"&&id!="preset_sunset"&&id!="preset_pulse"{return nil}
 dst:=image.NewRGBA(image.Rect(0,0,ScreenWidth,ScreenHeight))
 seconds:=float64(now.UnixNano())/1e9
 for y:=0;y<ScreenHeight;y++{
  for x:=0;x<ScreenWidth;x++{
   xf:=float64(x)/float64(ScreenWidth)
   yf:=float64(y)/float64(ScreenHeight)
   var r,g,b float64
   switch id {
   case "preset_aurora":
    wave:=math.Sin(xf*13+yf*7)+math.Sin(xf*23-yf*11)
    glow:=math.Max(0,1-math.Abs(yf-(.44+.18*math.Sin(xf*8)))*3)
    r,g,b=8+glow*18,20+glow*72+wave*4,35+glow*78
   case "preset_neon":
    line:=0.0
    if x%40==0||y%12==0{line=1}
    r,g,b=17+line*26,15+line*14,35+line*65
   case "preset_circuit":
    line:=0.0
    if (y==12||y==35)&&(x%96<70) || (x%96==20||x%96==69)&&(y>12&&y<35){line=1}
    r,g,b=6+line*14,25+line*73,33+line*55
   case "preset_sunset":
    horizon:=math.Max(0,1-math.Abs(yf-.73)*2)
    sun:=math.Max(0,1-math.Hypot((xf-.72)*3,(yf-.45)*2.3))
    r,g,b=36+105*horizon+56*sun,18+27*horizon+60*sun,48+29*horizon
   case "preset_pulse":
    // Moving luminous band and softly moving star field, regenerated each second.
    phase:=math.Mod(seconds*36,float64(ScreenWidth)+160)-80
    band:=math.Exp(-math.Pow((float64(x)-phase)/58,2))
    sparkle:=math.Max(0,math.Sin(float64(x)*.11+seconds*2)*math.Sin(float64(y)*.29-seconds*1.4))
    r,g,b=12+band*26+sparkle*6,17+band*84+sparkle*14,42+band*115+sparkle*22
   }
   toByte:=func(v float64)uint8{if v<0{return 0};if v>255{return 255};return uint8(v)}
   dst.SetRGBA(x,y,color.RGBA{toByte(r),toByte(g),toByte(b),255})
  }
 }
 return dst
}

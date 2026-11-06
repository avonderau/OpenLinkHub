package deck

import (
 "image"
 "image/color"
 "image/draw"

 xdraw "golang.org/x/image/draw"
)

// The default folder icons are rendered in-process so LCD keys work offline
// and the preview can use the matching static mask artwork.
func renderDeckFolderKey(size int, key KeyConfig) image.Image {
 canvas:=renderKeyLabel(size,"").(*image.RGBA)
 icon:=image.NewRGBA(image.Rect(0,0,48,48))
 ink:=color.RGBA{R:242,G:207,B:147,A:255}
 gold:=image.NewUniform(ink)
 switch key.Action {
 case "folder-open":
  draw.Draw(icon,image.Rect(7,13,24,22),gold,image.Point{},draw.Src)
  draw.Draw(icon,image.Rect(7,19,41,37),gold,image.Point{},draw.Src)
  draw.Draw(icon,image.Rect(10,23,38,34),image.NewUniform(color.RGBA{R:32,G:35,B:40,A:255}),image.Point{},draw.Src)
 case "folder-back":
  draw.Draw(icon,image.Rect(15,22,40,27),gold,image.Point{},draw.Src)
  for y:=12;y<=36;y++{
   width:=12-absDeckIconInt(y-24)
   for x:=7;x<=7+width;x++ {icon.Set(x,y,ink)}
  }
 }
 label:=keyDisplayLabel(key)
 glyphSize:=size*80/100
 top:=(size-glyphSize)/2
 if label!=""{glyphSize=size*65/100;top=1}
 left:=(size-glyphSize)/2
 xdraw.NearestNeighbor.Scale(canvas,image.Rect(left,top,left+glyphSize,top+glyphSize),
  icon,icon.Bounds(),draw.Over,nil)
 if label!=""{drawCenteredDeckString(canvas,label,size-6,color.White)}
 return canvas
}

func absDeckIconInt(n int) int {if n<0{return -n};return n}

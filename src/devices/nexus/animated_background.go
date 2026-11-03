package nexus

import (
 "bytes"
 "errors"
 "fmt"
 "image"
 "image/color"
 "image/draw"
 "image/gif"
 "io"
 "os"
 "path/filepath"
 "sync"
 "time"

 xdraw "golang.org/x/image/draw"
)

const (
 maxBackgroundBytes = 4 * 1024 * 1024
 maxAnimationFrames = 80
 maxAnimationPixels = 8_000_000
 maxAnimationDimension = 1280
 minAnimationDelay = 5 // centiseconds, so even fast GIFs remain display-safe
)

type animatedBackground struct {
 frames []*image.RGBA
 delays []time.Duration
 duration time.Duration
}

func (a *animatedBackground) frameAt(now time.Time) image.Image {
 if len(a.frames)==0{return nil}
 if len(a.frames)==1||a.duration<=0{return a.frames[0]}
 elapsed:=time.Duration(now.UnixNano()%int64(a.duration))
 for index,delay:=range a.delays {
  if elapsed<delay{return a.frames[index]}
  elapsed-=delay
 }
 return a.frames[len(a.frames)-1]
}

// inspectGIF counts descriptors before gif.DecodeAll can allocate palette images,
// rejecting oversized compressed GIFs without expanding every frame in memory.
func inspectGIF(data []byte)(int,int,int,error){
 if len(data)<13 || (string(data[:6])!="GIF87a"&&string(data[:6])!="GIF89a"){return 0,0,0,errors.New("invalid GIF signature")}
 width:=int(data[6])|int(data[7])<<8
 height:=int(data[8])|int(data[9])<<8
 if width<1||height<1||width>maxAnimationDimension||height>ScreenHeight*4 {return 0,0,0,errors.New("GIF dimensions exceed 1280 x 192")}
 offset:=13
 if data[10]&0x80!=0 {offset+=3*(1<<((data[10]&7)+1))}
 if offset>len(data){return 0,0,0,io.ErrUnexpectedEOF}
 frames:=0
 skipBlocks:=func()error{
  for{
   if offset>=len(data){return io.ErrUnexpectedEOF}
   size:=int(data[offset]);offset++
   if size==0{return nil}
   if size>len(data)-offset{return io.ErrUnexpectedEOF}
   offset+=size
  }
 }
 for offset<len(data) {
  marker:=data[offset];offset++
  switch marker{
  case 0x3b:
   if frames==0{return 0,0,0,errors.New("GIF contains no frames")}
   return width,height,frames,nil
  case 0x21:
   if offset>=len(data){return 0,0,0,io.ErrUnexpectedEOF}
   offset++ // extension label, including graphic-control and application extension
   if err:=skipBlocks();err!=nil{return 0,0,0,err}
  case 0x2c:
   if len(data)-offset<9{return 0,0,0,io.ErrUnexpectedEOF}
   frameWidth:=int(data[offset+4])|int(data[offset+5])<<8
   frameHeight:=int(data[offset+6])|int(data[offset+7])<<8
   packed:=data[offset+8];offset+=9
   if frameWidth<1||frameHeight<1{return 0,0,0,errors.New("invalid GIF frame size")}
   if packed&0x80!=0{offset+=3*(1<<((packed&7)+1))}
   if offset>=len(data){return 0,0,0,io.ErrUnexpectedEOF}
   offset++ // LZW minimum code size
   if err:=skipBlocks();err!=nil{return 0,0,0,err}
   frames++
   if frames>maxAnimationFrames || int64(width)*int64(height)*int64(frames)>maxAnimationPixels {
    return 0,0,0,errors.New("GIF animation exceeds frame or decoded-pixel limit")
   }
  default:
   return 0,0,0,fmt.Errorf("invalid GIF block %#x",marker)
  }
 }
 return 0,0,0,io.ErrUnexpectedEOF
}

func decodeAnimatedBackground(data []byte)(*animatedBackground,error){
 width,height,count,err:=inspectGIF(data)
 if err!=nil{return nil,err}
 source,err:=gif.DecodeAll(bytes.NewReader(data))
 if err!=nil{return nil,err}
 if len(source.Image)!=count||source.Config.Width!=width||source.Config.Height!=height{return nil,errors.New("GIF metadata mismatch")}
 current:=image.NewRGBA(image.Rect(0,0,width,height))
 result:=&animatedBackground{
  frames:make([]*image.RGBA,0,count),
  delays:make([]time.Duration,0,count),
 }
 // Respect disposal-to-background and disposal-to-previous semantics. Draw
 // the complete frame before scaling, including offset image rectangles.
 var background color.Color=color.Transparent
 if palette,ok:=source.Config.ColorModel.(color.Palette);ok&&int(source.BackgroundIndex)<len(palette){
  background=palette[source.BackgroundIndex]
 }
 for i,frame:=range source.Image{
  bounds:=frame.Bounds().Intersect(current.Bounds())
  var before *image.RGBA
  if i<len(source.Disposal)&&source.Disposal[i]==gif.DisposalPrevious {
   before=image.NewRGBA(current.Bounds())
   draw.Draw(before,before.Bounds(),current,image.Point{},draw.Src)
  }
  draw.Draw(current,bounds,frame,bounds.Min,draw.Over)
  scaled:=image.NewRGBA(image.Rect(0,0,ScreenWidth,ScreenHeight))
  xdraw.CatmullRom.Scale(scaled,scaled.Bounds(),current,current.Bounds(),draw.Src,nil)
  result.frames=append(result.frames,scaled)
  delay:=minAnimationDelay
  if i<len(source.Delay)&&source.Delay[i]>delay{delay=source.Delay[i]}
  if delay>1000{delay=1000}
  duration:=time.Duration(delay)*10*time.Millisecond
  result.delays=append(result.delays,duration)
  result.duration+=duration
  if i<len(source.Disposal) {
   switch source.Disposal[i]{
   case gif.DisposalBackground:
    draw.Draw(current,bounds,image.NewUniform(background),image.Point{},draw.Src)
   case gif.DisposalPrevious:
    if before!=nil{draw.Draw(current,current.Bounds(),before,image.Point{},draw.Src)}
   }
  }
 }
 return result,nil
}

// The cache holds only the two most recently used backgrounds, avoiding GIF
// re-decoding on each framebuffer refresh and limiting retained frame memory.
type cachedBackground struct {
 path string
 size int64
 modified time.Time
 static image.Image
 animated *animatedBackground
}
var backgroundsCache=struct{
 sync.Mutex
 entries []cachedBackground
}{}

func decodeBackgroundFile(path string)(image.Image,*animatedBackground,error){
 raw,err:=os.ReadFile(path)
 if err!=nil{return nil,nil,err}
 if len(raw)==0||len(raw)>maxBackgroundBytes{return nil,nil,errors.New("invalid background size")}
 if filepath.Ext(path)==".gif" {
  animation,err:=decodeAnimatedBackground(raw)
  return nil,animation,err
 }
 config,format,err:=image.DecodeConfig(bytes.NewReader(raw))
 if err!=nil{return nil,nil,err}
 if config.Width<1||config.Height<1||config.Width>4096||config.Height>4096{return nil,nil,errors.New("invalid background dimensions")}
 if format!="png"&&format!="jpeg"{return nil,nil,errors.New("unsupported background format")}
 picture,_,err:=image.Decode(bytes.NewReader(raw))
 return picture,nil,err
}

func backgroundFilePath(dir,id string)(string,error){
 if !safeID.MatchString(id){return "",errors.New("invalid background ID")}
 for _,ext:=range []string{".gif",".png",".jpg",".jpeg"}{
  path:=filepath.Join(dir,id+ext)
  stat,err:=os.Stat(path)
  if err==nil&&stat.Mode().IsRegular(){return path,nil}
 }
 return "",os.ErrNotExist
}

func IsAnimatedBackground(dir,id string)bool{
 path,err:=backgroundFilePath(dir,id)
 return err==nil && filepath.Ext(path)==".gif"
}

func ReadBackgroundAt(dir,id string,now time.Time)(image.Image,error){
 path,err:=backgroundFilePath(dir,id)
 if err!=nil{return nil,err}
 stat,err:=os.Stat(path)
 if err!=nil{return nil,err}
 if stat.Size()==0||stat.Size()>maxBackgroundBytes{return nil,errors.New("background size exceeds limit")}
 backgroundsCache.Lock()
 defer backgroundsCache.Unlock()
 for _,entry:=range backgroundsCache.entries{
  if entry.path==path && entry.size==stat.Size() && entry.modified.Equal(stat.ModTime()){
   if entry.animated!=nil{return entry.animated.frameAt(now),nil}
   return entry.static,nil
  }
 }
 picture,animation,err:=decodeBackgroundFile(path)
 if err!=nil{return nil,err}
 backgroundsCache.entries=append(backgroundsCache.entries,cachedBackground{path:path,size:stat.Size(),modified:stat.ModTime(),static:picture,animated:animation})
 if len(backgroundsCache.entries)>2{backgroundsCache.entries=backgroundsCache.entries[len(backgroundsCache.entries)-2:]}
 if animation!=nil{return animation.frameAt(now),nil}
 return picture,nil
}

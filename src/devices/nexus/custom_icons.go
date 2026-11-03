package nexus

import (
 "bytes"
 "fmt"
 "image"
 "image/png"
 "io"
 "os"
 "path/filepath"

 "golang.org/x/image/draw"
)

const maxCustomIconBytes = 512 * 1024

// SaveCustomIcon decodes a bounded PNG/JPEG and stores it as a normalized PNG.
// Stored IDs, rather than user-provided filenames, are referenced by widgets.
func SaveCustomIcon(dir,id string,contents []byte)(string,error){
 if !safeID.MatchString(id){return "",fmt.Errorf("invalid icon ID")}
 if len(contents)==0 || len(contents)>maxCustomIconBytes{return "",fmt.Errorf("icon must be at most 512 KiB")}
 config,format,err:=image.DecodeConfig(bytes.NewReader(contents))
 if err!=nil{return "",fmt.Errorf("invalid icon: %w",err)}
 if format!="png" && format!="jpeg"{return "",fmt.Errorf("only PNG and JPEG icons are supported")}
 if config.Width<1||config.Height<1||config.Width>512||config.Height>512{return "",fmt.Errorf("icon dimensions must be 1 to 512 pixels")}
 source,_,err:=image.Decode(bytes.NewReader(contents))
 if err!=nil{return "",fmt.Errorf("invalid icon pixels: %w",err)}
 // Fit the artwork in a 32x32 RGBA image while retaining transparency and aspect ratio.
 icon:=image.NewRGBA(image.Rect(0,0,32,32))
 scale:=float64(32)/float64(config.Width)
 if s:=float64(32)/float64(config.Height);s<scale{scale=s}
 width:=int(float64(config.Width)*scale);height:=int(float64(config.Height)*scale)
 if width<1{width=1};if height<1{height=1}
 target:=image.Rect((32-width)/2,(32-height)/2,(32+width)/2,(32+height)/2)
 draw.CatmullRom.Scale(icon,target,source,source.Bounds(),draw.Over,nil)
 if err=os.MkdirAll(dir,0700);err!=nil{return "",err}
 f,err:=os.CreateTemp(dir,".nexus-icon-*")
 if err!=nil{return "",err}
 defer os.Remove(f.Name())
 if err=f.Chmod(0600);err!=nil{f.Close();return "",err}
 if err=png.Encode(f,icon);err!=nil{f.Close();return "",err}
 if err=f.Sync();err!=nil{f.Close();return "",err}
 if err=f.Close();err!=nil{return "",err}
 path:=filepath.Join(dir,id+".png")
 if err=os.Rename(f.Name(),path);err!=nil{return "",err}
 return path,nil
}

func ReadCustomIcon(dir,id string)(image.Image,error){
 if !safeID.MatchString(id){return nil,fmt.Errorf("invalid icon ID")}
 f,err:=os.Open(filepath.Join(dir,id+".png"))
 if err!=nil{return nil,err}
 defer f.Close()
 stat,err:=f.Stat()
 if err!=nil{return nil,err}
 if stat.Size()>maxCustomIconBytes{return nil,fmt.Errorf("stored icon too large")}
 img,err:=png.Decode(io.LimitReader(f,maxCustomIconBytes))
 if err!=nil{return nil,err}
 if img.Bounds().Dx()!=32||img.Bounds().Dy()!=32{return nil,fmt.Errorf("invalid stored icon dimensions")}
 return img,nil
}

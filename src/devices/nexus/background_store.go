package nexus

import (
 "bytes"
 "fmt"
 "image"
 "image/jpeg"
 "image/png"
 "io"
 "os"
 "path/filepath"
)

// SaveBackground stores a bounded PNG, JPEG or validated animated GIF.
// The caller must enforce authentication and upload limits at the HTTP layer.
func SaveBackground(dir,id string,contents []byte)(string,error){
 if !safeID.MatchString(id){return "",fmt.Errorf("invalid background ID")}
 if len(contents)==0||len(contents)>maxBackgroundBytes{return "",fmt.Errorf("invalid image size")}
 cfg,format,err:=image.DecodeConfig(bytes.NewReader(contents))
 if err!=nil{return "",err}
 if cfg.Width<1||cfg.Height<1||cfg.Width>4096||cfg.Height>4096{return "",fmt.Errorf("invalid image dimensions")}
 var ext string
 switch format {
 case "png":ext=".png"
 case "jpeg":ext=".jpg"
 case "gif":
  ext=".gif"
  if _,err:=decodeAnimatedBackground(contents);err!=nil{return "",fmt.Errorf("invalid animated GIF: %w",err)}
 default:return "",fmt.Errorf("unsupported image format")
 }
 if err=os.MkdirAll(dir,0700);err!=nil{return "",err}
 file,err:=os.CreateTemp(dir,".nexus-background-*")
 if err!=nil{return "",err}
 defer os.Remove(file.Name())
 if err=file.Chmod(0600);err!=nil{file.Close();return "",err}
 if _,err=io.Copy(file,bytes.NewReader(contents));err!=nil{file.Close();return "",err}
 if err=file.Sync();err!=nil{file.Close();return "",err}
 if err=file.Close();err!=nil{return "",err}
 path:=filepath.Join(dir,id+ext)
 if err=os.Rename(file.Name(),path);err!=nil{return "",err}
 // A background ID refers to exactly one image format. Remove old extensions
 // so changing between GIF/PNG/JPEG never accidentally loads a stale asset.
 for _,other:=range []string{".png",".jpg",".jpeg",".gif"}{
  if other!=ext{_ = os.Remove(filepath.Join(dir,id+other))}
 }
 return path,nil
}
var _ = png.Decode
var _ = jpeg.Decode

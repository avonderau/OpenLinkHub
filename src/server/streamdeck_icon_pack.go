package server

import (
 "archive/zip"
 "bytes"
 "encoding/json"
 "errors"
 "fmt"
 "image"
 "image/draw"
 "image/gif"
 "image/png"
 "io"
 "path"
 "strings"
 "unicode"
 "unicode/utf8"

 "golang.org/x/image/webp"
 xdraw "golang.org/x/image/draw"

 "OpenLinkHub/src/devices/deck"
)

// Stream Deck archives use a manifest and an explicit searchable icon index.
// Import only index-referenced assets, never extract archive paths to disk.
type streamDeckManifest struct {
 Name string `json:"Name"`
 Author string `json:"Author"`
 Version string `json:"Version"`
 Description string `json:"Description"`
 Icon string `json:"Icon"`
 License string `json:"License"`
}
type streamDeckIcon struct {
 Path string `json:"path"`
 Name string `json:"name"`
 Tags []string `json:"tags"`
}
type streamDeckPack struct {
 Manifest streamDeckManifest
 Icons []pendingIcon
}

func safeStreamDeckPath(p string) bool {
 if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.ContainsRune(p, 0) { return false }
 for _, part := range strings.Split(p, "/") {
  if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") { return false }
 }
 return path.Clean(p) == p
}
func safeStreamDeckText(s string, max int) bool {
 if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > max { return false }
 for _, r := range s { if unicode.IsControl(r) { return false } }
 return true
}
func readStreamDeckMember(file *zip.File, limit int64) ([]byte,error) {
 if file.UncompressedSize64 > uint64(limit) { return nil,errors.New("Stream Deck member exceeds limit") }
 reader,err:=file.Open(); if err!=nil{return nil,err}
 data,readErr:=io.ReadAll(io.LimitReader(reader,limit+1))
 closeErr:=reader.Close()
 if readErr!=nil{return nil,readErr};if closeErr!=nil{return nil,closeErr}
 if len(data)==0 || int64(len(data))>limit {return nil,errors.New("invalid Stream Deck member size")}
 return data,nil
}
func streamDeckStillFrame(raw []byte, ext string) ([]byte,error) {
 var source image.Image
 var err error
 switch ext {
 case ".gif": source,err=gif.Decode(bytes.NewReader(raw))
 case ".webp": source,err=webp.Decode(bytes.NewReader(raw))
 default: return nil,errors.New("unsupported animation format")
 }
 if err!=nil{return nil,fmt.Errorf("invalid animated image: %w",err)}
 bounds:=source.Bounds();width,height:=bounds.Dx(),bounds.Dy()
 if width<1 || height<1 || width>512 || height>512 {return nil,errors.New("invalid animated icon dimensions")}
 // Deck icons are static. Convert only the first frame for both devices.
 target:=image.NewRGBA(image.Rect(0,0,128,128))
 ratio:=float64(128)/float64(width)
 if s:=float64(128)/float64(height);s<ratio{ratio=s}
 w,h:=int(float64(width)*ratio),int(float64(height)*ratio)
 if w<1{w=1};if h<1{h=1}
 rect:=image.Rect((128-w)/2,(128-h)/2,(128-w)/2+w,(128-h)/2+h)
 xdraw.CatmullRom.Scale(target,rect,source,bounds,draw.Over,nil)
 var output bytes.Buffer
 if err:=png.Encode(&output,target);err!=nil{return nil,err}
 if output.Len()>deck.MaxDeckIconBytes{return nil,errors.New("normalized icon too large")}
 return output.Bytes(),nil
}

func parseStreamDeckPack(archive *zip.Reader) (streamDeckPack,error) {
 var output streamDeckPack
 files:=make(map[string]*zip.File,len(archive.File))
 var metadataBytes int64
 for _, f:=range archive.File {
  if f.FileInfo().IsDir(){continue}
  if !safeStreamDeckPath(f.Name) || !f.Mode().IsRegular() {return output,errors.New("unsafe Stream Deck archive path or file type")}
  key:=strings.ToLower(f.Name)
  if files[key]!=nil{return output,errors.New("duplicate Stream Deck archive path")}
  files[key]=f
  if f.UncompressedSize64>uint64(maxIconSetTotalImageBytes) {return output,errors.New("oversized Stream Deck archive entry")}
  metadataBytes+=int64(f.UncompressedSize64)
  if metadataBytes>maxIconSetTotalImageBytes{return output,errors.New("Stream Deck archive exceeds extracted quota")}
 }
 manifestFile:=files["manifest.json"];indexFile:=files["icons.json"]
 if manifestFile==nil || indexFile==nil{return output,errors.New("Stream Deck archive requires manifest.json and icons.json")}
 manifestBytes,err:=readStreamDeckMember(manifestFile,16*1024);if err!=nil{return output,err}
 indexBytes,err:=readStreamDeckMember(indexFile,128*1024);if err!=nil{return output,err}
 if err=json.Unmarshal(manifestBytes,&output.Manifest);err!=nil{return output,fmt.Errorf("invalid Stream Deck manifest: %w",err)}
 if !safeStreamDeckText(output.Manifest.Name,60) || !safeStreamDeckText(output.Manifest.Author,120) || !safeStreamDeckText(output.Manifest.Version,40) || !safeStreamDeckText(output.Manifest.Icon,256) || !safeStreamDeckPath(output.Manifest.Icon) {
  return output,errors.New("invalid Stream Deck pack metadata")
 }
 if len(output.Manifest.Description)>1024 || (output.Manifest.License!="" && !safeStreamDeckPath(output.Manifest.License)){return output,errors.New("invalid Stream Deck optional metadata")}
 if files[strings.ToLower(output.Manifest.Icon)]==nil{return output,errors.New("missing Stream Deck pack artwork")}
 if output.Manifest.License!="" && files[strings.ToLower(output.Manifest.License)]==nil{return output,errors.New("missing Stream Deck license")}
 var indexes []streamDeckIcon
 if err=json.Unmarshal(indexBytes,&indexes);err!=nil{return output,fmt.Errorf("invalid Stream Deck icons index: %w",err)}
 if len(indexes)==0 || len(indexes)>maxIconSetImages{return output,fmt.Errorf("Stream Deck pack must contain 1-%d icons",maxIconSetImages)}
 usedNames:=map[string]bool{}
 usedPaths:=map[string]bool{}
 for _,entry:=range indexes {
  if !safeStreamDeckText(entry.Name,50) || !safeStreamDeckPath(entry.Path) || len(entry.Tags)>24 {return output,errors.New("invalid Stream Deck icon index entry")}
  if usedNames[strings.ToLower(entry.Name)] || usedPaths[strings.ToLower(entry.Path)] {return output,errors.New("duplicate Stream Deck icon name or path")}
  usedNames[strings.ToLower(entry.Name)]=true;usedPaths[strings.ToLower(entry.Path)]=true
  tags:=make([]string,0,len(entry.Tags))
  for _,tag:=range entry.Tags {
   if !safeStreamDeckText(tag,40){return output,errors.New("invalid Stream Deck icon tag")}
   tags=append(tags,tag)
  }
  full:="icons/"+entry.Path
  file:=files[strings.ToLower(full)]
  if file==nil{return output,fmt.Errorf("missing Stream Deck icon %q",entry.Path)}
  ext:=strings.ToLower(path.Ext(entry.Path))
  if ext!=".png" && ext!=".jpg" && ext!=".jpeg" && ext!=".svg" && ext!=".gif" && ext!=".webp" {return output,fmt.Errorf("unsupported Stream Deck icon %q",entry.Path)}
  raw,err:=readStreamDeckMember(file,deck.MaxDeckIconBytes);if err!=nil{return output,err}
  switch ext {
  case ".svg":
   raw,err=rasterizeIconSVG(raw)
  case ".gif",".webp":
   raw,err=streamDeckStillFrame(raw,ext)
  default:
   cfg,format,decodeErr:=image.DecodeConfig(bytes.NewReader(raw))
   expected:="jpeg";if ext==".png"{expected="png"}
   if decodeErr!=nil || format!=expected || cfg.Width<1 || cfg.Height<1 || cfg.Width>512 || cfg.Height>512 {err=errors.New("invalid Stream Deck image")}
  }
  if err!=nil{return output,fmt.Errorf("icon %q: %w",entry.Name,err)}
  output.Icons=append(output.Icons,pendingIcon{Name:entry.Name,Tags:tags,Data:raw})
 }
 return output,nil
}

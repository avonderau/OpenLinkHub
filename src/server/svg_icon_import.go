package server

import (
 "bytes"
 "context"
 "encoding/xml"
 "errors"
 "fmt"
 "image"
 _ "image/png"
 "io"
 "os/exec"
 "regexp"
 "strings"
 "time"
)

// A bounded static SVG is rasterized to PNG before entering either LCD's
// existing image store. No SVG markup is served to clients or hardware.
const maxIconSetSVGBytes=512*1024
var internalSVGReference=regexp.MustCompile(`url\(\s*['"]?#[A-Za-z0-9_-]+['"]?\s*\)`)
func svgConverterAvailable()bool{_,err:=exec.LookPath("rsvg-convert");return err==nil}

func validateIconSVG(data []byte)error{
 if len(data)==0||len(data)>maxIconSetSVGBytes{return errors.New("SVG exceeds 512 KiB")}
 dec:=xml.NewDecoder(bytes.NewReader(data))
 depth,count:=0,0
 for{
  token,err:=dec.Token()
  if err==io.EOF{break}
  if err!=nil{return fmt.Errorf("invalid SVG XML: %w",err)}
  switch t:=token.(type){
  case xml.Directive:
   return errors.New("SVG DOCTYPE/entities are forbidden")
  case xml.ProcInst:
   if strings.ToLower(t.Target)!="xml"{return errors.New("SVG processing instructions are forbidden")}
  case xml.StartElement:
   depth++;count++
   if depth>24||count>2048||(depth==1&&t.Name.Local!="svg"){return errors.New("invalid or overly complex SVG")}
   switch t.Name.Local{
   case "svg","g","path","circle","ellipse","rect","line","polyline","polygon","defs","linearGradient","radialGradient","stop","clipPath":
   default:return fmt.Errorf("unsupported SVG element %q",t.Name.Local)
   }
   if len(t.Attr)>64{return errors.New("too many SVG attributes")}
   for _,attr:=range t.Attr{
    name:=strings.ToLower(attr.Name.Local)
    value:=strings.ToLower(strings.TrimSpace(attr.Value))
    if name=="href"||name=="base"||strings.HasPrefix(name,"on")||
     strings.Contains(value,"@import")||strings.Contains(value,"javascript:")||
     strings.Contains(value,"data:")||strings.Contains(value,"expression("){
     return errors.New("SVG external links or active content forbidden")
    }
    if strings.Contains(value,"url(")&&strings.Contains(internalSVGReference.ReplaceAllString(value,""),"url("){
     return errors.New("SVG external resources forbidden")
    }
   }
  case xml.EndElement:depth--
  case xml.CharData:
   if len(t)>8192&&len(strings.TrimSpace(string(t)))>0{return errors.New("oversized SVG text")}
  }
 }
 if depth!=0||count==0{return errors.New("empty or unbalanced SVG")}
 return nil
}
func rasterizeIconSVG(data []byte)([]byte,error){
 if err:=validateIconSVG(data);err!=nil{return nil,err}
 binary,err:=exec.LookPath("rsvg-convert")
 if err!=nil{return nil,errors.New("SVG import requires librsvg2-bin (rsvg-convert)")}
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Second)
 defer cancel()
 cmd:=exec.CommandContext(ctx,binary,"--format=png","--width=128","--height=128")
 cmd.Stdin=bytes.NewReader(data)
 png,err:=cmd.Output()
 if ctx.Err()!=nil{return nil,errors.New("SVG conversion timed out")}
 if err!=nil{return nil,fmt.Errorf("SVG conversion failed: %w",err)}
 if len(png)==0||len(png)>512*1024{return nil,errors.New("invalid rendered SVG size")}
 size,format,err:=image.DecodeConfig(bytes.NewReader(png))
 if err!=nil||format!="png"||size.Width<1||size.Width>128||size.Height<1||size.Height>128{
  return nil,errors.New("SVG did not produce bounded PNG")
 }
 return png,nil
}

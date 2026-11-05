package deck

import (
 "bytes"
 "errors"
 "fmt"
 "image"
 "image/jpeg"
 "io"
 "golang.org/x/image/draw"
)

// The Mirabox wire format is based on 4ndv/mirajazz (MPL-2.0) and
// 4ndv/opendeck-akp153. Protocol versions 1 and 3 differ in packet size,
// key-release semantics and some screen-clearing behavior.
type outputReporter interface { SendOutputReport([]byte)(int,error) }

var (
 errUnsupportedModel=errors.New("not a supported Mirabox device")
 errInvalidKey=errors.New("key out of range")
)
var miraboxMapping=[18]int{4,10,16,3,9,15,2,8,14,1,7,13,0,6,12,5,11,17}

func writeMiraboxReport(writer outputReporter, m Model, payload []byte)error{
 if m.Family!="mirabox" || (m.Protocol!=1 && m.Protocol!=3){return errUnsupportedModel}
 size:=512
 if m.Protocol==3 {size=1024}
 if len(payload)>size+1 || len(payload)==0 || payload[0]!=0 {return errors.New("invalid Mirabox report")}
 report:=make([]byte,size+1)
 copy(report,payload)
 n,err:=writer.SendOutputReport(report)
 if err!=nil{return err}
 if n!=len(report){return io.ErrShortWrite}
 return nil
}

func miraboxCommand(op string, suffix ...byte)[]byte {
 data:=append([]byte{0,0x43,0x52,0x54,0,0},[]byte(op)...)
 return append(data,suffix...)
}

func MiraboxInitialize(writer outputReporter,m Model)error{
 if err:=writeMiraboxReport(writer,m,miraboxCommand("DIS"));err!=nil{return err}
 return writeMiraboxReport(writer,m,miraboxCommand("LIG",0,0,0,0))
}

func MiraboxBrightness(writer outputReporter,m Model,percent uint8)error{
 if percent>100{return errors.New("brightness must be 0-100")}
 return writeMiraboxReport(writer,m,miraboxCommand("LIG",0,0,percent))
}

// MiraboxKeyImage prepares a JPEG image using Mirajazz's 90-degree rotation
// and dual-mirror transform. This is equivalent to a 90-degree CCW rotation
// on the square key image.
func MiraboxKeyImage(m Model,key int,src image.Image)([]byte,error){
 if m.Family!="mirabox" {return nil,errUnsupportedModel}
 if key<0||key>=m.Keys() {return nil,errInvalidKey}
 if src==nil{return nil,errors.New("missing key image")}
 size:=m.NativeImagePixels(key)
 resized:=image.NewRGBA(image.Rect(0,0,size,size))
 draw.CatmullRom.Scale(resized,resized.Bounds(),src,src.Bounds(),draw.Over,nil)
 converted:=image.NewRGBA(resized.Bounds())
 for y:=0;y<size;y++{for x:=0;x<size;x++{converted.Set(x,y,resized.At(size-1-y,x))}}
 var buf bytes.Buffer
 if err:=jpeg.Encode(&buf,converted,&jpeg.Options{Quality:90});err!=nil{return nil,err}
 return buf.Bytes(),nil
}

func MiraboxSetKeyJPEG(writer outputReporter,m Model,key int,jpg []byte)error{
 if m.Family!="mirabox" {return errUnsupportedModel}
 if key<0||key>=m.Keys(){return errInvalidKey}
 if len(jpg)==0 || len(jpg)>65535 || !bytes.HasPrefix(jpg,[]byte{0xff,0xd8}) {
  return errors.New("invalid JPEG data")
 }
 header:=miraboxCommand("BAT",0,0,byte(len(jpg)>>8),byte(len(jpg)),byte(key+1))
 if err:=writeMiraboxReport(writer,m,header);err!=nil{return fmt.Errorf("image header: %w",err)}
 chunk:=512
 if m.Protocol==3 {chunk=1024}
 for start:=0;start<len(jpg);start+=chunk {
  end:=start+chunk
  if end>len(jpg){end=len(jpg)}
  payload:=append([]byte{0},jpg[start:end]...)
  if err:=writeMiraboxReport(writer,m,payload);err!=nil{return fmt.Errorf("image body: %w",err)}
 }
 return writeMiraboxReport(writer,m,miraboxCommand("STP"))
}

// MiraboxKeyEvent decodes a single key event. V1 hardware reports presses
// only; it cannot reliably detect releases or long-press duration.
func MiraboxKeyEvent(m Model,report []byte)(key int,pressed bool,ok bool){
 if m.Family!="mirabox" || len(report)<11 || !bytes.Equal(report[:3],[]byte{'A','C','K'}) {return 0,false,false}
 raw:=int(report[9])
 if raw<1||raw>len(miraboxMapping){return 0,false,false}
 key=miraboxMapping[raw-1]
 if key>=m.Keys(){return 0,false,false}
 if m.Protocol==1{return key,true,true}
 return key,report[10]!=0,true
}

package deck

import (
 "crypto/sha256"
 "errors"
 "fmt"
 "image"
 "image/color"
 imagedraw "image/draw"
 "strings"
 "sync"
 "time"

 "OpenLinkHub/src/common"
 "OpenLinkHub/src/config"
 "OpenLinkHub/src/logger"
 "github.com/sstallion/go-hid"
 "golang.org/x/image/font"
 "golang.org/x/image/font/basicfont"
 "golang.org/x/image/math/fixed"
)

// Device is the first native Deck implementation. Mirabox gets HID input and
// static key-image output; Elgato is discovery-only until its protocol driver
// is implemented. No third-party code executes inside the system daemon.
type Device struct {
 Product string
 Serial string
 Manufacturer string
 Firmware string
 Model Model
 Template string

 mu sync.RWMutex
 hidDevice *hid.Device
 root string
 layout Layout
 status string
 connected bool
 pressedKey int
 eventCount uint64
 initialized bool
 done chan struct{}
 once sync.Once
}

type Status struct {
 Connected bool `json:"connected"`
 Status string `json:"status"`
 LastPressedKey int `json:"lastPressedKey"`
 ButtonEvents uint64 `json:"buttonEvents"`
 KeyOutputSupported bool `json:"keyOutputSupported"`
}

func deviceID(info *hid.DeviceInfo,m Model)string {
 serial:=info.SerialNbr
 if m.Protocol==1 {serial="legacy-fixed-serial"}
 if serial=="" {serial=info.Path}
 digest:=sha256.Sum256([]byte(serial))
 return fmt.Sprintf("deck-%04x-%04x-%x",m.VendorID,m.ProductID,digest[:8])
}

func Init(info *hid.DeviceInfo)*common.Device {
 if info==nil{return nil}
 m,ok:=LookupModel(info.VendorID,info.ProductID)
 if !ok{return nil}
 id:=deviceID(info,m)
 d:=&Device{
  Product:m.Name,Serial:id,Manufacturer:m.Family,Model:m,
  Template:"deck.html",root:config.GetConfig().ConfigPath,
  pressedKey:-1,done:make(chan struct{}),
  status:"Detected; native Elgato driver not implemented yet",
 }
 l,err:=LoadLayout(d.root,id,m)
 if err!=nil{
  logger.Log(logger.Fields{"error":err,"serial":id}).Warn("Unable to load Deck layout; using defaults")
  l=DefaultLayout(m)
 }
 d.layout=l
 if m.Family=="mirabox" {
  handle,err:=hid.OpenPath(info.Path)
  if err!=nil{
   logger.Log(logger.Fields{"error":err,"model":m.Name,"path":info.Path}).Warn("Unable to open Deck HID device")
   return nil
  }
  d.hidDevice=handle
  d.connected=true
  d.status="Connected; static images and key presses available"
  go d.listen()
 }
 return &common.Device{
  ProductType:800,Product:m.Name,Serial:id,Firmware:"",
  DeviceType:common.DeviceTypeKeypad,ProductId:m.ProductID,
  DevicePath:info.Path,DeviceLookup:id,Image:"icon-device.svg",Instance:d,
 }
}

func(d *Device)GetDeviceTemplate()string{return d.Template}
func(d *Device)Stop(){
 d.once.Do(func(){
  close(d.done)
  d.mu.Lock()
  defer d.mu.Unlock()
  d.connected=false
  d.status="Stopped"
  if d.hidDevice!=nil { _=d.hidDevice.Close();d.hidDevice=nil }
 })
}
func(d *Device)StopDirty()uint8{d.Stop();return 2}

func(d *Device)GetStatus()Status{
 d.mu.RLock()
 defer d.mu.RUnlock()
 return Status{Connected:d.connected,Status:d.status,LastPressedKey:d.pressedKey,
  ButtonEvents:d.eventCount,KeyOutputSupported:d.hidDevice!=nil && d.Model.Family=="mirabox"}
}

func cloneLayout(l Layout)Layout{
 out:=l
 out.Pages=append([]Page(nil),l.Pages...)
 for i:=range out.Pages {out.Pages[i].Keys=append([]KeyConfig(nil),l.Pages[i].Keys...)}
 return out
}
func(d *Device)GetLayout()Layout{
 d.mu.RLock()
 defer d.mu.RUnlock()
 return cloneLayout(d.layout)
}
func(d *Device)SetLayout(layout Layout)error{
 if err:=layout.Validate(d.Model);err!=nil{return err}
 d.mu.Lock()
 defer d.mu.Unlock()
 if err:=SaveLayout(d.root,d.Serial,d.Model,layout);err!=nil{return err}
 d.layout=cloneLayout(layout)
 if d.Model.Family=="mirabox" && d.hidDevice!=nil {
  if err:=d.drawActivePageLocked();err!=nil{
   d.status="HID image transfer failed: "+err.Error()
   return err
  }
 }
 return nil
}

func(d *Device)drawActivePageLocked()error{
 if d.hidDevice==nil{return errors.New("Deck not connected")}
 if !d.initialized {
  if err:=MiraboxInitialize(d.hidDevice,d.Model);err!=nil{return err}
  d.initialized=true
 }
 var page *Page
 for i:=range d.layout.Pages {if d.layout.Pages[i].ID==d.layout.ActivePage{page=&d.layout.Pages[i];break}}
 if page==nil{return errors.New("no active page")}
 labels:=make(map[int]string,len(page.Keys))
 for _,key:=range page.Keys{labels[key.Index]=key.Label}
 for i:=0;i<d.Model.Keys();i++ {
  size:=d.Model.NativeImagePixels(i)
  frame:=renderKeyLabel(size,labels[i])
  jpegData,err:=MiraboxKeyImage(d.Model,i,frame)
  if err!=nil{return err}
  if err:=MiraboxSetKeyJPEG(d.hidDevice,d.Model,i,jpegData);err!=nil{return err}
 }
 return nil
}
func renderKeyLabel(size int,label string)image.Image{
 img:=image.NewRGBA(image.Rect(0,0,size,size))
 imagedraw.Draw(img,img.Bounds(),image.NewUniform(color.RGBA{R:22,G:38,B:57,A:255}),image.Point{},imagedraw.Src)
 if label=="" {return img}
 text:=[]rune(strings.TrimSpace(label))
 // The first implementation uses a small built-in font and truncates to fit.
 for len(text)>0 && font.MeasureString(basicfont.Face7x13,string(text)).Ceil()>size-8 {
  text=text[:len(text)-1]
 }
 if len(text)==0{return img}
 w:=font.MeasureString(basicfont.Face7x13,string(text)).Ceil()
 drawer:=font.Drawer{Dst:img,Src:image.NewUniform(color.White),
  Face:basicfont.Face7x13,Dot:fixed.P((size-w)/2,(size+13)/2)}
 drawer.DrawString(string(text))
 return img
}
func(d *Device)listen(){
 buffer:=make([]byte,512)
 for {
  select{case <-d.done:return;default:}
  d.mu.RLock()
  handle:=d.hidDevice
  d.mu.RUnlock()
  if handle==nil{return}
  n,err:=handle.ReadWithTimeout(buffer,250*time.Millisecond)
  if errors.Is(err,hid.ErrTimeout){continue}
  if err!=nil{
   d.mu.Lock()
   d.connected=false
   d.status="Disconnected: "+err.Error()
   d.mu.Unlock()
   return
  }
  key,pressed,ok:=MiraboxKeyEvent(d.Model,buffer[:n])
  if !ok||!pressed{continue}
  d.mu.Lock()
  d.eventCount++
  d.pressedKey=key
  d.mu.Unlock()
 }
}

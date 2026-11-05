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
 "OpenLinkHub/src/devices/nexus"
 "OpenLinkHub/src/logger"
 "github.com/sstallion/go-hid"
 "golang.org/x/image/font"
 "golang.org/x/image/font/basicfont"
 "golang.org/x/image/math/fixed"
)

// Device is the first native Deck implementation. Mirabox gets HID input and
// static key-image output; Elgato is discovery-only until its protocol driver
// is implemented. No third-party code executes inside the system daemon.
// hidSession isolates the HID handle from reconnect and protocol tests.
type hidSession interface {
 Write([]byte)(int,error)
 ReadWithTimeout([]byte,time.Duration)(int,error)
 Close()error
}

type Device struct {
 Product string
 Serial string
 Manufacturer string
 Firmware string
 Model Model
 Template string

 mu sync.RWMutex
 hidDevice hidSession
 root string
 layout Layout
 status string
 connected bool
 pressedKey int
 eventCount uint64
 initialized bool
 macroRunning bool
 // Used only by tests; nil uses the real shared Nexus input backend.
 inputActionRunner func(string,string)error
 done chan struct{}
 readerDone chan struct{}
 once sync.Once
}

type Status struct {
 Connected bool `json:"connected"`
 Status string `json:"status"`
 LastPressedKey int `json:"lastPressedKey"`
 ButtonEvents uint64 `json:"buttonEvents"`
 ActivePage string `json:"activePage"`
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
  // The desktop launch agent is needed even when no Corsair Nexus
  // touchscreen is connected. A failed agent setup must never stop HID.
  if err:=nexus.InitializeDeckDesktopLauncher(d.root);err!=nil{
   logger.Log(logger.Fields{"error":err.Error()}).Warn("Deck desktop launcher unavailable")
  }
 }
 if m.Family=="mirabox" {
  handle,err:=hid.OpenPath(info.Path)
  if err!=nil{
   logger.Log(logger.Fields{"error":err,"model":m.Name,"path":info.Path}).Warn("Unable to open Deck HID device")
   return nil
  }
  // The vendor protocol requires initialization before any button
  // reports are read. Mirajazz also sets brightness before attaching its
  // input reader; opening the handle alone is not a ready device session.
  if err:=d.attachSession(handle);err!=nil{
   _=handle.Close()
   logger.Log(logger.Fields{"error":err,"model":m.Name,"path":info.Path}).Warn("Unable to initialize Deck HID session")
   return nil
  }
  d.readerDone=make(chan struct{})
  go d.listen()
 }
 return &common.Device{
  ProductType:common.ProductTypeDeck,Product:m.Name,Serial:id,Firmware:"",
  DeviceType:common.DeviceTypeKeypad,ProductId:m.ProductID,
  DevicePath:info.Path,DeviceLookup:id,Image:"icon-device.svg",Instance:d,
 }
}

func(d *Device)GetDeviceTemplate()string{return d.Template}
func(d *Device)Stop(){
 d.once.Do(func(){
  close(d.done)
  // Let the timed HID reader leave before closing its handle.
  if d.readerDone!=nil {
   select {case <-d.readerDone:case <-time.After(2*time.Second):}
  }
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
  ButtonEvents:d.eventCount,ActivePage:d.layout.ActivePage,
  KeyOutputSupported:d.hidDevice!=nil && d.Model.Family=="mirabox"}
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
// LayoutResult distinguishes saving configuration from actually applying it
// to hardware. A failed output report must never be reported as Save failed.
type LayoutResult struct {
 Saved bool `json:"saved"`
 Applied bool `json:"applied"`
 Message string `json:"message"`
}
func(d *Device)UpdateLayout(layout Layout)(LayoutResult,error){
 if err:=layout.Validate(d.Model);err!=nil{return LayoutResult{},err}
 d.mu.Lock()
 defer d.mu.Unlock()
 if err:=SaveLayout(d.root,d.Serial,d.Model,layout);err!=nil{return LayoutResult{},err}
 d.layout=cloneLayout(layout)
 result:=LayoutResult{Saved:true}
 if d.Model.Family!="mirabox"{
  result.Message="Saved. Elgato image output is not available yet."
  return result,nil
 }
 if d.hidDevice==nil || !d.connected {
  d.status="Disconnected; layout saved and queued for reconnect"
  result.Message="Saved locally; display will sync after reconnect."
  return result,nil
 }
 if err:=d.drawActivePageLocked();err!=nil{
  d.status="Connected; display update failed: "+err.Error()
  result.Message="Saved locally, but device display update failed: "+err.Error()
  return result,nil
 }
 d.status="Connected; layout applied to display"
 result.Applied=true
 result.Message="Saved and applied to Mirabox display."
 return result,nil
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
 for _,key:=range page.Keys{labels[key.Index]=keyDisplayLabel(key)}
 for i:=0;i<d.Model.Keys();i++ {
  size:=d.Model.NativeImagePixels(i)
  frame:=renderKeyLabel(size,labels[i])
  jpegData,err:=MiraboxKeyImage(d.Model,i,frame)
  if err!=nil{return err}
  if err:=MiraboxWriteKeyJPEG(d.hidDevice,d.Model,i,jpegData);err!=nil{return err}
 }
 return MiraboxFlush(d.hidDevice,d.Model)
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
// attachSession initializes a new handle and replays the saved layout.
// A display-write failure must not make a healthy HID input connection disappear.
func(d *Device)attachSession(handle hidSession)error{
 if err:=initializeMiraboxSession(handle,d.Model);err!=nil{return err}
 d.mu.Lock()
 defer d.mu.Unlock()
 select {case <-d.done:return errors.New("Deck driver stopping");default:}
 d.hidDevice=handle
 d.connected=true
 d.initialized=true
 if err:=d.drawActivePageLocked();err!=nil{
  d.status="Connected; display update failed: "+err.Error()
  logger.Log(logger.Fields{"error":err,"model":d.Model.Name}).Warn("Deck display image transfer failed")
 }else{
  d.status="Connected; layout applied to display"
 }
 return nil
}

// matchesReconnectCandidate filters interface and model identity. Stable
// Mirabox v1 IDs intentionally cannot distinguish two identical legacy units.
func(d *Device)matchesReconnectCandidate(info *hid.DeviceInfo)bool{
 return info!=nil && info.Path!="" &&
  info.VendorID==d.Model.VendorID && info.ProductID==d.Model.ProductID &&
  info.UsagePage==0xffa0 && info.Usage==1 &&
  deviceID(info,d.Model)==d.Serial
}

// findReconnectCandidate deliberately matches VID, PID, interface usage and
// logical device identity; the old /dev/hidraw path may no longer exist.
func(d *Device)findReconnectCandidate()(*hid.DeviceInfo,error){
 var found *hid.DeviceInfo
 err:=hid.Enumerate(d.Model.VendorID,d.Model.ProductID,hid.EnumFunc(func(info *hid.DeviceInfo)error{
  if !d.matchesReconnectCandidate(info){return nil}
  candidate:=*info
  found=&candidate
  return nil
 }))
 return found,err
}

func(d *Device)tryReconnect(){
 info,err:=d.findReconnectCandidate()
 if err!=nil{
  logger.Log(logger.Fields{"error":err,"serial":d.Serial}).Warn("Unable to enumerate Deck HID")
  return
 }
 if info==nil{return}
 handle,err:=hid.OpenPath(info.Path)
 if err!=nil{return} // A competing application may own the device.
 if err:=d.attachSession(handle);err!=nil{
  _=handle.Close()
  d.mu.Lock()
  d.status="Disconnected; reconnect initialization failed: "+err.Error()
  d.mu.Unlock()
 }
}

// A single reader owns all reconnect attempts, avoiding multiple readers and
// ensuring that a removed device's handle is closed before opening the new one.
func(d *Device)listen(){
 defer close(d.readerDone)
 buffer:=make([]byte,512)
 for {
  select{case <-d.done:return;default:}
  d.mu.RLock()
  handle:=d.hidDevice
  d.mu.RUnlock()
  if handle==nil{
   select{
   case <-d.done:return
   case <-time.After(1500*time.Millisecond):
   }
   d.tryReconnect()
   continue
  }
  n,err:=handle.ReadWithTimeout(buffer,250*time.Millisecond)
  if errors.Is(err,hid.ErrTimeout){continue}
  if err!=nil{
   d.mu.Lock()
   if d.hidDevice==handle{
    d.hidDevice=nil
    d.connected=false
    d.initialized=false
    d.status="Disconnected; waiting for Mirabox to reconnect"
   }
   d.mu.Unlock()
   _=handle.Close()
   logger.Log(logger.Fields{"error":err,"serial":d.Serial}).Warn("Deck HID disconnected; retrying")
   continue
  }
  if n<=0{continue}
  key,pressed,ok:=MiraboxKeyEvent(d.Model,buffer[:n])
  if !ok||!pressed{continue}
  d.handleKeyPress(key)
 }
}

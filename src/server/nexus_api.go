package server

import (
 "OpenLinkHub/src/devices"
 "OpenLinkHub/src/devices/nexus"
 "encoding/json"
 "io"
 "net/http"
 "net/url"
 "strings"
)

func nexusDevice(r *http.Request)(*nexus.Device,bool){
 serial:=r.URL.Query().Get("serial")
 if serial=="" {return nil,false}
 d,ok:=devices.GetDevice(serial).(*nexus.Device)
 return d,ok
}
func nexusWriteAllowed(r *http.Request)bool{
 if r.Header.Get("Origin")!="" {
  origin,err:=url.Parse(r.Header.Get("Origin"))
  if err!=nil||!strings.EqualFold(origin.Host,r.Host){return false}
 }
 return true
}
func nexusScreensAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet&&r.Method!=http.MethodPut {w.Header().Set("Allow","GET, PUT");http.Error(w,"method not allowed",405);return}
 d,ok:=nexusDevice(r);if !ok {http.Error(w,"Nexus device not found",404);return}
 if r.Method==http.MethodGet {
  w.Header().Set("Content-Type","application/json")
  _=json.NewEncoder(w).Encode(d.GetCustomScreens())
  return
 }
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",403);return}
 if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json"){http.Error(w,"JSON required",415);return}
 r.Body=http.MaxBytesReader(w,r.Body,1024*1024)
 defer r.Body.Close()
 var cfg nexus.ScreenConfiguration
 dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields()
 if err:=dec.Decode(&cfg);err!=nil{http.Error(w,"invalid JSON: "+err.Error(),400);return}
 if err:=dec.Decode(new(any));err!=io.EOF{http.Error(w,"unexpected trailing JSON",400);return}
 if err:=d.SetCustomScreens(cfg);err!=nil{http.Error(w,err.Error(),400);return}
 w.WriteHeader(http.StatusNoContent)
}
func nexusBackgroundAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost {w.Header().Set("Allow","POST");http.Error(w,"method not allowed",405);return}
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",403);return}
 d,ok:=nexusDevice(r);if !ok {http.Error(w,"Nexus device not found",404);return}
 id:=r.URL.Query().Get("id")
 r.Body=http.MaxBytesReader(w,r.Body,4*1024*1024)
 defer r.Body.Close()
 content,err:=io.ReadAll(r.Body)
 if err!=nil{http.Error(w,"image too large or unreadable",400);return}
 if _,err=d.SaveCustomBackground(id,content);err!=nil{http.Error(w,err.Error(),400);return}
 w.Header().Set("Content-Type","application/json")
 _=json.NewEncoder(w).Encode(map[string]string{"id":id})
}

func nexusCustomIconAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost && r.Method!=http.MethodGet {
  w.Header().Set("Allow","GET, POST");http.Error(w,"method not allowed",405);return
 }
 d,ok:=nexusDevice(r);if !ok {http.Error(w,"Nexus device not found",404);return}
 id:=r.URL.Query().Get("id")
 if r.Method==http.MethodGet {
  path,err:=d.CustomIconPath(id)
  if err!=nil{http.Error(w,"icon not found",404);return}
  w.Header().Set("Content-Type","image/png")
  w.Header().Set("Cache-Control","private, no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  http.ServeFile(w,r,path)
  return
 }
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",403);return}
 if !strings.HasPrefix(r.Header.Get("Content-Type"),"image/png") && !strings.HasPrefix(r.Header.Get("Content-Type"),"image/jpeg"){http.Error(w,"PNG or JPEG required",415);return}
 r.Body=http.MaxBytesReader(w,r.Body,512*1024)
 defer r.Body.Close()
 content,err:=io.ReadAll(r.Body)
 if err!=nil{http.Error(w,"icon too large or unreadable",400);return}
 if _,err:=d.SaveCustomIcon(id,content);err!=nil{http.Error(w,err.Error(),400);return}
 w.Header().Set("Content-Type","application/json")
 _=json.NewEncoder(w).Encode(map[string]string{"id":id})
}

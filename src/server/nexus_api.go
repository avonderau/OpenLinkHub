package server

import (
 "OpenLinkHub/src/devices"
 "OpenLinkHub/src/devices/nexus"
 "OpenLinkHub/src/config"
 "OpenLinkHub/src/dashboard"
 "encoding/json"
 "io"
 "mime"
 "net"
 "net/http"
 "net/url"
 "strings"
)

// Bind loopback-only Nexus APIs to loopback Host headers, preventing DNS
// rebinding against an unauthenticated local WebUI. The operator may still
// configure an authenticated HTTPS Origin for a reverse proxy.
func nexusHostAllowed(listenAddress,requestHost,authOrigin string)bool{
 listenIP:=net.ParseIP(listenAddress)
 if !strings.EqualFold(listenAddress,"localhost") && (listenIP==nil||!listenIP.IsLoopback()){
  return true
 }
 host:=requestHost
 if h,_,err:=net.SplitHostPort(requestHost);err==nil{host=h}
 if strings.ContainsAny(host,"@/\\")||host==""{return false}
 if strings.EqualFold(host,"localhost"){return true}
 if ip:=net.ParseIP(host);ip!=nil&&ip.IsLoopback(){return true}
 if authOrigin!="" {
  origin,err:=url.Parse(authOrigin)
  if err==nil&&origin.Scheme=="https"&&origin.Host!=""&&
   strings.EqualFold(origin.Host,requestHost)&&origin.User==nil&&
   origin.Path==""&&origin.RawQuery==""&&origin.Fragment==""{
   return true
  }
 }
 return false
}
func nexusRequestHostAllowed(r *http.Request)bool{
 authOrigin:=""
 dashboardSettings:=dashboard.GetDashboard()
 if dashboardSettings.AuthEnabled{authOrigin=dashboardSettings.AuthOrigin}
 return nexusHostAllowed(config.GetConfig().ListenAddress,r.Host,authOrigin)
}

func nexusDevice(r *http.Request)(*nexus.Device,bool){
 if !nexusRequestHostAllowed(r){return nil,false}
 serial:=r.URL.Query().Get("serial")
 if serial=="" {return nil,false}
 d,ok:=devices.GetDevice(serial).(*nexus.Device)
 return d,ok
}
// Require same-origin browser writes. A missing Origin is still accepted for
// direct local CLI/API clients, but browser Fetch Metadata never permits
// cross-site/same-site subdomain writes without an exact origin match.
func nexusWriteAllowed(r *http.Request)bool{
 if !nexusRequestHostAllowed(r){return false}
 switch r.Header.Get("Sec-Fetch-Site"){
 case "","none","same-origin":
 default:return false
 }
 raw:=r.Header.Get("Origin")
 if raw==""{return true}
 origin,err:=url.Parse(raw)
 if err!=nil||origin.User!=nil||origin.Opaque!=""||
  origin.Path!=""||origin.RawQuery!=""||origin.Fragment!=""||
  (origin.Scheme!="http"&&origin.Scheme!="https")||
  origin.Host==""||!strings.EqualFold(origin.Host,r.Host){
  return false
 }
 if r.TLS!=nil&&origin.Scheme!="https"{return false}
 return true
}
func nexusUploadMediaType(value string)bool{
 mediaType,_,err:=mime.ParseMediaType(value)
 if err!=nil{return false}
 return mediaType=="image/png"||mediaType=="image/jpeg"||mediaType=="image/gif"
}
func nexusScreensAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet&&r.Method!=http.MethodPut {w.Header().Set("Allow","GET, PUT");http.Error(w,"method not allowed",405);return}
 d,ok:=nexusDevice(r);if !ok {http.Error(w,"Nexus device not found",404);return}
 if r.Method==http.MethodGet {
  w.Header().Set("Content-Type","application/json")
  w.Header().Set("Cache-Control","private, no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  _=json.NewEncoder(w).Encode(d.GetCustomScreens())
  return
 }
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",403);return}
 if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json"){http.Error(w,"JSON required",415);return}
 r.Body=http.MaxBytesReader(w,r.Body,nexus.MaxScreenConfigBytes)
 defer r.Body.Close()
 var cfg nexus.ScreenConfiguration
 dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields()
 if err:=dec.Decode(&cfg);err!=nil{http.Error(w,"invalid JSON: "+err.Error(),400);return}
 if err:=dec.Decode(new(any));err!=io.EOF{http.Error(w,"unexpected trailing JSON",400);return}
 if err:=d.SetCustomScreens(cfg);err!=nil{http.Error(w,err.Error(),400);return}
 w.WriteHeader(http.StatusNoContent)
}
// nexusTelemetryAPI serves cached live graph history. This endpoint is
// read-only and never triggers a new device telemetry collection.
func nexusTelemetryAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{w.Header().Set("Allow","GET");http.Error(w,"method not allowed",405);return}
 d,ok:=nexusDevice(r);if !ok{http.Error(w,"Nexus device not found",404);return}
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","private, no-store")
 w.Header().Set("X-Content-Type-Options","nosniff")
 _=json.NewEncoder(w).Encode(d.GetNexusTelemetryPreview())
}
func nexusBackgroundAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost && r.Method!=http.MethodGet {
  w.Header().Set("Allow","GET, POST");http.Error(w,"method not allowed",405);return
 }
 d,ok:=nexusDevice(r);if !ok {http.Error(w,"Nexus device not found",404);return}
 id:=r.URL.Query().Get("id")
 if r.Method==http.MethodGet {
  path,err:=d.CustomBackgroundPath(id)
  if err!=nil{http.Error(w,"background not found",404);return}
  w.Header().Set("Cache-Control","private, no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  http.ServeFile(w,r,path)
  return
 }
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",403);return}
 if !nexusUploadMediaType(r.Header.Get("Content-Type")){
  http.Error(w,"PNG, JPEG or GIF required",http.StatusUnsupportedMediaType);return
 }
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
  w.Header().Set("Cache-Control","private, no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  http.ServeFile(w,r,path)
  return
 }
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",403);return}
 if !nexusUploadMediaType(r.Header.Get("Content-Type")){
  http.Error(w,"PNG, JPEG or GIF required",http.StatusUnsupportedMediaType);return
 }
 r.Body=http.MaxBytesReader(w,r.Body,512*1024)
 defer r.Body.Close()
 content,err:=io.ReadAll(r.Body)
 if err!=nil{http.Error(w,"icon too large or unreadable",400);return}
 if _,err:=d.SaveCustomIcon(id,content);err!=nil{http.Error(w,err.Error(),400);return}
 w.Header().Set("Content-Type","application/json")
 _=json.NewEncoder(w).Encode(map[string]string{"id":id})
}

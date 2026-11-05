package server

import (
 "encoding/json"
 "io"
 "mime"
 "net/http"

 "OpenLinkHub/src/devices"
 "OpenLinkHub/src/devices/deck"
)

func deckDevice(r *http.Request)(*deck.Device,bool){
 if !nexusRequestHostAllowed(r){return nil,false}
 id:=r.URL.Query().Get("serial")
 if id==""{return nil,false}
 d,ok:=devices.GetDevice(id).(*deck.Device)
 return d,ok
}

func deckStatusAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{
  w.Header().Set("Allow","GET")
  http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
  return
 }
 d,ok:=deckDevice(r)
 if !ok{http.Error(w,"Deck device not found",http.StatusNotFound);return}
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","private, no-store")
 w.Header().Set("X-Content-Type-Options","nosniff")
 _=json.NewEncoder(w).Encode(d.GetStatus())
}

func deckLayoutAPI(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet && r.Method!=http.MethodPut {
  w.Header().Set("Allow","GET, PUT")
  http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
  return
 }
 d,ok:=deckDevice(r)
 if !ok{http.Error(w,"Deck device not found",http.StatusNotFound);return}
 if r.Method==http.MethodGet{
  w.Header().Set("Content-Type","application/json")
  w.Header().Set("Cache-Control","private, no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  _=json.NewEncoder(w).Encode(d.GetLayout())
  return
 }
 if !nexusWriteAllowed(r){http.Error(w,"cross-origin write forbidden",http.StatusForbidden);return}
 mediaType,_,err:=mime.ParseMediaType(r.Header.Get("Content-Type"))
 if err!=nil||mediaType!="application/json"{http.Error(w,"JSON required",http.StatusUnsupportedMediaType);return}
 r.Body=http.MaxBytesReader(w,r.Body,deck.MaxDeckLayoutBytes)
 defer r.Body.Close()
 var layout deck.Layout
 dec:=json.NewDecoder(r.Body)
 dec.DisallowUnknownFields()
 if err:=dec.Decode(&layout);err!=nil{http.Error(w,"invalid layout: "+err.Error(),http.StatusBadRequest);return}
 if err:=dec.Decode(new(any));err!=io.EOF{http.Error(w,"unexpected trailing JSON",http.StatusBadRequest);return}
 result,err:=d.UpdateLayout(layout)
 if err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return}
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","private, no-store")
 w.Header().Set("X-Content-Type-Options","nosniff")
 _=json.NewEncoder(w).Encode(result)
}

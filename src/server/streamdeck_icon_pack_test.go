package server

import (
 "archive/zip"
 "bytes"
 "encoding/json"
 "image"
 "image/color"
 "image/png"
 "strings"
 "testing"
)

func makeStreamDeckZIP(t *testing.T, manifest map[string]any, index []map[string]any, overrides map[string][]byte) []byte {
 t.Helper()
 var raw bytes.Buffer
 writer:=zip.NewWriter(&raw)
 makeJSON:=func(v any) []byte {data,err:=json.Marshal(v);if err!=nil{t.Fatal(err)};return data}
 files:=map[string][]byte{"manifest.json":makeJSON(manifest),"icons.json":makeJSON(index)}
 frame:=image.NewRGBA(image.Rect(0,0,16,16))
 frame.Set(4,4,color.RGBA{R:255,A:255})
 var picture bytes.Buffer
 if err:=png.Encode(&picture,frame);err!=nil{t.Fatal(err)}
 files["icon.png"]=picture.Bytes()
 files["icons/play.png"]=picture.Bytes()
 for name,contents:=range overrides{files[name]=contents}
 for name,contents:=range files{
  file,err:=writer.Create(name);if err!=nil{t.Fatal(err)}
  if _,err=file.Write(contents);err!=nil{t.Fatal(err)}
 }
 if err:=writer.Close();err!=nil{t.Fatal(err)}
 return raw.Bytes()
}
func streamDeckFixture() (map[string]any,[]map[string]any) {
 return map[string]any{"Name":"Sample Pack","Author":"Artist","Version":"1.0.0","Icon":"icon.png","Description":"Example pack"},
 []map[string]any{{"path":"play.png","name":"Play","tags":[]string{"media","transport"}}}
}
func TestStreamDeckPackImportMetadata(t *testing.T){
 manifest,index:=streamDeckFixture()
 raw:=makeStreamDeckZIP(t,manifest,index,nil)
 imported,err:=importIconSet(t.TempDir(),"",raw)
 if err!=nil{t.Fatal(err)}
 if imported.Name!="Sample Pack" || imported.Source!="streamdeck" || imported.Author!="Artist" || imported.Version!="1.0.0" {t.Fatalf("metadata not retained: %+v",imported)}
 if len(imported.Icons)!=1 || imported.Icons[0].Name!="Play" || len(imported.Icons[0].Tags)!=2 {t.Fatalf("icon tags not retained: %+v",imported.Icons)}
 if imported.Icons[0].DeckID=="" || imported.Icons[0].NexusID=="" {t.Fatal("device icon IDs missing")}
}
func TestStreamDeckPackRejectsInvalidEntries(t *testing.T){
 cases:=[]struct{name string;edit func(map[string]any,[]map[string]any) (map[string]any,[]map[string]any,map[string][]byte)}{
  {"missing referenced icon",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){i[0]["path"]="gone.png";return m,i,nil}},
  {"traversal",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){i[0]["path"]="../play.png";return m,i,nil}},
  {"control name",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){i[0]["name"]="Play\nNow";return m,i,nil}},
  {"invalid tag",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){i[0]["tags"]=[]string{"bad\ttag"};return m,i,nil}},
  {"duplicate indexed icon",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){i=append(i,i[0]);return m,i,nil}},
  {"malformed manifest",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){return m,i,map[string][]byte{"manifest.json":[]byte("{")}}},
  {"unsafe archive member",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){return m,i,map[string][]byte{"../escape.png":[]byte("x")}}},
  {"missing artwork",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){m["Icon"]="missing.png";return m,i,nil}},
  {"unsupported format",func(m map[string]any,i []map[string]any)(map[string]any,[]map[string]any,map[string][]byte){i[0]["path"]="play.exe";return m,i,map[string][]byte{"icons/play.exe":[]byte("x")}}},
 }
 for _,tc:=range cases{
  t.Run(tc.name,func(t *testing.T){
   m,i:=streamDeckFixture();m,i,extra:=tc.edit(m,i)
   _,err:=importIconSet(t.TempDir(),"",makeStreamDeckZIP(t,m,i,extra))
   if err==nil{t.Fatal("invalid Stream Deck pack accepted")}
  })
 }
}
func TestStreamDeckStillFrameRejectsCorruptAnimation(t *testing.T){
 if _,err:=streamDeckStillFrame([]byte("not a gif"),".gif");err==nil{t.Fatal("bad GIF accepted")}
 if _,err:=streamDeckStillFrame([]byte("not webp"),".webp");err==nil{t.Fatal("bad WebP accepted")}
}
func TestStreamDeckPackCanImportAsCustomNamedSet(t *testing.T){
 m,i:=streamDeckFixture()
 p,err:=importIconSet(t.TempDir(),"My Collection",makeStreamDeckZIP(t,m,i,nil))
 if err!=nil{t.Fatal(err)}
 if p.Name!="My Collection"{t.Fatal("custom name ignored")}
}
func TestStreamDeckPathValidation(t *testing.T){
 for _,name:=range []string{"../x.png","icons/../x.png","/icons/x.png","icons\\x.png","icons//x.png",".hidden.png","icons/./x.png"}{
  if safeStreamDeckPath(name){t.Fatalf("unsafe path accepted %q",name)}
 }
 if !safeStreamDeckPath("icons/play.png"){t.Fatal("safe path rejected")}
}
func TestLegacyIconPackIsNotReclassified(t *testing.T){
 imported,err:=importIconSet(t.TempDir(),"Legacy",makeIconSetZIP(t,"media/rewind.png"))
 if err!=nil{t.Fatal(err)}
 if imported.Source!="" || len(imported.Icons)!=1 {t.Fatalf("legacy pack changed: %+v",imported)}
}
func TestStreamDeckMetadataLimits(t *testing.T){
 m,i:=streamDeckFixture();m["Name"]=strings.Repeat("X",61)
 if _,err:=importIconSet(t.TempDir(),"",makeStreamDeckZIP(t,m,i,nil));err==nil{t.Fatal("oversized metadata accepted")}
}

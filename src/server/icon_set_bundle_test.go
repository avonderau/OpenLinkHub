package server

import (
 "archive/zip"
 "bytes"
 "encoding/json"
 "fmt"
 "image"
 "image/color"
 "image/png"
 "testing"
)

func makeNestedStreamDeckBundle(t *testing.T) []byte {
 t.Helper()
 var buf bytes.Buffer
 writer:=zip.NewWriter(&buf)
 write:=func(name string,data []byte) {
  f,err:=writer.Create(name);if err!=nil{t.Fatal(err)}
  if _,err=f.Write(data);err!=nil{t.Fatal(err)}
 }
 frame:=image.NewRGBA(image.Rect(0,0,12,12))
 frame.Set(6,6,color.RGBA{R:255,A:255})
 var pngData bytes.Buffer
 if err:=png.Encode(&pngData,frame);err!=nil{t.Fatal(err)}
 for _,flavor:=range []string{"filled","regular"} {
  prefix:="download/fluent-"+flavor+".sdIconPack/"
  manifest:=map[string]any{
   "Name":"Fluent ("+flavor+")",
   "Author":"Test Author",
   "Version":"1.0",
   "Icon":"icons/cover.png",
  }
  index:=[]map[string]any{{"name":"Play","path":"play.png","tags":[]string{"media"}}}
  manifestJSON,_:=json.Marshal(manifest)
  indexJSON,_:=json.Marshal(index)
  write(prefix+"manifest.json",manifestJSON)
  write(prefix+"icons.json",indexJSON)
  write(prefix+"icons/cover.png",pngData.Bytes())
  write(prefix+"icons/play.png",pngData.Bytes())
 }
 write("download/README.md",[]byte("External metadata, not an icon"))
 if err:=writer.Close();err!=nil{t.Fatal(err)}
 return buf.Bytes()
}

func TestNestedStreamDeckBundleImportsBothPacks(t *testing.T) {
 packs,err:=importIconSetBundle(t.TempDir(),"",makeNestedStreamDeckBundle(t))
 if err!=nil{t.Fatal(err)}
 if len(packs)!=2 || packs[0].Name!="Fluent (filled)" || packs[1].Name!="Fluent (regular)" {
  t.Fatalf("unexpected imported packs: %+v",packs)
 }
 for _,pack:=range packs {
  if len(pack.Icons)!=1 || pack.Icons[0].Name!="Play" || pack.Source!="streamdeck" {
   t.Fatalf("incorrect nested pack: %+v",pack)
  }
 }
}

func TestNestedStreamDeckBundleRejectsUnsafeRoot(t *testing.T) {
 var buf bytes.Buffer
 zw:=zip.NewWriter(&buf)
 f,err:=zw.Create("../escape.zip");if err!=nil{t.Fatal(err)}
 if _,err=f.Write([]byte("wrong"));err!=nil{t.Fatal(err)}
 if err:=zw.Close();err!=nil{t.Fatal(err)}
 if _,err:=importIconSetBundle(t.TempDir(),"",buf.Bytes());err==nil{t.Fatal("unsafe bundle accepted")}
}

func TestLargeStreamDeckIndexCanBeParsed(t *testing.T) {
 raw:=capacityPackFixture(t,1914)
 reader,err:=zip.NewReader(bytes.NewReader(raw),int64(len(raw)))
 if err!=nil{t.Fatal(err)}
 pack,err:=parseStreamDeckPack(reader)
 if err!=nil{t.Fatal(err)}
 if len(pack.Icons)!=1914 {t.Fatalf("expected 1914, got %d",len(pack.Icons))}
}

func TestBundleLimitAllowsSampleArchiveEntries(t *testing.T) {
 if maxIconSetArchiveEntries<3843 {t.Fatalf("bundle fixture has 3843 entries; limit %d",maxIconSetArchiveEntries)}
 if maxIconSetImages<1914 {t.Fatalf("sample pack has 1914 icons; limit %d",maxIconSetImages)}
 _ = fmt.Sprintf
}

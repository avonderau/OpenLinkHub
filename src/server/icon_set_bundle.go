package server

import (
 "archive/zip"
 "bytes"
 "errors"
 "fmt"
 "io"
 "path"
 "sort"
 "strings"
)

// importIconSetBundle accepts either the historical single-pack ZIP or a
// distribution ZIP containing multiple *.sdIconPack directories.
// Each nested pack is installed as a separate library entry using its own
// manifest name, preserving the original icon set import pipeline.
func importIconSetBundle(root, name string, data []byte) ([]iconSetManifest,error) {
 if len(data)==0 || len(data)>maxIconSetZipBytes{return nil,errors.New("ZIP exceeds 32 MiB")}
 archive,err:=zip.NewReader(bytes.NewReader(data),int64(len(data)))
 if err!=nil{return nil,fmt.Errorf("invalid ZIP: %w",err)}
 if len(archive.File)>maxIconSetArchiveEntries {return nil,errors.New("too many archive entries")}
 var unpacked uint64
 for _,file:=range archive.File {
  if !safeBundlePath(file.Name) {return nil,fmt.Errorf("unsafe bundle path %q",file.Name)}
  if !file.FileInfo().IsDir() && !file.Mode().IsRegular() {return nil,errors.New("unsupported archive member type")}
  unpacked+=file.UncompressedSize64
  if unpacked>maxIconSetTotalImageBytes {return nil,errors.New("icon pack bundle exceeds extracted quota")}
 }
 prefixes:=map[string]bool{}
 for _,file:=range archive.File {
  if file.FileInfo().IsDir() || path.Base(file.Name)!="manifest.json" {continue}
  dir:=path.Dir(file.Name)
  if strings.HasSuffix(strings.ToLower(dir),".sdiconpack"){prefixes[dir+"/"]=true}
 }
 if len(prefixes)==0 {
  for _,member:=range archive.File {
   if member.Name=="manifest.json" {name="";break}
  }
  pack,err:=importIconSet(root,name,data)
  if err!=nil{return nil,err}
  return []iconSetManifest{pack},nil
 }
 if len(prefixes)>maxIconSets{return nil,errors.New("too many icon packs in ZIP")}
 sorted:=make([]string,0,len(prefixes))
 for prefix:=range prefixes{sorted=append(sorted,prefix)}
 sort.Strings(sorted)
 archives:=make([][]byte,0,len(sorted))
 for _,prefix:=range sorted {
  var buffer bytes.Buffer
  writer:=zip.NewWriter(&buffer)
  count:=0
  for _,file:=range archive.File {
   if file.FileInfo().IsDir() || !strings.HasPrefix(file.Name,prefix){continue}
   local:=strings.TrimPrefix(file.Name,prefix)
   if local=="" || strings.Contains(local,"/../") || !safeStreamDeckPath(local){return nil,errors.New("unsafe nested pack path")}
   // Strip only the enclosing .sdIconPack directory. This makes the archive
   // compatible with the existing parser and its metadata/path checks.
   if count>=maxIconSetArchiveEntries{return nil,errors.New("nested archive has too many entries")}
   if file.UncompressedSize64>1<<20{return nil,errors.New("oversized nested pack file")}
   entry,err:=writer.Create(local);if err!=nil{return nil,err}
   reader,err:=file.Open();if err!=nil{return nil,err}
   _,copyErr:=io.Copy(entry,io.LimitReader(reader,1<<20+1))
   closeErr:=reader.Close()
   if copyErr!=nil{return nil,copyErr}
   if closeErr!=nil{return nil,closeErr}
   count++
  }
  if err:=writer.Close();err!=nil{return nil,err}
  archives=append(archives,buffer.Bytes())
 }
 // Validate all subpacks before saving any icons to avoid partially importing
 // a bundle with an invalid later manifest.
 for _,compressed:=range archives {
  r,err:=zip.NewReader(bytes.NewReader(compressed),int64(len(compressed)))
  if err!=nil{return nil,err}
  if _,err:=parseStreamDeckPack(r);err!=nil{return nil,err}
 }
 result:=make([]iconSetManifest,0,len(archives))
 for _,compressed:=range archives {
  // Individual names come from their own manifests, rather than a ZIP name
  // or the optional Settings field used for legacy single-pack imports.
  pack,err:=importIconSet(root,"",compressed)
  if err!=nil{return nil,err}
  result=append(result,pack)
 }
 return result,nil
}

func safeBundlePath(p string) bool {
 if p=="" || strings.HasPrefix(p,"/") || strings.Contains(p,"\\") || strings.ContainsRune(p,0){return false}
 trimmed:=strings.TrimSuffix(p,"/")
 if trimmed=="" || path.Clean(trimmed)!=trimmed{return false}
 for _,seg:=range strings.Split(trimmed,"/"){
  if seg=="" || seg=="." || seg==".."{return false}
 }
 return true
}

package nexus

import (
 "bufio"
 "encoding/json"
 "net"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func TestNexusDesktopBrokerForwardsOnlyValidatedLocalLaunches(t *testing.T){
 path:=filepath.Join(t.TempDir(),"nexus","desktop-launch.sock")
 broker,err:=startNexusDesktopLaunchBroker(path)
 if err!=nil{t.Fatal(err)}
 defer broker.close()
 info,err:=os.Stat(path)
 if err!=nil{t.Fatal(err)}
 if info.Mode().Perm()!=0660{t.Fatalf("unsafe socket permissions: %v",info.Mode().Perm())}
 if err:=broker.send("url","https://example.com/first");err==nil||!strings.Contains(err.Error(),"no Nexus desktop agent"){
  t.Fatalf("expected actionable disconnected error: %v",err)
 }
 conn,err:=net.DialTimeout("unix",path,time.Second)
 if err!=nil{t.Fatal(err)}
 defer conn.Close()
 deadline:=time.After(time.Second)
 for{
  err=broker.send("url","https://example.com/path?q=test")
  if err==nil{break}
  select{case <-deadline:t.Fatalf("broker never accepted agent: %v",err);default:time.Sleep(5*time.Millisecond)}
 }
 if err:=conn.SetReadDeadline(time.Now().Add(time.Second));err!=nil{t.Fatal(err)}
 var event nexusDesktopLaunchEvent
 if err:=json.NewDecoder(bufio.NewReader(conn)).Decode(&event);err!=nil{t.Fatal(err)}
 if event.Action!="url"||event.Target!="https://example.com/path?q=test"{t.Fatalf("unexpected forwarded event: %+v",event)}
 if err:=broker.send("url","file:///etc/passwd");err==nil{t.Fatal("unsafe URL should be rejected")}
 if err:=broker.send("application","../../bin/sh");err==nil{t.Fatal("unsafe application should be rejected")}
}

func TestNexusDesktopBrokerRejectsWorldWritableAndExistingFiles(t *testing.T){
 dir:=filepath.Join(t.TempDir(),"nexus")
 if err:=os.MkdirAll(dir,0755);err!=nil{t.Fatal(err)}
 path:=filepath.Join(dir,"desktop-launch.sock")
 if err:=os.WriteFile(path,[]byte("not a socket"),0600);err!=nil{t.Fatal(err)}
 if _,err:=startNexusDesktopLaunchBroker(path);err==nil{t.Fatal("non-socket file unexpectedly removed")}
 data,err:=os.ReadFile(path)
 if err!=nil||string(data)!="not a socket"{t.Fatalf("existing file clobbered: %s %v",data,err)}
 if err:=os.Remove(path);err!=nil{t.Fatal(err)}
 if err:=os.Chmod(dir,0777);err!=nil{t.Fatal(err)}
 if _,err:=startNexusDesktopLaunchBroker(path);err==nil{t.Fatal("world-writable socket directory accepted")}
}

package nexus
import (
 "os"
 "strings"
)
// detectCounterDevices chooses the primary-route interface and a whole disk.
// Override with explicit configuration in a future device settings API.
func detectCounterDevices()(string,string){
 network:=""
 if b,err:=os.ReadFile("/proc/net/route");err==nil {
  for _,line:=range strings.Split(string(b),"\n"){
   fields:=strings.Fields(line)
   if len(fields)>=2&&fields[1]=="00000000"&&fields[0]!="Iface"{network=fields[0];break}
  }
 }
 if network=="" {
  if b,err:=os.ReadFile("/proc/net/dev");err==nil {
   for _,line:=range strings.Split(string(b),"\n"){
    parts:=strings.SplitN(line,":",2)
    if len(parts)==2 {
     name:=strings.TrimSpace(parts[0])
     if name!="lo"&&name!="" {network=name;break}
    }
   }
  }
 }
 disk:=""
 if b,err:=os.ReadFile("/proc/diskstats");err==nil {
  for _,line:=range strings.Split(string(b),"\n"){
   fields:=strings.Fields(line)
   if len(fields)<14 {continue}
   name:=fields[2]
   if strings.HasPrefix(name,"loop")||strings.HasPrefix(name,"ram")||strings.HasPrefix(name,"dm-") {continue}
   if strings.HasPrefix(name,"nvme") {
    if strings.HasSuffix(name,"p1")||strings.HasSuffix(name,"p2")||strings.HasSuffix(name,"p3") {continue}
    if strings.Contains(name,"p"){continue}
   } else if len(name)>0&&name[len(name)-1]>='0'&&name[len(name)-1]<='9' {continue}
   disk=name;break
  }
 }
 return network,disk
}

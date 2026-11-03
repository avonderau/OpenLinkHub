package nexus

import (
 "bufio"
 "fmt"
 "os"
 "strconv"
 "strings"
 "time"
)

// LinuxCounters represents cumulative kernel counters. Rates are calculated
// from consecutive samples, not from the values themselves.
type LinuxCounters struct {
 NetworkRX uint64
 NetworkTX uint64
 DiskReadSectors uint64
 DiskWriteSectors uint64
}
type LinuxRates struct {
 NetworkRXBytesPerSecond float64
 NetworkTXBytesPerSecond float64
 DiskReadBytesPerSecond float64
 DiskWriteBytesPerSecond float64
 Valid bool
}
type CounterSampler struct {
 history metricHistory
 snapshot MetricSnapshot
 snapshotAt time.Time
 previous LinuxCounters
 at time.Time
 initialized bool
}
// Cached returns a read-only, immutable metrics map when it is less than
// one second old. Animated GIFs may render five times per second without
// causing additional GPU command or sysfs reads.
func (s *CounterSampler) Cached(now time.Time)(MetricSnapshot,MetricHistory,bool){
 if s.snapshot==nil||s.snapshotAt.IsZero()||now.Before(s.snapshotAt)||now.Sub(s.snapshotAt)>=time.Second{
  return nil,nil,false
 }
 return s.snapshot,s.history.snapshot(),true
}

// Store updates the bounded 60-second graph history exactly once per sampled
// metrics snapshot. Callers synchronize CounterSampler with the device mutex.
func (s *CounterSampler) Store(now time.Time,values MetricSnapshot)MetricHistory{
 s.snapshot=values
 s.snapshotAt=now
 s.history.add(values,now)
 return s.history.snapshot()
}

func (s *CounterSampler) Sample(c LinuxCounters, now time.Time) LinuxRates {
 if !s.initialized {s.previous=c;s.at=now;s.initialized=true;return LinuxRates{}}
 seconds:=now.Sub(s.at).Seconds()
 old:=s.previous
 s.previous=c;s.at=now
 if seconds<=0||c.NetworkRX<old.NetworkRX||c.NetworkTX<old.NetworkTX||c.DiskReadSectors<old.DiskReadSectors||c.DiskWriteSectors<old.DiskWriteSectors{return LinuxRates{}}
 return LinuxRates{
  NetworkRXBytesPerSecond:float64(c.NetworkRX-old.NetworkRX)/seconds,
  NetworkTXBytesPerSecond:float64(c.NetworkTX-old.NetworkTX)/seconds,
  DiskReadBytesPerSecond:float64(c.DiskReadSectors-old.DiskReadSectors)*512/seconds,
  DiskWriteBytesPerSecond:float64(c.DiskWriteSectors-old.DiskWriteSectors)*512/seconds,
  Valid:true,
 }
}
func ParseNetworkCounters(contents string, interfaceName string)(uint64,uint64,error){
 scanner:=bufio.NewScanner(strings.NewReader(contents))
 for scanner.Scan(){
  parts:=strings.SplitN(scanner.Text(),":",2)
  if len(parts)!=2||strings.TrimSpace(parts[0])!=interfaceName{continue}
  f:=strings.Fields(parts[1]);if len(f)<16{return 0,0,fmt.Errorf("invalid network counters")}
  rx,e:=strconv.ParseUint(f[0],10,64);if e!=nil{return 0,0,e}
  tx,e:=strconv.ParseUint(f[8],10,64);return rx,tx,e
 }
 if err:=scanner.Err();err!=nil{return 0,0,err}
 return 0,0,fmt.Errorf("network interface %q not found",interfaceName)
}
func ParseDiskCounters(contents string, diskName string)(uint64,uint64,error){
 scanner:=bufio.NewScanner(strings.NewReader(contents))
 for scanner.Scan(){
  f:=strings.Fields(scanner.Text())
  if len(f)<14||f[2]!=diskName{continue}
  read,e:=strconv.ParseUint(f[5],10,64);if e!=nil{return 0,0,e}
  write,e:=strconv.ParseUint(f[9],10,64);return read,write,e
 }
 if err:=scanner.Err();err!=nil{return 0,0,err}
 return 0,0,fmt.Errorf("disk %q not found",diskName)
}
func ReadLinuxCounters(networkInterface,diskDevice string)(LinuxCounters,error){
 var c LinuxCounters
 netData,err:=os.ReadFile("/proc/net/dev");if err!=nil{return c,err}
 c.NetworkRX,c.NetworkTX,err=ParseNetworkCounters(string(netData),networkInterface);if err!=nil{return c,err}
 diskData,err:=os.ReadFile("/proc/diskstats");if err!=nil{return c,err}
 c.DiskReadSectors,c.DiskWriteSectors,err=ParseDiskCounters(string(diskData),diskDevice)
 return c,err
}

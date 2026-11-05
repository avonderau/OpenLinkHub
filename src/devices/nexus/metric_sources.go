package nexus

import (
 "math"
 "os"
 "regexp"
 "sort"
 "strings"
 "sync"
 "syscall"
 "time"
)

// Each source is an opaque, validated identifier, not an arbitrary file path.
// The empty ID continues to select the existing automatic/default metric.
type MetricSourceOption struct {
 ID string `json:"id"`
 Label string `json:"label"`
}
type MetricSourceRequest struct {
 Metric string
 Source string
}
var metricSourceDeviceName = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)
var metricSourceMountPath = regexp.MustCompile(`^/[a-zA-Z0-9_./ -]{0,160}package nexus

import (
 "math"
 "os"
 "regexp"
 "sort"
 "strings"
 "sync"
 "syscall"
 "time"
)

// Each source is an opaque, validated identifier, not an arbitrary file path.
// The empty ID continues to select the existing automatic/default metric.
type MetricSourceOption struct {
 ID string `json:"id"`
 Label string `json:"label"`
}
type MetricSourceRequest struct {
 Metric string
 Source string
}
var metricSourceDeviceName = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)
)

func metricSourceKind(metric string) string {
 switch metric {
 case "network.rx_rate", "network.tx_rate": return "net"
 case "disk.read_rate", "disk.write_rate": return "disk"
 case "disk.utilization", "disk.free_gb": return "mount"
 }
 return ""
}

func ValidMetricSource(metric, source string) bool {
 if source == "" { return true }
 kind := metricSourceKind(metric)
 if kind == "" || !strings.HasPrefix(source,kind+":") { return false }
 name := strings.TrimPrefix(source,kind+":")
 if kind == "mount" {
  if !metricSourceMountPath.MatchString(name) { return false }
  for _,segment := range strings.Split(name,"/") {
   if segment == "." || segment == ".." { return false }
  }
  return true
 }
 return metricSourceDeviceName.MatchString(name) && name != "lo"
}

func QualifiedMetricKey(metric,source string) string {
 if source == "" {return metric}
 return metric+"|"+source
}

// DiscoverMetricSources lists currently available Linux counter devices and
// mounted filesystems. The labels are deliberately simple: the raw device
// name or mount point remains recognizable if a device is later removed.
func DiscoverMetricSources() map[string][]MetricSourceOption {
 result:=map[string][]MetricSourceOption{}
 for _,metric:=range []string{"network.rx_rate","network.tx_rate","disk.read_rate","disk.write_rate","disk.utilization","disk.free_gb"} {
  result[metric]=[]MetricSourceOption{}
 }
 if data,err:=os.ReadFile("/proc/net/dev");err==nil {
  for _,line:=range strings.Split(string(data),"\n") {
   parts:=strings.SplitN(line,":",2)
   if len(parts)!=2 {continue}
   name:=strings.TrimSpace(parts[0])
   if !metricSourceDeviceName.MatchString(name)||name=="lo" {continue}
   option:=MetricSourceOption{ID:"net:"+name,Label:name}
   for _,metric:=range []string{"network.rx_rate","network.tx_rate"} {result[metric]=append(result[metric],option)}
  }
 }
 if data,err:=os.ReadFile("/proc/diskstats");err==nil {
  seen:=map[string]bool{}
  for _,line:=range strings.Split(string(data),"\n") {
   fields:=strings.Fields(line)
   if len(fields)<14 {continue}
   name:=fields[2]
   if !metricSourceDeviceName.MatchString(name)||seen[name]||strings.HasPrefix(name,"loop")||strings.HasPrefix(name,"ram") {continue}
   seen[name]=true
   option:=MetricSourceOption{ID:"disk:"+name,Label:"/dev/"+name}
   for _,metric:=range []string{"disk.read_rate","disk.write_rate"} {result[metric]=append(result[metric],option)}
  }
 }
 if data,err:=os.ReadFile("/proc/self/mountinfo");err==nil {
  seen:=map[string]bool{}
  for _,line:=range strings.Split(string(data),"\n") {
   fields:=strings.Fields(line)
   if len(fields)<7 {continue}
   // mountinfo encodes whitespace in mount paths.
   mount:=strings.ReplaceAll(fields[4],"\\040"," ")
   if !ValidMetricSource("disk.utilization","mount:"+mount)||seen[mount] {continue}
   if strings.HasPrefix(mount,"/proc")||strings.HasPrefix(mount,"/sys")||strings.HasPrefix(mount,"/dev")||
      (strings.HasPrefix(mount,"/run")&&!strings.HasPrefix(mount,"/run/media/")) {continue}
   seen[mount]=true
   option:=MetricSourceOption{ID:"mount:"+mount,Label:mount}
   for _,metric:=range []string{"disk.utilization","disk.free_gb"} {result[metric]=append(result[metric],option)}
  }
 }
 for key:=range result {
  sort.Slice(result[key],func(i,j int)bool{return result[key][i].Label<result[key][j].Label})
 }
 return result
}

type sourceRateSample struct {
 RX uint64
 TX uint64
 At time.Time
}
type SourceRateSampler struct {
 mu sync.Mutex
 previous map[string]sourceRateSample
}
func(s *SourceRateSampler) rate(source string,rx,tx uint64,now time.Time,scale float64)(float64,float64,bool) {
 s.mu.Lock()
 defer s.mu.Unlock()
 if s.previous==nil{s.previous=map[string]sourceRateSample{}}
 old,ok:=s.previous[source]
 s.previous[source]=sourceRateSample{RX:rx,TX:tx,At:now}
 if !ok||!now.After(old.At)||rx<old.RX||tx<old.TX{return 0,0,false}
 seconds:=now.Sub(old.At).Seconds()
 if seconds<=0{return 0,0,false}
 left,right:=float64(rx-old.RX)*scale/seconds,float64(tx-old.TX)*scale/seconds
 if math.IsNaN(left)||math.IsInf(left,0)||math.IsNaN(right)||math.IsInf(right,0){return 0,0,false}
 return left,right,true
}

// ReadSourcedMetrics samples only sources referenced by the active layout.
// It checks every requested ID against discovered devices before reading,
// so saved layouts cannot cause arbitrary filesystem stat calls.
func ReadSourcedMetrics(sampler *SourceRateSampler, requests []MetricSourceRequest,now time.Time) MetricSnapshot {
 values:=MetricSnapshot{}
 if sampler==nil||len(requests)==0{return values}
 available:=DiscoverMetricSources()
 requested:=map[string]map[string]bool{}
 for _,r:=range requests {
  if r.Source==""||!ValidMetricSource(r.Metric,r.Source){continue}
  found:=false
  for _,option:=range available[r.Metric]{if option.ID==r.Source{found=true;break}}
  if !found {continue}
  if requested[r.Source]==nil {requested[r.Source]=map[string]bool{}}
  requested[r.Source][r.Metric]=true
 }
 if len(requested)==0{return values}
 networkData,_:=os.ReadFile("/proc/net/dev")
 diskData,_:=os.ReadFile("/proc/diskstats")
 for source,metrics:=range requested {
  switch {
  case strings.HasPrefix(source,"net:"):
   name:=strings.TrimPrefix(source,"net:")
   rx,tx,err:=ParseNetworkCounters(string(networkData),name)
   if err!=nil{continue}
   left,right,valid:=sampler.rate(source,rx,tx,now,1)
   if !valid{continue}
   if metrics["network.rx_rate"] {values[QualifiedMetricKey("network.rx_rate",source)]=left}
   if metrics["network.tx_rate"] {values[QualifiedMetricKey("network.tx_rate",source)]=right}
  case strings.HasPrefix(source,"disk:"):
   name:=strings.TrimPrefix(source,"disk:")
   rx,tx,err:=ParseDiskCounters(string(diskData),name)
   if err!=nil{continue}
   left,right,valid:=sampler.rate(source,rx,tx,now,512)
   if !valid{continue}
   if metrics["disk.read_rate"]{values[QualifiedMetricKey("disk.read_rate",source)]=left}
   if metrics["disk.write_rate"]{values[QualifiedMetricKey("disk.write_rate",source)]=right}
  case strings.HasPrefix(source,"mount:"):
   mount:=strings.TrimPrefix(source,"mount:")
   var stat syscall.Statfs_t
   if err:=syscall.Statfs(mount,&stat);err!=nil||stat.Blocks==0{continue}
   if metrics["disk.utilization"] {
    values[QualifiedMetricKey("disk.utilization",source)]=100*float64(stat.Blocks-stat.Bfree)/float64(stat.Blocks)
   }
   if metrics["disk.free_gb"]{
    values[QualifiedMetricKey("disk.free_gb",source)]=float64(stat.Bavail)*float64(stat.Bsize)/1e9
   }
  }
 }
 return values
}

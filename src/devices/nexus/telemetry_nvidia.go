package nexus

import (
 "context"
 "os/exec"
 "strconv"
 "strings"
 "time"
)

// parseNvidiaMetrics converts a single nvidia-smi CSV row to optional metrics.
// The kernel/driver decides which columns are available; missing values are
// omitted rather than rendered as zero.
func parseNvidiaMetrics(csv string)MetricSnapshot{
 result:=MetricSnapshot{}
 line:=strings.TrimSpace(strings.SplitN(csv,"\n",2)[0])
 fields:=strings.Split(line,",")
 if len(fields)!=4{return result}
 values:=make([]float64,len(fields))
 valid:=make([]bool,len(fields))
 for i,raw:=range fields{
  text:=strings.TrimSpace(raw)
  n,err:=strconv.ParseFloat(text,64)
  if err==nil && n>=0 && n<1000000 {values[i]=n;valid[i]=true}
 }
 if valid[0]&&valid[1]&&values[1]>0&&values[0]<=values[1]{
  result["gpu.vram.used_mb"]=values[0]
  result["gpu.vram.total_mb"]=values[1]
  result["gpu.vram.utilization"]=100*values[0]/values[1]
 }
 if valid[2]&&values[2]>=100&&values[2]<=5000{result["gpu.clock_mhz"]=values[2]}
 if valid[3]&&values[3]<=2000{result["gpu.power_w"]=values[3]}
 return result
}

// ReadNvidiaMetrics uses a fixed, shell-free command with an 800ms deadline.
// It is called at most once per second through the Nexus telemetry cache,
// and only when a configured NVIDIA GPU is present.
func ReadNvidiaMetrics(index int)MetricSnapshot{
 if index<0||index>15{return MetricSnapshot{}}
 ctx,cancel:=context.WithTimeout(context.Background(),800*time.Millisecond)
 defer cancel()
 command:=exec.CommandContext(ctx,"nvidia-smi","-i",strconv.Itoa(index),
  "--query-gpu=memory.used,memory.total,clocks.gr,power.draw",
  "--format=csv,noheader,nounits")
 output,err:=command.Output()
 if err!=nil||len(output)>1024{return MetricSnapshot{}}
 return parseNvidiaMetrics(string(output))
}

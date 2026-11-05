package nexus

import (
 "math"
 "strings"
 "time"
)

// NexusTelemetryPreview is a read-only finite subset of the same telemetry
// already used by the physical Nexus. NaN gaps are encoded as JSON null.
type NexusTelemetryPreview struct {
 SampledAt int64 `json:"sampledAt"`
 Metrics map[string]float64 `json:"metrics"`
 History map[string][]*float64 `json:"history"`
}

func prepareNexusTelemetryPreview(snapshot MetricSnapshot,history MetricHistory,sampledAt,now time.Time)NexusTelemetryPreview{
 result:=NexusTelemetryPreview{Metrics:map[string]float64{},History:map[string][]*float64{}}
 if sampledAt.IsZero()||now.Before(sampledAt)||now.Sub(sampledAt)>10*time.Second{return result}
 result.SampledAt=sampledAt.Unix()
 names:=map[string]bool{}
 for name:=range supportedMetrics {names[name]=true}
 for name:=range snapshot {if strings.Contains(name,"|"){names[name]=true}}
 for name:=range history {if strings.Contains(name,"|"){names[name]=true}}
 for name:=range names{
  if v,ok:=snapshot[name];ok&&!math.IsNaN(v)&&!math.IsInf(v,0){result.Metrics[name]=v}
  points,ok:=history[name]
  if !ok{continue}
  normalized:=make([]*float64,len(points))
  for i,v:=range points{
   if math.IsNaN(v)||math.IsInf(v,0){continue}
   copyOfValue:=v
   normalized[i]=&copyOfValue
  }
  result.History[name]=normalized
 }
 return result
}

// GetNexusTelemetryPreview never polls hardware or launches a process. It
// returns only existing cached samples, without holding a mutable map after
// releasing the device mutex.
func (d *Device) GetNexusTelemetryPreview()NexusTelemetryPreview {
 d.mutex.Lock()
 snapshot:=make(MetricSnapshot,len(d.customSampler.snapshot))
 for k,v:=range d.customSampler.snapshot{snapshot[k]=v}
 history:=d.customSampler.history.snapshot()
 sampledAt:=d.customSampler.snapshotAt
 d.mutex.Unlock()
 return prepareNexusTelemetryPreview(snapshot,history,sampledAt,time.Now())
}

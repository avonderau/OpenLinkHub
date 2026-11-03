package nexus

import (
 "testing"
 "time"
)

func TestNexusTelemetryCacheHoldsValuesBetweenAnimatedFrames(t *testing.T){
 start:=time.Unix(1000,0)
 var sampler CounterSampler
 if _,_,ok:=sampler.Cached(start);ok{t.Fatal("empty sampler returned cached metrics")}
 initial:=MetricSnapshot{"cpu.utilization":23,"gpu.utilization":50}
 firstHistory:=sampler.Store(start,initial)
 if len(firstHistory["cpu.utilization"])!=1{t.Fatal("initial history sample missing")}
 for frame:=1;frame<5;frame++{
  now:=start.Add(time.Duration(frame)*200*time.Millisecond)
  cached,history,ok:=sampler.Cached(now)
  if !ok{t.Fatalf("200ms animation frame %d unnecessarily sampled telemetry",frame)}
  if cached["cpu.utilization"]!=23{t.Fatal("cached metrics changed")}
  if len(history["cpu.utilization"])!=1{t.Fatal("a GIF redraw added spurious history")}
 }
 if _,_,ok:=sampler.Cached(start.Add(time.Second));ok{
  t.Fatal("metrics cache remained fresh after one second")
 }
 second:=MetricSnapshot{"cpu.utilization":48,"gpu.utilization":55}
 nextHistory:=sampler.Store(start.Add(time.Second),second)
 if len(nextHistory["cpu.utilization"])!=2{t.Fatal("next sample missing")}
 if nextHistory["cpu.utilization"][0]!=23||nextHistory["cpu.utilization"][1]!=48{
  t.Fatalf("unexpected two-sample history: %v",nextHistory["cpu.utilization"])
 }
}

func TestNexusTelemetryCacheRejectsClockReversalAndReturnsIndependentHistory(t *testing.T){
 var sampler CounterSampler
 now:=time.Unix(100,0)
 sampler.Store(now,MetricSnapshot{"cpu.temperature":52})
 _,history,ok:=sampler.Cached(now.Add(100*time.Millisecond))
 if !ok{t.Fatal("expected cached telemetry")}
 history["cpu.temperature"][0]=999
 _,again,ok:=sampler.Cached(now.Add(300*time.Millisecond))
 if !ok||again["cpu.temperature"][0]!=52{t.Fatal("history copy aliases internal samples")}
 if _,_,ok:=sampler.Cached(now.Add(-time.Second));ok{t.Fatal("cached telemetry reused after clock moved backward")}
}

func TestNexusTelemetryCacheAddsMissingSensorGapOnlyOncePerSecond(t *testing.T){
 var sampler CounterSampler
 now:=time.Unix(101,0)
 sampler.Store(now,MetricSnapshot{"fan.rpm":1200})
 for i:=1;i<5;i++{
  _,_,ok:=sampler.Cached(now.Add(time.Duration(i)*200*time.Millisecond))
  if !ok{t.Fatal("duplicate sample needed within one second")}
 }
 history:=sampler.Store(now.Add(time.Second),MetricSnapshot{})
 values:=history["fan.rpm"]
 if len(values)!=2{t.Fatalf("expected two history points, got %v",values)}
 if values[0]!=1200||values[1]==values[1]{t.Fatalf("missing sensor was not a NaN graph gap: %v",values)}
}

package nexus

import (
 "math"
 "testing"
 "time"
)

func TestMetricSourceValidation(t *testing.T){
 accepted:=[]MetricSourceRequest{
  {"disk.free_gb","mount:/"},{"disk.utilization","mount:/home"},
  {"disk.read_rate","disk:nvme0n1"},{"disk.write_rate","disk:sda"},
  {"network.rx_rate","net:eth0"},{"network.tx_rate","net:wlp2s0"},
 }
 for _,tc:=range accepted {if !ValidMetricSource(tc.Metric,tc.Source){t.Errorf("source %s for %s rejected",tc.Source,tc.Metric)}}
 rejected:=[]MetricSourceRequest{
  {"cpu.temperature","disk:sda"},{"disk.read_rate","net:eth0"},
  {"network.tx_rate","disk:sda"},{"disk.utilization","mount:/../etc"},
  {"disk.free_gb","mount:/home/../../tmp"},{"disk.free_gb","disk:sda"},
  {"disk.read_rate","disk:../../sda"},{"network.rx_rate","net:lo"},
  {"disk.utilization","mount:/media/space mount"},
 }
 for _,tc:=range rejected {if ValidMetricSource(tc.Metric,tc.Source){t.Errorf("source %s for %s accepted",tc.Source,tc.Metric)}}
 if QualifiedMetricKey("disk.read_rate","")!="disk.read_rate"{t.Fatal("old layout metric key was changed")}
 if QualifiedMetricKey("disk.read_rate","disk:sda")!="disk.read_rate|disk:sda"{t.Fatal("source-specific metric key mismatch")}
}

func TestSourceRateSamplerTracksEachDeviceIndependently(t *testing.T){
 var sampler SourceRateSampler
 t0:=time.Unix(100,0)
 if _,_,ok:=sampler.rate("disk:a",10,20,t0,512);ok{t.Fatal("first sample must not be a rate")}
 if _,_,ok:=sampler.rate("disk:b",1000,2000,t0,512);ok{t.Fatal("first alternate disk should not have a rate")}
 read,write,ok:=sampler.rate("disk:a",12,24,t0.Add(2*time.Second),512)
 if !ok||read!=512||write!=1024{t.Fatalf("unexpected disk A rates %f %f",read,write)}
 read,write,ok=sampler.rate("disk:b",1004,2008,t0.Add(2*time.Second),512)
 if !ok||read!=1024||write!=2048{t.Fatalf("unexpected disk B rates %f %f",read,write)}
 if _,_,ok=sampler.rate("disk:a",0,0,t0.Add(3*time.Second),512);ok{t.Fatal("counter reset produced spike")}
 if _,_,ok=sampler.rate("disk:a",^uint64(0),0,t0.Add(4*time.Second),512);ok{t.Fatal("negative counter on write should not produce a rate")}
}

func TestDiscoveredMetricSourcesAreValidAndFinite(t *testing.T){
 sources:=DiscoverMetricSources()
 for metric,options:=range sources{
  ids:=map[string]bool{}
  for _,source:=range options{
   if !ValidMetricSource(metric,source.ID) {t.Fatalf("invalid discovered source %q for %q",source.ID,metric)}
   if ids[source.ID]{t.Fatalf("duplicate source %q",source.ID)}
   ids[source.ID]=true
  }
 }
 values:=ReadSourcedMetrics(&SourceRateSampler{},[]MetricSourceRequest{{Metric:"disk.utilization",Source:"mount:/"}},time.Now())
 for key,value:=range values {
  if math.IsNaN(value)||math.IsInf(value,0){t.Fatalf("nonfinite result %s",key)}
 }
}

func TestNexusMetricSourceValidation(t *testing.T){
 base:=ScreenConfiguration{Version:1,ActiveScreenID:"home",Screens:[]Screen{{
  ID:"home",Name:"Home",Widgets:[]Widget{{
   ID:"disk",Type:"metric",Metric:"disk.utilization",X:0,Y:0,Width:120,Height:48,
   Settings:map[string]string{"metricSource":"mount:/"},
  }},
 }}}
 if err:=base.Validate();err!=nil{t.Fatalf("volume source should validate: %v",err)}
 base.Screens[0].Widgets[0].Settings["metricSource"]="disk:sda"
 if err:=base.Validate();err==nil{t.Fatal("wrong source type should fail")}
 base.Screens[0].Widgets[0].Type="button"
 if err:=base.Validate();err==nil{t.Fatal("source cannot be assigned to a button")}
}

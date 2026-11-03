package nexus

import(
 "math"
 "testing"
 "time"
)

func TestMetricHistorySamplingIsBoundedAndRateLimited(t *testing.T){
 var h metricHistory
 start:=time.Unix(1_000,0)
 for i:=0;i<100;i++{
  when:=start.Add(time.Duration(i)*time.Second)
  h.add(MetricSnapshot{"cpu.utilization":float64(i),"gpu.temperature":60},when)
  h.add(MetricSnapshot{"cpu.utilization":999},when.Add(200*time.Millisecond))
 }
 snapshot:=h.snapshot()
 values:=snapshot["cpu.utilization"]
 if len(values)!=maxNexusHistorySamples{t.Fatalf("got %d samples",len(values))}
 if values[0]!=40||values[len(values)-1]!=99{t.Fatalf("wrong bounded window: first=%v last=%v",values[0],values[len(values)-1])}
 snapshot["cpu.utilization"][0]=999
 if h.values["cpu.utilization"][0]!=40{t.Fatal("snapshot aliased internal data")}
}
func TestMetricHistoryMissingMetricsCreateGaps(t *testing.T){
 var h metricHistory
 now:=time.Unix(50,0)
 h.add(MetricSnapshot{"cpu.utilization":10},now)
 h.add(MetricSnapshot{},now.Add(time.Second))
 h.add(MetricSnapshot{"cpu.utilization":35},now.Add(2*time.Second))
 points:=h.snapshot()["cpu.utilization"]
 if len(points)!=3||points[0]!=10||!math.IsNaN(points[1])||points[2]!=35{t.Fatalf("bad gap samples: %v",points)}
}
func TestGraphRendersRealHistoryPixels(t *testing.T){
 screen:=Screen{ID:"graphs",Name:"Graphs",Widgets:[]Widget{{
  ID:"graph",Type:"graph",Metric:"cpu.utilization",
  X:0,Y:0,Width:208,Height:48,
  Settings:map[string]string{"label":"CPU","style":"solid","textColor":"#00ff99"},
 }}}
 values:=MetricSnapshot{"cpu.utilization":70}
 history:=MetricHistory{"cpu.utilization":{10,20,40,70,30,85,60}}
 withGraph,err:=RenderScreenWithBackgroundAndIconsHistory(screen,values,nil,"",history)
 if err!=nil{t.Fatal(err)}
 withoutGraph,err:=RenderScreen(screen,values)
 if err!=nil{t.Fatal(err)}
 differences:=0
 for y:=23;y<43;y++{for x:=4;x<204;x++{
  if withGraph.RGBAAt(x,y)!=withoutGraph.RGBAAt(x,y){differences++}
 }}
 if differences<5{t.Fatalf("no meaningful sparkline drawn (%d changed pixels)",differences)}
}
func TestInvalidGraphMetricRejected(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"graphs",Screens:[]Screen{{
  ID:"graphs",Name:"Graphs",Widgets:[]Widget{{ID:"bad",Type:"graph",Metric:"unsupported",X:0,Y:0,Width:100,Height:48}},
 }}}
 if err:=cfg.Validate();err==nil{t.Fatal("unsupported graph metric accepted")}
}

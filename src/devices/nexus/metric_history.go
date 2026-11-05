package nexus

import (
 "image"
 "image/color"
 "math"
 "strings"
 "time"
)

const maxNexusHistorySamples=60

// MetricHistory is a bounded, copied set of one-second metric samples.
type MetricHistory map[string][]float64

type metricHistory struct{
 last time.Time
 values MetricHistory
}

// add never samples faster than 1 Hz, even when animated GIFs trigger a
// faster LCD refresh. Unavailable measurements are stored as NaN gaps.
func (h *metricHistory) add(values MetricSnapshot,now time.Time){
 if !h.last.IsZero()&&now.Sub(h.last)<time.Second{return}
 h.last=now
 if h.values==nil{h.values=make(MetricHistory)}
 metricKeys:=map[string]bool{}
 for metric:=range supportedMetrics{metricKeys[metric]=true}
 for metric:=range values {
  if strings.Contains(metric,"|"){metricKeys[metric]=true}
 }
 for metric:=range h.values {
  if strings.Contains(metric,"|"){metricKeys[metric]=true}
 }
 for metric:=range metricKeys{
  reading,ok:=values[metric]
  if !ok||math.IsInf(reading,0)||math.IsNaN(reading){reading=math.NaN()}
  samples:=append(h.values[metric],reading)
  if len(samples)>maxNexusHistorySamples{samples=append([]float64(nil),samples[len(samples)-maxNexusHistorySamples:]...)}
  h.values[metric]=samples
  // Remove disconnected source histories once every sample is unavailable.
  if strings.Contains(metric,"|")&&len(samples)==maxNexusHistorySamples {
   missing:=true
   for _,v:=range samples {if !math.IsNaN(v){missing=false;break}}
   if missing{delete(h.values,metric)}
  }
 }
}

func (h *metricHistory) snapshot()MetricHistory{
 out:=make(MetricHistory,len(h.values))
 for name,samples:=range h.values{out[name]=append([]float64(nil),samples...)}
 return out
}

func isUtilizationMetric(name string)bool{
 switch name{
 case "cpu.utilization","gpu.utilization","memory.utilization","disk.utilization","gpu.vram.utilization":return true
 }
 return false
}

func drawNexusHistoryGraph(canvas *image.RGBA,bounds image.Rectangle,metric string,samples []float64,ink color.RGBA){
 if len(samples)<2||bounds.Dx()<16||bounds.Dy()<25{return}
 plot:=image.Rect(bounds.Min.X+4,bounds.Min.Y+23,bounds.Max.X-4,bounds.Max.Y-5)
 if plot.Dx()<2||plot.Dy()<2{return}
 minimum,maximum:=math.Inf(1),math.Inf(-1)
 for _,v:=range samples{
  if math.IsNaN(v)||math.IsInf(v,0){continue}
  if v<minimum{minimum=v}
  if v>maximum{maximum=v}
 }
 if math.IsInf(minimum,1){return}
 if isUtilizationMetric(metric){minimum,maximum=0,100}
 if maximum-minimum<1 {maximum+=.5;minimum-=.5}
 previousX,previousY:=0,0
 havePrevious:=false
 for i,v:=range samples{
  if math.IsNaN(v)||math.IsInf(v,0){havePrevious=false;continue}
  x:=plot.Min.X+int(float64(i)*float64(plot.Dx()-1)/float64(len(samples)-1))
  scaled:=(v-minimum)/(maximum-minimum)
  scaled=math.Max(0,math.Min(1,scaled))
  y:=plot.Max.Y-1-int(scaled*float64(plot.Dy()-1))
  if havePrevious{drawNexusHistoryLine(canvas,plot,previousX,previousY,x,y,ink)}
  previousX,previousY=x,y
  havePrevious=true
 }
}

func drawNexusHistoryLine(canvas *image.RGBA,plot image.Rectangle,x0,y0,x1,y1 int,ink color.RGBA){
 dx:=int(math.Abs(float64(x1-x0)))
 dy:=-int(math.Abs(float64(y1-y0)))
 sx,sy:=-1,-1
 if x0<x1{sx=1}
 if y0<y1{sy=1}
 err:=dx+dy
 for {
  if image.Pt(x0,y0).In(plot){canvas.SetRGBA(x0,y0,ink)}
  if x0==x1&&y0==y1{break}
  twice:=2*err
  if twice>=dy{err+=dy;x0+=sx}
  if twice<=dx{err+=dx;y0+=sy}
 }
}

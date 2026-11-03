package nexus

import (
 "image/color"
 "testing"
)

func TestNexusTypographySettingsValidation(t *testing.T){
 c:=ScreenConfiguration{Version:1,ActiveScreenID:"type",Screens:[]Screen{{
  ID:"type",Name:"Typography",Widgets:[]Widget{{
   ID:"label",Type:"text",X:0,Y:0,Width:160,Height:48,
   Settings:map[string]string{"text":"OpenLinkHub","fontSize":"large","textAlign":"center"},
  }},
 }}}
 if err:=c.Validate();err!=nil{t.Fatal(err)}
 c.Screens[0].Widgets[0].Settings["fontSize"]="9000px"
 if err:=c.Validate();err==nil{t.Fatal("unsafe font size accepted")}
 c.Screens[0].Widgets[0].Settings["fontSize"]="normal"
 c.Screens[0].Widgets[0].Settings["textAlign"]="bogus"
 if err:=c.Validate();err==nil{t.Fatal("invalid alignment accepted")}
}

func TestNexusStyledTextPositionsAndBounds(t *testing.T){
 makeScreen:=func(size,align string)Screen{
  return Screen{ID:"type",Name:"Typography",Widgets:[]Widget{{
   ID:"label",Type:"text",X:0,Y:0,Width:160,Height:48,
   Settings:map[string]string{"text":"ABC","fontSize":size,"textAlign":align,"style":"solid"},
  }}}
 }
 colorBackground:=color.RGBA{27,41,53,255}
 measure:=func(size,align string)(int,int,int){
  frame,err:=RenderScreen(makeScreen(size,align),nil)
  if err!=nil{t.Fatal(err)}
  minimum,maximum,count:=161,-1,0
  for y:=0;y<48;y++{for x:=0;x<200;x++{
   pix:=frame.RGBAAt(x,y)
   if pix==colorBackground||pix==(color.RGBA{12,19,27,255}){continue}
   count++;if x<minimum{minimum=x};if x>maximum{maximum=x}
  }}
  return minimum,maximum,count
 }
 first,_,count:=measure("normal","left")
 middle,_,countMiddle:=measure("small","center")
 right,_,countRight:=measure("large","right")
 if count<5||countMiddle<5||countRight<5{t.Fatalf("missing font pixels: normal=%d, small=%d, large=%d",count,countMiddle,countRight)}
 if !(first<middle&&middle<right){t.Fatalf("alignment did not move text: left=%d, center=%d, right=%d",first,middle,right)}
 _,maxRight,_:=measure("large","right")
 if maxRight>=160{t.Fatalf("large text overflowed widget boundary: %d",maxRight)}
}

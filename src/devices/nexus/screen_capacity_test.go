package nexus

import (
 "os"
 "path/filepath"
 "strconv"
 "testing"
)

func TestNexusSupports256ScreensWithResourceCap(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"s0"}
 for i:=0;i<MaxScreens;i++{
  cfg.Screens=append(cfg.Screens,Screen{ID:"s"+strconv.Itoa(i),Name:"Screen "+strconv.Itoa(i),Widgets:[]Widget{}})
 }
 if MaxScreens!=256{t.Fatalf("expected 256 screen limit, got %d",MaxScreens)}
 if err:=cfg.Validate();err!=nil{t.Fatalf("256 screens rejected: %v",err)}
 path:=filepath.Join(t.TempDir(),"screens.json")
 if err:=SaveScreens(path,cfg);err!=nil{t.Fatal(err)}
 loaded,err:=LoadScreens(path)
 if err!=nil{t.Fatal(err)}
 if len(loaded.Screens)!=256{t.Fatal("256-screen roundtrip failed")}
 cfg.Screens=append(cfg.Screens,Screen{ID:"overflow",Name:"Overflow"})
 if err:=cfg.Validate();err==nil{t.Fatal("accepted 257 screens")}
}

func TestNexusTotalWidgetCap(t *testing.T){
 cfg:=ScreenConfiguration{Version:1,ActiveScreenID:"s0"}
 for i:=0;i<65;i++{
  screen:=Screen{ID:"s"+strconv.Itoa(i),Name:"Screen"}
  for j:=0;j<64;j++{
   screen.Widgets=append(screen.Widgets,Widget{ID:"w"+strconv.Itoa(j),Type:"text",X:0,Y:0,Width:8,Height:8})
  }
  cfg.Screens=append(cfg.Screens,screen)
 }
 if err:=cfg.Validate();err==nil{t.Fatal("accepted more than 4096 widgets")}
 cfg.Screens=cfg.Screens[:64]
 if err:=cfg.Validate();err!=nil{t.Fatalf("4096 widgets rejected: %v",err)}
}

func TestNexusRejectsOversizedLayoutFile(t *testing.T){
 path:=filepath.Join(t.TempDir(),"too-large.json")
 if err:=os.WriteFile(path,make([]byte,MaxScreenConfigBytes+1),0600);err!=nil{t.Fatal(err)}
 if _,err:=LoadScreens(path);err==nil{t.Fatal("accepted oversized JSON")}
}

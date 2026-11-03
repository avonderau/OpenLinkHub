package nexus
import ("path/filepath";"testing")
func sample()ScreenConfiguration{return ScreenConfiguration{Version:1,ActiveScreenID:"system",Screens:[]Screen{{ID:"system",Name:"System",Widgets:[]Widget{{ID:"cpu",Type:"metric",Metric:"cpu.temperature",X:0,Y:0,Width:120,Height:48}}},{ID:"network",Name:"Network"}}}}
func TestValidation(t *testing.T){c:=sample();if err:=c.Validate();err!=nil{t.Fatal(err)};c.Screens[0].Widgets[0].Width=641;if c.Validate()==nil{t.Fatal("out of bounds accepted")}}
func TestNavigation(t *testing.T){c:=sample();if c.NextScreen()!="network"||c.PreviousScreen()!="network"{t.Fatal("navigation failed")}}
func TestPersistence(t *testing.T){p:=filepath.Join(t.TempDir(),"screens.json");c:=sample();if err:=SaveScreens(p,c);err!=nil{t.Fatal(err)};got,err:=LoadScreens(p);if err!=nil||got.ActiveScreenID!="system"{t.Fatalf("load: %v",err)}}
func TestInvalidMetric(t *testing.T){c:=sample();c.Screens[0].Widgets[0].Metric="shell.command";if c.Validate()==nil{t.Fatal("invalid metric accepted")}}

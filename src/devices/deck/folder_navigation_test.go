package deck

import (
 "strings"
 "testing"
 "os"
 "path/filepath"
 "encoding/json"
)

func testNestedFolderLayout(m Model) Layout {
 return Layout{Version:1,ActivePage:"home",Pages:[]Page{
  {ID:"home",Name:"Home",Keys:[]KeyConfig{
   {Index:0,Action:"folder-open",TargetPage:"games"},
   {Index:1,Action:"page-next"},
  }},
  {ID:"games",Name:"Games",ParentPage:"home",Keys:[]KeyConfig{
   {Index:0,Action:"folder-open",TargetPage:"gaming-tools"},
   {Index:16,Action:"folder-back"},
  }},
  {ID:"gaming-tools",Name:"Gaming Tools",ParentPage:"games",Keys:[]KeyConfig{
   {Index:16,Action:"folder-back"},
  }},
 }}
}

func TestNestedFolderNavigationOnPhysicalDeck(t *testing.T) {
 d:=testDevice(t)
 d.layout=testNestedFolderLayout(d.Model)
 if err:=d.layout.Validate(d.Model);err!=nil{t.Fatal(err)}
 handle:=new(fakeDeckSession)
 if err:=d.attachSession(handle);err!=nil{t.Fatal(err)}
 before:=countMiraboxOperation(handle.reports,"STP")
 for i,expected:=range []struct{key int;page string}{
  {0,"games"},{0,"gaming-tools"},{16,"games"},{16,"home"},
 }{
  d.handleKeyPress(expected.key)
  if got:=d.GetStatus().ActivePage;got!=expected.page{
   t.Fatalf("step %d: page %q, want %q",i,got,expected.page)
  }
  stored,err:=LoadLayout(d.root,d.Serial,d.Model)
  if err!=nil || stored.ActivePage!=expected.page{
   t.Fatalf("step %d: persisted page = %q, err %v",i,stored.ActivePage,err)
  }
 }
 if got:=countMiraboxOperation(handle.reports,"STP");got!=before+4{
  t.Fatalf("expected four physical screen updates, got %d (initial %d)",got,before)
 }
 d.Stop()
}

func TestFolderBackUsesPersistedParentAfterRestart(t *testing.T) {
 d:=testDevice(t)
 layout:=testNestedFolderLayout(d.Model)
 layout.ActivePage="gaming-tools"
 if err:=SaveLayout(d.root,d.Serial,d.Model,layout);err!=nil{t.Fatal(err)}
 stored,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil{t.Fatal(err)}
 // An application restart loses any in-memory navigation history. The
 // parentPage metadata is sufficient for returning from the active folder.
 restarted:=testDevice(t)
 restarted.root=d.root
 restarted.Serial=d.Serial
 restarted.layout=stored
 restarted.handleKeyPress(16)
 if got:=restarted.GetStatus().ActivePage;got!="games"{
  t.Fatalf("Back after restart navigated to %q, want games",got)
 }
 restarted.handleKeyPress(16)
 if got:=restarted.GetStatus().ActivePage;got!="home"{
  t.Fatalf("Back from parent navigated to %q, want home",got)
 }
}

func TestFolderValidationRejectsBadTreeAndActions(t *testing.T){
 d:=testDevice(t)
 good:=testNestedFolderLayout(d.Model)
 cases:=[]struct{
  name string
  change func(*Layout)
 }{
  {"missing parent",func(l *Layout){l.Pages[1].ParentPage="missing"}},
  {"direct cycle",func(l *Layout){l.Pages[1].ParentPage="games"}},
  {"nested cycle",func(l *Layout){l.Pages[0].ParentPage="gaming-tools"}},
  {"nonchild target",func(l *Layout){l.Pages[0].Keys[0].TargetPage="gaming-tools"}},
  {"unknown folder",func(l *Layout){l.Pages[0].Keys[0].TargetPage="missing"}},
  {"back on root",func(l *Layout){l.Pages[0].Keys[0]=KeyConfig{Index:0,Action:"folder-back"}}},
  {"back with explicit target",func(l *Layout){l.Pages[1].Keys[1].TargetPage="home"}},
  {"back with command",func(l *Layout){l.Pages[1].Keys[1].Target="sh -c bad"}},
  {"open folder with command",func(l *Layout){l.Pages[0].Keys[0].Target="sh -c bad"}},
 }
 for _,tc:=range cases{
  t.Run(tc.name,func(t *testing.T){
   bad:=cloneLayout(good)
   tc.change(&bad)
   if err:=bad.Validate(d.Model);err==nil{t.Fatal("invalid folder configuration accepted")}
  })
 }
}

func TestFolderActionMalformedInMemoryDoesNotNavigate(t *testing.T){
 d:=testDevice(t)
 d.layout=testNestedFolderLayout(d.Model)
 d.layout.Pages[0].Keys[0].TargetPage="gaming-tools"
 d.handleKeyPress(0)
 if got:=d.GetStatus().ActivePage;got!="home"{
  t.Fatalf("malformed folder action navigated to %q",got)
 }
 d.layout.Pages[0].Keys[0].Action="folder-back"
 d.handleKeyPress(0)
 if got:=d.GetStatus().ActivePage;got!="home"{
  t.Fatalf("Back from root navigated to %q",got)
 }
}

func TestFolderDefaultLabelsAndRasterIcons(t *testing.T){
 for _,tc:=range []struct{action,want string}{
  {"folder-open","Folder"},{"folder-back","Back"},
 }{
  if got:=keyDisplayLabel(KeyConfig{Action:tc.action});got!=tc.want{
   t.Errorf("%s default label = %q",tc.action,got)
  }
  icon:=renderDeckFolderKey(85,KeyConfig{Action:tc.action})
  background:=renderKeyLabel(85,"")
  changed:=0
  bounds:=icon.Bounds()
  for y:=bounds.Min.Y;y<bounds.Max.Y;y++{
   for x:=bounds.Min.X;x<bounds.Max.X;x++{
    if icon.At(x,y)!=background.At(x,y){changed++}
   }
  }
  if changed==0{t.Errorf("%s has no visible icon",tc.action)}
 }
 if text:=keyDisplayLabel(KeyConfig{Action:"folder-open",Label:"Games"});!strings.Contains(text,"Games"){
  t.Fatal("custom folder caption was lost")
 }
}

func TestLegacyDeckLayoutRemainsCompatible(t *testing.T){
 d:=testDevice(t)
 l:=DefaultLayout(d.Model)
 l.Pages=append(l.Pages,Page{ID:"second",Name:"Second",Keys:[]KeyConfig{{Index:16,Action:"page-go",TargetPage:"home"}}})
 if err:=l.Validate(d.Model);err!=nil{t.Fatalf("v1 pages without folder metadata failed: %v",err)}
}

func TestMiraboxPassiveDisplayCellsOnlySupportVisualContent(t *testing.T) {
 d:=testDevice(t)
 for _,index:=range []int{5,11,17} {
  layout:=DefaultLayout(d.Model)
  layout.Pages[0].Keys=[]KeyConfig{{Index:index,Label:"CPU",Metric:"cpu.temperature"}}
  if err:=layout.Validate(d.Model);err!=nil{t.Fatalf("display %d cannot show metrics: %v",index+1,err)}
  for _,action:=range []string{"page-next","folder-back","media","macro"}{
   layout.Pages[0].Keys=[]KeyConfig{{Index:index,Action:action}}
   if err:=layout.Validate(d.Model);err==nil{t.Errorf("display %d accepted action %q",index+1,action)}
  }
  d.layout.Pages[0].Keys=[]KeyConfig{{Index:index,Action:"page-next"}}
  d.handleKeyPress(index)
  status:=d.GetStatus()
  if status.ButtonEvents!=0||status.ActivePage!="home"{
   t.Errorf("passive display %d dispatched a physical action: %+v",index+1,status)
  }
 }
}

func TestMiraboxBackAtPhysicalButtonSeventeen(t *testing.T) {
 d:=testDevice(t)
 if got:=d.Model.PreferredBackIndex();got!=16{t.Fatalf("Back index = %d, want 16",got)}
 l:=testNestedFolderLayout(d.Model)
 if err:=l.Validate(d.Model);err!=nil{t.Fatal(err)}
 for _,page:=range l.Pages {
  if page.ParentPage==""{continue}
  found:=false
  for _,key:=range page.Keys{
   if key.Action=="folder-back" {
    found=true
    if key.Index!=16 {t.Errorf("Back on display %d instead of button 17",key.Index+1)}
   }
  }
  if !found{t.Errorf("folder %q has no Back key",page.Name)}
 }
}

func TestMiraboxLegacyBackKeyMigratesOffDisplayEighteen(t *testing.T) {
 d:=testDevice(t)
 layout:=testNestedFolderLayout(d.Model)
 for pageIndex:=range layout.Pages {
  for keyIndex:=range layout.Pages[pageIndex].Keys{
   if layout.Pages[pageIndex].Keys[keyIndex].Action=="folder-back"{
    layout.Pages[pageIndex].Keys[keyIndex].Index=17 // old test build
   }
  }
 }
 // Reproduce an older persisted JSON file that predates display-only validation.
 data,err:=json.Marshal(layout)
 if err!=nil{t.Fatal(err)}
 filename:=layoutFilename(d.root,d.Serial)
 if err:=os.MkdirAll(filepath.Dir(filename),0700);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filename,data,0600);err!=nil{t.Fatal(err)}
 migrated,err:=LoadLayout(d.root,d.Serial,d.Model)
 if err!=nil{t.Fatalf("old layout failed to load: %v",err)}
 for _,page:=range migrated.Pages {
  if page.ParentPage==""{continue}
  for _,key:=range page.Keys {
   if key.Action=="folder-back" && key.Index!=16 {
    t.Fatalf("old Back migrated to %d instead of physical button 17",key.Index+1)
   }
  }
 }
}

func TestMiraboxLegacyDisplayActionsDoNotEraseArtwork(t *testing.T){
 d:=testDevice(t)
 l:=DefaultLayout(d.Model)
 l.Pages[0].Keys=[]KeyConfig{{Index:5,Label:"Status",Action:"page-next",Icon:strings.Repeat("a",32)}}
 migrateDisplayOnlyActions(&l,d.Model)
 if err:=l.Validate(d.Model);err!=nil{t.Fatal(err)}
 k:=l.Pages[0].Keys[0]
 if k.Label!="Status"||k.Icon!=strings.Repeat("a",32)||k.Action!=""{
  t.Fatalf("passive display lost content or retained unusable action: %+v",k)
 }
}

func TestBackArrowPointsLeftOnPhysicalLCDArtwork(t *testing.T) {
 arrow:=renderDeckFolderKey(96,KeyConfig{Action:"folder-back"})
 blank:=renderKeyLabel(96,"")
 differs:=func(x,y int)bool {
  ar,ag,ab,aa:=arrow.At(x,y).RGBA()
  br,bg,bb,ba:=blank.At(x,y).RGBA()
  return ar!=br||ag!=bg||ab!=bb||aa!=ba
 }
 // Scale 48x48 glyph into the centered 76x76 area. The arrow tip is
 // at (7,24) and the shaft runs horizontally to the right.
 if !differs(22,48) {t.Fatal("left arrow tip is missing")}
 if differs(22,29) {t.Fatal("left arrow points diagonally/has old vertical strip")}
 if !differs(65,48) {t.Fatal("arrow shaft is missing")}
}

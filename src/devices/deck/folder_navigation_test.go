package deck

import (
 "strings"
 "testing"
)

func testNestedFolderLayout(m Model) Layout {
 return Layout{Version:1,ActivePage:"home",Pages:[]Page{
  {ID:"home",Name:"Home",Keys:[]KeyConfig{
   {Index:0,Action:"folder-open",TargetPage:"games"},
   {Index:1,Action:"page-next"},
  }},
  {ID:"games",Name:"Games",ParentPage:"home",Keys:[]KeyConfig{
   {Index:0,Action:"folder-open",TargetPage:"gaming-tools"},
   {Index:17,Action:"folder-back"},
  }},
  {ID:"gaming-tools",Name:"Gaming Tools",ParentPage:"games",Keys:[]KeyConfig{
   {Index:17,Action:"folder-back"},
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
  {0,"games"},{0,"gaming-tools"},{17,"games"},{17,"home"},
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
 restarted.handleKeyPress(17)
 if got:=restarted.GetStatus().ActivePage;got!="games"{
  t.Fatalf("Back after restart navigated to %q, want games",got)
 }
 restarted.handleKeyPress(17)
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
 l.Pages=append(l.Pages,Page{ID:"second",Name:"Second",Keys:[]KeyConfig{{Index:17,Action:"page-go",TargetPage:"home"}}})
 if err:=l.Validate(d.Model);err!=nil{t.Fatalf("v1 pages without folder metadata failed: %v",err)}
}

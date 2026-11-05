package server
import(
 "bytes"
 "os/exec"
 "testing"
)
func TestSVGIconsAllowStaticPathsAndBlockActiveContent(t *testing.T){
 valid:=[]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2 L22 12 L2 22 Z"/></svg>`)
 if err:=validateIconSVG(valid);err!=nil{t.Fatalf("valid static SVG rejected: %v",err)}
 for _,bad:=range []string{
  `<!DOCTYPE svg [<!ENTITY e SYSTEM "file:///etc/passwd">]><svg>&e;</svg>`,
  `<svg><script>alert(1)</script></svg>`,
  `<svg><image href="http://example.org/icon.png"/></svg>`,
  `<svg onload="alert(1)"><path d="M0 0"/></svg>`,
  `<svg><path fill="url(https://example.org/image.png)"/></svg>`,
  `<?xml-stylesheet href="http://example.org/a.css"?><svg/>`,
 }{
  if err:=validateIconSVG([]byte(bad));err==nil{t.Errorf("accepted unsafe SVG: %s",bad)}
 }
 if _,err:=rasterizeIconSVG([]byte("<svg><script/></svg>"));err==nil{t.Fatal("unsafe file reached renderer")}
 if _,err:=exec.LookPath("rsvg-convert");err==nil{
  png,err:=rasterizeIconSVG(valid)
  if err!=nil||!bytes.HasPrefix(png,[]byte{137,80,78,71}){t.Fatalf("SVG not rasterized: %v",err)}
 }
}

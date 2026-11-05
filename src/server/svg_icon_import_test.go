package server
import(
 "archive/zip"
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

 // These variants previously bypassed the literal "url(" check in
 // presentation attributes. CSS processors can decode escaped identifiers.
 for name,markup:=range map[string]string{
  "escaped inline resource":`<svg><path d="M0 0" style="fill:\75rl(https://example.org/secret)"/></svg>`,
  "escaped presentation resource":`<svg><path d="M0 0" fill="\75rl(file:///etc/passwd)"/></svg>`,
  "unrecognized stylesheet property":`<svg><rect width="20" height="20" style="background-image:url(#paint)"/></svg>`,
  "CSS comments":`<svg><rect width="20" height="20" style="fill:/*comment*/red"/></svg>`,
  "foreign SVG namespace":`<svg xmlns="https://example.org/not-svg"><path d="M0 0"/></svg>`,
  "unapproved resource attribute":`<svg><path d="M0 0" filter="https://example.org/paint"/></svg>`,
 }{
  t.Run(name,func(t *testing.T){
   if err:=validateIconSVG([]byte(markup));err==nil{t.Errorf("unsafe SVG accepted: %s",markup)}
  })
 }
 for name,markup:=range map[string]string{
  "common presentation style":`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1 L12 12" style="fill:none;stroke:currentColor;stroke-width:2;stroke-linejoin:round"/></svg>`,
  "internal gradient":`<svg xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="tone"><stop offset="0" stop-color="#ff0000"/></linearGradient></defs><rect width="24" height="24" fill="url(#tone)"/></svg>`,
  "internal styled gradient":`<svg xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="tone"><stop offset="0" stop-color="#ff0000"/></linearGradient></defs><rect width="24" height="24" style="fill:url(#tone);opacity:.8"/></svg>`,
 }{
  t.Run(name,func(t *testing.T){
   if err:=validateIconSVG([]byte(markup));err!=nil{t.Errorf("safe SVG rejected: %v",err)}
  })
 }

 if _,err:=rasterizeIconSVG([]byte("<svg><script/></svg>"));err==nil{t.Fatal("unsafe file reached renderer")}
 if _,err:=exec.LookPath("rsvg-convert");err==nil{
  png,err:=rasterizeIconSVG(valid)
  if err!=nil||!bytes.HasPrefix(png,[]byte{137,80,78,71}){t.Fatalf("SVG not rasterized: %v",err)}
 }
}

func TestSVGIconPackImportRendersForBothDevices(t *testing.T) {
 if !svgConverterAvailable() {t.Skip("install librsvg2-bin for SVG import integration")}
 var buf bytes.Buffer
 archive:=zip.NewWriter(&buf)
 icon,err:=archive.Create("symbols/play.svg")
 if err!=nil{t.Fatal(err)}
 _,err=icon.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#ff0000" d="M2 2 L22 12 L2 22 Z"/></svg>`))
 if err!=nil{t.Fatal(err)}
 if err=archive.Close();err!=nil{t.Fatal(err)}
 pack,err:=importIconSet(t.TempDir(),"Vectors",buf.Bytes())
 if err!=nil{t.Fatal(err)}
 if len(pack.Icons)!=1||pack.Icons[0].DeckID==""||pack.Icons[0].NexusID==""{
  t.Fatalf("SVG conversion did not produce reusable LCD icons: %+v",pack)
 }
}

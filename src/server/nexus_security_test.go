package server

import (
 "crypto/tls"
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestNexusWriteRequiresSameOriginBrowser(t *testing.T){
 cases:=[]struct{
  name,origin,fetchSite string
  tls bool
  allow bool
 }{
  {"same origin","http://localhost:27003","same-origin",false,true},
  {"same origin HTTPS","https://localhost:27003","same-origin",true,true},
  {"direct CLI no Origin","","",false,true},
  {"direct navigation fetch none","","none",false,true},
  {"cross site","http://attacker.example","cross-site",false,false},
  {"missing Origin cross-site","","cross-site",false,false},
  {"same-site sibling subdomain","","same-site",false,false},
  {"same-site explicitly matched","http://localhost:27003","same-site",false,false},
  {"malformed origin","null","same-origin",false,false},
  {"spoofed path","http://localhost:27003/extra","same-origin",false,false},
  {"spoofed query","http://localhost:27003?x=1","same-origin",false,false},
  {"spoofed credentials","http://user@localhost:27003","same-origin",false,false},
  {"spoofed scheme","file://localhost:27003","same-origin",false,false},
  {"scheme downgrade on TLS","http://localhost:27003","same-origin",true,false},
  {"wrong port","https://localhost:27004","same-origin",true,false},
 }
 for _,tc:=range cases{
  t.Run(tc.name,func(t *testing.T){
   req:=httptest.NewRequest(http.MethodPost,"http://localhost:27003/api/nexus/background",nil)
   if tc.tls{req.TLS=&tls.ConnectionState{}}
   if tc.origin!=""{req.Header.Set("Origin",tc.origin)}
   if tc.fetchSite!=""{req.Header.Set("Sec-Fetch-Site",tc.fetchSite)}
   if got:=nexusWriteAllowed(req);got!=tc.allow{
    t.Fatalf("Origin %q Sec-Fetch-Site %q TLS %t: got %t want %t",tc.origin,tc.fetchSite,tc.tls,got,tc.allow)
   }
  })
 }
}

func TestNexusUploadMediaTypes(t *testing.T){
 for _,typ:=range []string{"image/png","image/jpeg","image/gif","image/png; charset=utf-8"}{
  if !nexusUploadMediaType(typ){t.Errorf("valid MIME %q rejected",typ)}
 }
 for _,typ:=range []string{"","text/plain","application/octet-stream","image/svg+xml","image/png.fake","image/png;invalid","image/jpeg+xml"}{
  if nexusUploadMediaType(typ){t.Errorf("unsafe MIME %q accepted",typ)}
 }
}

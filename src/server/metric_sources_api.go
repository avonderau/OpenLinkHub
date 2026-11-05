package server

import (
 "encoding/json"
 "net/http"

 "OpenLinkHub/src/devices/nexus"
)

// Read-only host source discovery is shared by the Nexus and Deck editors.
// It reveals only validated currently visible Linux mount/device identifiers.
func hostMetricSourcesAPI(w http.ResponseWriter,r *http.Request) {
 if r.Method!=http.MethodGet {
  w.Header().Set("Allow","GET")
  http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
  return
 }
 if !nexusRequestHostAllowed(r) {
  http.Error(w,"host not allowed",http.StatusForbidden)
  return
 }
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","private, no-store")
 w.Header().Set("X-Content-Type-Options","nosniff")
 _=json.NewEncoder(w).Encode(map[string]any{"sources":nexus.DiscoverMetricSources()})
}

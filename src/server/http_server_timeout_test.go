package server

import (
	"net/http"
	"testing"
	"time"
)

func TestWebServerLimitsSlowClients(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	s := newWebServer("127.0.0.1:27003", handler)
	if s.Addr != "127.0.0.1:27003" || s.Handler == nil {
		t.Fatal("server must preserve the requested listen address and application handler")
	}
	if s.ReadHeaderTimeout <= 0 || s.ReadHeaderTimeout > 10*time.Second {
		t.Fatalf("unbounded or excessive HTTP header timeout: %s", s.ReadHeaderTimeout)
	}
	if s.ReadTimeout < s.ReadHeaderTimeout || s.ReadTimeout > 2*time.Minute {
		t.Fatalf("missing or excessive request read timeout: %s", s.ReadTimeout)
	}
	if s.WriteTimeout <= 0 || s.WriteTimeout > 3*time.Minute {
		t.Fatalf("missing or excessive response write timeout: %s", s.WriteTimeout)
	}
	if s.IdleTimeout <= 0 || s.IdleTimeout > 2*time.Minute {
		t.Fatalf("missing or excessive idle keep-alive timeout: %s", s.IdleTimeout)
	}
	if s.MaxHeaderBytes <= 0 || s.MaxHeaderBytes > 64<<10 {
		t.Fatalf("unbounded request headers: %d bytes", s.MaxHeaderBytes)
	}
}

package server

import (
	"net/http/httptest"
	"testing"
)

func TestDeckPluginInstallOnlyFromThisComputer(t *testing.T) {
	cases := []struct {
		remote string
		header string
		want   bool
	}{
		{"127.0.0.1:5000", "", true},
		{"[::1]:5000", "", true},
		{"192.168.1.20:5000", "", false},
		{"127.0.0.1:5000", "X-Forwarded-For", false},
		{"127.0.0.1:5000", "Forwarded", false},
		{"garbage", "", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/api/deck/plugins", nil)
		r.RemoteAddr = c.remote
		if c.header != "" {
			r.Header.Set(c.header, "203.0.113.9")
		}
		if got := deckPluginInstallAllowed(r); got != c.want {
			t.Errorf("%s %s: got %v", c.remote, c.header, got)
		}
	}
}

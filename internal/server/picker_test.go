package server

import (
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestLocalPeerAndPickerAvailability(t *testing.T) {
	for _, tc := range []struct {
		address string
		local   bool
	}{
		{"127.0.0.1:1234", true}, {"[::1]:1234", true}, {"192.0.2.1:1234", false}, {"invalid", false}, {"example.com:1234", false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://localhost/api/paths", nil)
			r.RemoteAddr = tc.address
			if got := localPeer(r); got != tc.local {
				t.Fatalf("local=%v, want %v", got, tc.local)
			}
			if got := localPickerAvailable(r); got != (tc.local && runtime.GOOS == "windows") {
				t.Fatalf("picker=%v", got)
			}
		})
	}
}

func TestPickerAccessGuards(t *testing.T) {
	s := New(t.TempDir())
	defer s.Close()
	handler := s.Handler("127.0.0.1:8765")
	for _, tc := range []struct {
		name, method, address, kind string
		auth                        bool
		want                        int
	}{
		{"unauthenticated", "POST", "127.0.0.1:1234", "files", false, 401},
		{"remote peer", "POST", "192.0.2.1:1234", "files", true, 403},
		{"forwarded loopback untrusted", "POST", "192.0.2.1:1234", "directory", true, 403},
		{"invalid method", "GET", "127.0.0.1:1234", "files", true, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://127.0.0.1:8765/api/pick-paths?kind="+tc.kind, strings.NewReader(""))
			r.RemoteAddr = tc.address
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			if tc.auth {
				r.Header.Set("Authorization", "Bearer "+s.Token())
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body.String(), tc.want)
			}
		})
	}
	if runtime.GOOS == "windows" {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/pick-paths?kind=invalid", nil)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("invalid picker kind: %d", w.Code)
		}
	}
}

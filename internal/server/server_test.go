package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthAndOrigin(t *testing.T) {
	s := New(t.TempDir())
	h := s.Handler("127.0.0.1:8765")
	cases := []struct {
		token, host, origin string
		status              int
	}{{"", "127.0.0.1:8765", "", 401}, {s.Token(), "127.0.0.1:8765", "", 200}, {s.Token(), "evil.test:8765", "", 403}, {s.Token(), "127.0.0.1:8765", "https://evil.test", 403}}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/status", nil)
		r.Host = c.host
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		r.Header.Set("Origin", c.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%+v status %d", c, w.Code)
		}
	}
}
func TestAPIWorkflow(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "demo.log")
	os.WriteFile(p, []byte("2026-10-14 12:30:00 ERROR connection failed\n"), 0600)
	s := New(dir)
	h := s.Handler("127.0.0.1:8765")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8765"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if err := s.Start([]string{p}, false, nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.RLock()
		busy := s.busy
		s.mu.RUnlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("analysis timed out")
		}
		time.Sleep(time.Millisecond)
	}
	for _, path := range []string{"/api/groups", "/api/details", "/api/export", "/api/report", "/api/browse"} {
		w := call("GET", path, "")
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := call("GET", "/api/groups?regex=%5B", ""); w.Code != 400 {
		t.Fatal("bad regex response", w.Code)
	}
	if w := call("POST", "/api/upload?name=small.log", "WARN example\n"); w.Code != 200 {
		t.Fatal("upload", w.Body.String())
	}
	if w := call("GET", "/api/details?limit=101", ""); w.Code != 400 {
		t.Fatal("bad pagination", w.Code)
	}
	for _, path := range []string{"/api/details?before=-1", "/api/details?after=10001", "/api/details?before=abc"} {
		if w := call("GET", path, ""); w.Code != 400 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if w := call("GET", "/api/details?before=0&after=0", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"before":""`) || !strings.Contains(w.Body.String(), `"after":""`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

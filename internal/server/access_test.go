package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientPolicy(t *testing.T) {
	prefixes, err := parseClientIPs([]string{"192.0.2.5", "10.1.0.0/16", "2001:db8::/32", "::1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		remote  string
		allowed bool
	}{
		{"192.0.2.5:99", true}, {"192.0.2.6:99", false}, {"10.1.255.255:1", true}, {"10.2.0.0:1", false},
		{"[2001:db8::1]:99", true}, {"[2001:db9::1]:99", false}, {"[::ffff:192.0.2.5]:99", true}, {"[::1]:99", true}, {"127.0.0.1:99", false}, {"bad", false},
	} {
		if got := clientAllowed(c.remote, prefixes); got != c.allowed {
			t.Errorf("%s: %v", c.remote, got)
		}
	}
	for _, value := range []string{"", "example.com", "10.0.0.1/33", "fe80::1%eth0", "::ffff:192.0.2.0/120"} {
		if _, err := parseClientIPs([]string{value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func accessRequest(s *Server, method, path, remote, authorization, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	r.RemoteAddr = remote
	if authorization != "" {
		r.Header.Set("Authorization", authorization)
	}
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	r.Header.Set("X-Real-IP", "127.0.0.1")
	w := httptest.NewRecorder()
	s.Handler("0.0.0.0:8765").ServeHTTP(w, r)
	return w
}

func TestAccessPersistenceAndAuthentication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := New(t.TempDir())
	if err := s.ConfigureAccess(path, []string{"127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	for _, auth := range []string{"", s.Token(), "Basic " + s.Token(), "Bearer ", "Bearer wrong"} {
		if w := accessRequest(s, "GET", "/api/access-settings", "127.0.0.1:9", auth, ""); w.Code != 401 {
			t.Fatalf("auth: %d", w.Code)
		}
	}
	for _, route := range []string{"/", "/app.js", "/api/status", "/api/access-settings"} {
		if w := accessRequest(s, "GET", route, "192.0.2.1:9", "Bearer "+s.Token(), ""); w.Code != 403 {
			t.Fatalf("bypass %s: %d", route, w.Code)
		}
	}
	for _, body := range []string{`{"allowed_client_ips":["192.0.2.1"]}`, `{"allowed_client_ips":[""]}`, `{}`, `null`, `{"allowed_client_ips":null}`, `{"allowed_client_ips":[]} {}`} {
		if w := accessRequest(s, "PUT", "/api/access-settings", "127.0.0.1:9", "Bearer "+s.Token(), body); w.Code != 400 {
			t.Fatalf("invalid update %s: %d", body, w.Code)
		}
	}
	w := accessRequest(s, "PUT", "/api/access-settings", "127.0.0.1:9", "Bearer "+s.Token(), `{"allowed_client_ips":["127.0.0.1","10.0.0.0/8"]}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	restarted := New(t.TempDir())
	if err := restarted.ConfigureAccess(path, nil); err != nil {
		t.Fatal(err)
	}
	if !restarted.allowedClient("10.2.3.4:9") || restarted.allowedClient("192.0.2.1:9") {
		t.Fatal("persistence failed")
	}
	if w := accessRequest(restarted, "GET", "/api/status", "127.0.0.1:9", "Bearer "+s.Token(), ""); w.Code != 401 {
		t.Fatal("old token accepted")
	}
	if err := restarted.ConfigureAccess(path, []string{}); err != nil {
		t.Fatal(err)
	}
	if !restarted.allowedClient("192.0.2.1:9") {
		t.Fatal("disable failed")
	}
}

func TestAccessStateFailure(t *testing.T) {
	for _, data := range []string{"broken", `{}`, `null`, `{"allowed_client_ips":["bad"]}`} {
		path := filepath.Join(t.TempDir(), "state.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := New(t.TempDir()).ConfigureAccess(path, nil); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	s := New(t.TempDir())
	s.access.path = filepath.Join(t.TempDir(), "missing", "state.json")
	w := accessRequest(s, "PUT", "/api/access-settings", "127.0.0.1:9", "Bearer "+s.Token(), `{"allowed_client_ips":["127.0.0.1"]}`)
	if w.Code != 500 || !s.allowedClient("192.0.2.1:9") {
		t.Fatal("failed save changed policy")
	}
}

func TestDuplicateAuthorizationRejected(t *testing.T) {
	s := New(t.TempDir())
	r := httptest.NewRequest("GET", "http://localhost/api/status", nil)
	r.Header.Add("Authorization", "Bearer "+s.Token())
	r.Header.Add("Authorization", "Bearer "+s.Token())
	w := httptest.NewRecorder()
	s.Handler("0.0.0.0:8765").ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("duplicate authorization accepted: %d", w.Code)
	}
}

package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRemoteHostAndOrigin(t *testing.T) {
	cases := []struct {
		bind, host, origin string
		allowed            []string
		want               int
	}{
		{"0.0.0.0:8765", "192.168.1.20:8765", "http://192.168.1.20:8765", nil, 200},
		{"[::]:8765", "[2001:db8::1]:8765", "http://[2001:db8::1]:8765", nil, 200},
		{"0.0.0.0:8765", "evil.example:8765", "", nil, 403},
		{"0.0.0.0:8765", "192.168.1.20:8765", "http://evil.example:8765", nil, 403},
		{"0.0.0.0:8765", "192.168.1.20:8765", "null", nil, 403},
		{"0.0.0.0:8765", "logs.example.com", "https://logs.example.com", []string{"logs.example.com"}, 200},
		{"127.0.0.1:8765", "localhost:8765", "http://localhost:8765", nil, 200},
		{"127.0.0.1:8765", "192.168.1.20:8765", "", nil, 403},
		{"0.0.0.0:8765", "192.168.1.20:8765", "http://192.168.1.20:8765/path", nil, 403},
	}
	for _, c := range cases {
		s := New(t.TempDir())
		s.allowedHosts = c.allowed
		h := s.Handler(c.bind)
		r := httptest.NewRequest("GET", "http://localhost/api/status", nil)
		r.Host = c.host
		r.Header.Set("Origin", c.origin)
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Fatalf("%+v: got %d: %s", c, w.Code, w.Body.String())
		}
	}
}

func TestWildcardListener(t *testing.T) {
	s := New(t.TempDir())
	l, srv, err := ServeHost(s, "0.0.0.0", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go srv.Serve(l)
	defer srv.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+port+"/api/status", nil)
	req.Host = "192.168.1.20:" + port
	client := &http.Client{Timeout: time.Second * 5}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("remote API must require token", res.StatusCode)
	}
	req.Header.Set("Authorization", "Bearer "+s.Token())
	req.Header.Set("Origin", "http://"+req.Host)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("authenticated remote API", res.StatusCode)
	}
}

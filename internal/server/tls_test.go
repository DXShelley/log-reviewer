package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTLSBundlePersistenceAndNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls.pem")
	config, err := TLSConfig("", "", path, []string{"logs.example.test", "0.0.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if config.MinVersion != tls.VersionTLS12 {
		t.Fatal("minimum TLS version")
	}
	for _, name := range []string{"localhost", "127.0.0.1", "::1", "logs.example.test"} {
		if err := config.Certificates[0].Leaf.VerifyHostname(name); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := os.ReadFile(path)
	if _, err := TLSConfig("", "", path, []string{"logs.example.test"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("certificate changed on restart")
	}
	if _, err := TLSConfig(path, path, "", nil); err != nil {
		t.Fatal("custom pair", err)
	}
	if _, err := TLSConfig(path, "", "", nil); err == nil {
		t.Fatal("accepted missing private key")
	}
	if _, err := TLSConfig("", "", path, []string{"new.example.test"}); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if bytes.Equal(before, after) {
		t.Fatal("SAN not refreshed")
	}
}

func TestTLSBundleFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls.pem")
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := TLSConfig("", "", path, nil); err == nil {
		t.Fatal("corrupt bundle accepted")
	}
	if _, err := TLSConfig("", "", filepath.Join(t.TempDir(), "missing", "tls.pem"), nil); err == nil {
		t.Fatal("unwritable bundle accepted")
	}
}

func TestExpiredTLSCertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls.pem")
	config, err := TLSConfig("", "", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := config.Certificates[0].PrivateKey.(*ecdsa.PrivateKey)
	certificate := *config.Certificates[0].Leaf
	certificate.NotBefore = time.Now().Add(-48 * time.Hour)
	certificate.NotAfter = time.Now().Add(-24 * time.Hour)
	der, err := x509.CreateCertificate(rand.Reader, &certificate, &certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	data := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := TLSConfig(path, path, "", nil); err == nil {
		t.Fatal("expired explicit certificate accepted")
	}
	renewed, err := TLSConfig("", "", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !renewed.Certificates[0].Leaf.NotAfter.After(time.Now().Add(24 * time.Hour)) {
		t.Fatal("automatic certificate not renewed")
	}
}

func TestHTTPSAccessControls(t *testing.T) {
	config, err := TLSConfig("", "", filepath.Join(t.TempDir(), "tls.pem"), nil)
	if err != nil {
		t.Fatal(err)
	}
	s := New(t.TempDir())
	if err := s.ConfigureAccess(filepath.Join(t.TempDir(), "state.json"), []string{"127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	l, srv, err := ServeHost(s, "127.0.0.1", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(tls.NewListener(l, config))
	pool := x509.NewCertPool()
	pool.AddCert(config.Certificates[0].Leaf)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, c := range []struct {
		token, origin string
		want          int
	}{{"", "", 401}, {s.Token(), "https://evil.example", 403}, {s.Token(), "https://" + l.Addr().String(), 200}} {
		r, _ := http.NewRequest("GET", "https://"+l.Addr().String()+"/api/status", nil)
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Fatalf("status %d want %d", res.StatusCode, c.want)
		}
	}
	s.access.mu.Lock()
	s.access.prefixes, _ = parseClientIPs([]string{"192.0.2.1"})
	s.access.mu.Unlock()
	res, err := client.Get("https://" + l.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("HTTPS bypassed IP policy")
	}
}

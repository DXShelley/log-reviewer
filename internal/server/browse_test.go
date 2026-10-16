package server

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestDeploymentBrowser(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app.log", "隐藏 [01].txt", "private.csv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := New(t.TempDir())
	defer s.Close()
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, path string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://127.0.0.1:8765/api/browse?path="+url.QueryEscape(path), nil)
		if auth {
			req.Header.Set("Authorization", "Bearer "+s.Token())
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	if res := call("GET", dir, false); res.Code != 401 {
		t.Fatal(res.Code)
	}
	if res := call("POST", dir, true); res.Code != 405 {
		t.Fatal(res.Code)
	}
	if res := call("GET", filepath.Join(dir, "missing"), true); res.Code != 400 {
		t.Fatal(res.Code)
	}
	res := call("GET", dir, true)
	if res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	var result struct {
		Path    string        `json:"path"`
		Parent  string        `json:"parent"`
		Roots   []string      `json:"roots"`
		Entries []browseEntry `json:"entries"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != dir || result.Parent != filepath.Dir(dir) || len(result.Roots) == 0 || len(result.Entries) != 3 || !result.Entries[0].Directory {
		t.Fatalf("%+v", result)
	}
	for _, entry := range result.Entries {
		if entry.Path != filepath.Join(dir, entry.Name) || entry.Name == "private.csv" {
			t.Fatalf("%+v", entry)
		}
	}
}

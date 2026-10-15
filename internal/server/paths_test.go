package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPathResolution(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "logs [production]")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "app[01].log")
	if err := os.WriteFile(file, []byte("2026-10-15 12:30:00 ERROR app : failed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := New(t.TempDir())
	defer s.Close()
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, route, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8765/api/"+route, strings.NewReader(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+s.Token())
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "paths", "", false); w.Code != 401 {
		t.Fatal("unauthenticated path access", w.Code)
	}
	var context struct {
		WorkingDirectory string `json:"working_directory"`
		OS               string `json:"os"`
	}
	w := call("GET", "paths", "", true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &context) != nil || context.WorkingDirectory == "" || context.OS == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"paths": []string{"'" + file + "'", dir, filepath.Join(dir, "*.log")}})
	w = call("POST", "paths", string(body), true)
	var result struct {
		Paths []string `json:"paths"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Paths) != 1 || result.Paths[0] != file {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, body := range []string{`{}`, `{"paths":[]}`, `{"paths":["missing.log"]}`, `{"paths":[],"unknown":true}`, `{} {}`} {
		if w := call("POST", "paths", body, true); w.Code != 400 {
			t.Fatal("invalid path request accepted", body, w.Code)
		}
	}
	if w := call("DELETE", "paths", "", true); w.Code != 405 {
		t.Fatal(w.Code)
	}
	body, _ = json.Marshal(map[string]any{"paths": result.Paths})
	if w := call("POST", "analyze", string(body), true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	s.jobs.Wait()
	if s.Report == nil || s.Report.Events != 1 || len(s.uploads) != 0 {
		t.Fatal("resolved paths did not analyze without upload")
	}
}

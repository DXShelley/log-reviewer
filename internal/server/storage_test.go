package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/remote"
)

func TestStorageMigrationPreservesAnalysis(t *testing.T) {
	s := New(t.TempDir())
	defer s.Close()
	s.remoteDir = filepath.Join(t.TempDir(), "old-session")
	oldRoot := s.remoteDir
	dir := filepath.Join(oldRoot, "task")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "app.log")
	text := "2026-10-15 12:30:00 ERROR app : migration test\n"
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	s.uploads["id"] = file
	s.remoteSources[file] = logs.RemoteSource{Protocol: "sftp", Host: "server", Path: "/var/log/app.log"}
	manifest := remote.Manifest{Files: []remote.File{{RemotePath: "/var/log/app.log", LocalPath: file}}}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600)
	cache := &remote.DirectSource{CacheDir: filepath.Join(oldRoot, "fallback")}
	s.directSources["remote-key"] = cache
	if err := s.Start([]string{file}, false, nil); err != nil {
		t.Fatal(err)
	}
	s.jobs.Wait()
	if s.Report == nil {
		t.Fatal(s.err)
	}
	state := filepath.Join(t.TempDir(), "state.json")
	s.access.path = state
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, route string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://127.0.0.1:8765/api/"+route, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	target := t.TempDir()
	w := call("PUT", "import-settings", importSettings{RemoteDirectory: target, FilePatterns: []string{"*.text", "*.log"}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	moved := s.uploads["id"]
	if !insidePath(target, moved) || moved == file {
		t.Fatal(moved)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("old file retained", err)
	}
	if body, err := os.ReadFile(moved); err != nil || string(body) != text {
		t.Fatal(string(body), err)
	}
	if s.Report.Sources[0].Path != moved || s.Report.Sources[0].Remote.Path != "/var/log/app.log" {
		t.Fatal(s.Report.Sources)
	}
	if cache.CacheDir != filepath.Join(s.remoteDir, "fallback") {
		t.Fatal(cache.CacheDir)
	}
	w = call("GET", "details?group="+s.Report.Groups[0].ID, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "migration test") {
		t.Fatal(w.Code, w.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(moved), "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil || manifest.Files[0].LocalPath != moved {
		t.Fatal(string(data), err)
	}
	w = call("PUT", "access-settings", map[string]any{"allowed_client_ips": []string{}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	restored := New(t.TempDir())
	defer restored.Close()
	if err := restored.ConfigureAccess(state, nil); err != nil {
		t.Fatal(err)
	}
	if restored.access.config.RemoteDirectory != target || len(restored.filePatterns()) != 2 {
		t.Fatal(restored.access.config)
	}
}

func TestStorageMigrationRollbackAndFormats(t *testing.T) {
	s := New(t.TempDir())
	defer s.Close()
	s.remoteDir = filepath.Join(t.TempDir(), "old")
	os.MkdirAll(s.remoteDir, 0700)
	file := filepath.Join(s.remoteDir, "app.text")
	os.WriteFile(file, []byte("original"), 0600)
	s.uploads["id"] = file
	s.access.path = filepath.Join(t.TempDir(), "missing", "state.json")
	target := t.TempDir()
	body, _ := json.Marshal(importSettings{RemoteDirectory: target, FilePatterns: []string{"*.text"}})
	r := httptest.NewRequest("PUT", "/", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	s.importSettings(w, r)
	if w.Code != 500 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("source lost", err)
	}
	if s.uploads["id"] != file {
		t.Fatal("reference changed on failure")
	}
	if _, err := os.Stat(s.sessionRemoteDirectory(target)); !os.IsNotExist(err) {
		t.Fatal("staged target retained")
	}
	os.WriteFile(filepath.Join(s.remoteDir, "not-a-log.png"), []byte("image"), 0600)
	paths, err := s.expandImportPaths([]string{s.remoteDir})
	if err != nil || len(paths) != 1 || paths[0] != file {
		t.Fatal(paths, err)
	}
	s.access.config.FilePatterns = []string{"*.png"}
	paths, err = s.expandImportPaths([]string{s.remoteDir})
	if err != nil || len(paths) != 1 || filepath.Ext(paths[0]) != ".png" {
		t.Fatal(paths, err)
	}
}

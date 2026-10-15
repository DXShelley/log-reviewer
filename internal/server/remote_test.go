package server

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/remote"
	"log-reviewer/internal/testsupport/sftpfixture"
)

func TestRemoteAPIAnalysisAndProvenance(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "app.log"), []byte("2026-10-14 12:30:00 ERROR remote failed\n"), 0600)
	fixture, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	s := New(t.TempDir())
	defer s.Close()
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8765/api/remote-fetch", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("POST", "{}"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	host, port, _ := net.SplitHostPort(fixture.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	options := remote.Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: fixture.Fingerprint, Path: "app.log"}
	body, _ := json.Marshal(options)
	if w := call("POST", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		s.mu.RLock()
		busy := s.remoteState.Busy
		s.mu.RUnlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("download timeout")
		}
		time.Sleep(time.Millisecond)
	}
	s.mu.RLock()
	state := s.remoteState
	var files []string
	for _, p := range s.uploads {
		files = append(files, p)
	}
	s.mu.RUnlock()
	if state.Error != "" || len(state.Downloads) != 1 {
		t.Fatalf("%+v", state)
	}
	status := call("GET", "").Body.String()
	if strings.Contains(status, "fixture-password") || strings.Contains(status, "private_key") {
		t.Fatal("credentials leaked")
	}
	if err := s.Start(files, false, nil); err != nil {
		t.Fatal(err)
	}
	s.jobs.Wait()
	if s.Report == nil || s.Report.Sources[0].Remote == nil || s.Report.Sources[0].Remote.Path != "app.log" {
		t.Fatalf("%+v", s.Report)
	}
	details, _, err := logs.Details(s.Report, logs.Filter{}, 0, 10)
	if err != nil || len(details) != 1 || details[0].Remote == nil {
		t.Fatalf("%+v %v", details, err)
	}
}

func TestRemoteAPICancel(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	s := New(t.TempDir())
	defer s.Close()
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8765/api/remote-fetch", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	bytes, _ := json.Marshal(remote.Options{Host: host, Port: number, User: "tester", Password: "secret", Fingerprint: "SHA256:expected", Path: "logs"})
	if w := call("POST", string(bytes)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("connect timeout")
	}
	if w := call("POST", string(bytes)); w.Code != 409 {
		t.Fatal("concurrent fetch accepted", w.Code)
	}
	if err := s.Start([]string{"nonexistent.log"}, false, nil); err == nil {
		t.Fatal("analysis accepted during transfer")
	}
	if w := call("DELETE", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	s.jobs.Wait()
	if s.remoteState.Busy || s.remoteState.Error == "" || len(s.uploads) != 0 {
		t.Fatalf("%+v", s.remoteState)
	}
}

func TestRemotePreviewSelectionAndHistory(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"app.log", "worker.log"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("ERROR example\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	s := New(t.TempDir())
	defer s.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := s.ConfigureAccess(statePath, nil); err != nil {
		t.Fatal(err)
	}
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, route string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "http://127.0.0.1:8765/api/"+route, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	host, port, _ := net.SplitHostPort(fixture.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	request := remoteRequest{Action: "preview", Options: remote.Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Path: ".", SSHDir: filepath.Join(t.TempDir(), "unknown"), MaxFiles: 1}}
	if w := call("POST", "remote-fetch", request); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.remoteState.HostKey == nil {
		t.Fatalf("expected host challenge: %+v", s.remoteState)
	}
	if len(s.access.config.RemoteHosts) != 0 {
		t.Fatal("unverified connection saved")
	}
	request.Fingerprint = s.remoteState.HostKey.Fingerprint
	if w := call("POST", "remote-fetch", request); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	preview := s.remoteState
	if preview.Error != "" || preview.Stage != "preview" || len(preview.Files) != 2 || len(s.uploads) != 0 {
		t.Fatalf("%+v", preview)
	}
	if entries, _ := os.ReadDir(filepath.Join(s.dir, "remote")); len(entries) != 0 {
		t.Fatal("preview downloaded files")
	}
	for _, paths := range [][]string{{}, {"app.log", "worker.log"}, {"../outside.log"}, {"app.log", "app.log"}} {
		w := call("POST", "remote-fetch", remoteRequest{Action: "download", PreviewID: preview.PreviewID, SelectedPaths: paths})
		if w.Code != 400 {
			t.Fatalf("invalid selection accepted: %v %s", paths, w.Body.String())
		}
	}
	saved, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "fixture-password") || strings.Contains(string(saved), "private_key") || strings.Contains(call("GET", "remote-fetch", nil).Body.String(), "fixture-password") {
		t.Fatal("credential leaked")
	}
	if w := call("PUT", "access-settings", map[string]any{"allowed_client_ips": []string{}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	restarted := New(t.TempDir())
	defer restarted.Close()
	if err := restarted.ConfigureAccess(statePath, nil); err != nil {
		t.Fatal(err)
	}
	if len(restarted.access.config.RemoteHosts) != 1 || restarted.access.config.RemoteHosts[0].Fingerprint != fixture.Fingerprint {
		t.Fatal("history lost after access update/restart")
	}
	if w := call("POST", "remote-fetch", remoteRequest{Action: "download", PreviewID: preview.PreviewID, SelectedPaths: []string{"app.log"}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.remoteState.Error != "" || len(s.remoteState.Downloads) != 1 || s.remoteState.Downloads[0].RemotePath != "app.log" {
		t.Fatalf("%+v", s.remoteState)
	}
	if s.remotePreview != nil {
		t.Fatal("credentials retained after download")
	}
	if w := call("POST", "remote-fetch", request); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	previewID := s.remoteState.PreviewID
	if w := call("DELETE", "remote-fetch", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if s.remotePreview != nil || s.remoteState.PreviewID != "" || len(s.remoteState.Files) != 0 {
		t.Fatal("canceled preview retained credentials or stale metadata")
	}
	if w := call("POST", "remote-fetch", remoteRequest{Action: "download", PreviewID: previewID, SelectedPaths: []string{"app.log"}}); w.Code != 400 {
		t.Fatal("canceled preview was reused", w.Code)
	}
}

func TestRemoteRejectsTrailingJSON(t *testing.T) {
	s := New(t.TempDir())
	defer s.Close()
	r := httptest.NewRequest("POST", "/api/remote-fetch", strings.NewReader(`{} {}`))
	w := httptest.NewRecorder()
	s.remoteFetch(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "expected one JSON object") {
		t.Fatal(w.Code, w.Body.String())
	}
}

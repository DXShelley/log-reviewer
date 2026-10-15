package server

import (
	"encoding/json"
	"log-reviewer/internal/remote"
	"log-reviewer/internal/testsupport/sftpfixture"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRemoteAttachReadOnly(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "app.log")
	text := "2026-10-15 12:30:00 ERROR app : direct\n"
	if err := os.WriteFile(source, []byte(text), 0400); err != nil {
		t.Fatal(err)
	}
	fixture, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	host, port, _ := net.SplitHostPort(fixture.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	s := New(t.TempDir())
	defer s.Close()
	handler := s.Handler("127.0.0.1:8765")
	call := func(route string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/"+route, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("remote-fetch", remoteRequest{Action: "preview", Options: remote.Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: fixture.Fingerprint, Path: "app.log"}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.remoteState.Error != "" {
		t.Fatal(s.remoteState.Error)
	}
	if w := call("remote-fetch", remoteRequest{Action: "attach", PreviewID: s.remoteState.PreviewID, SelectedPaths: []string{"app.log"}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.remoteState.Error != "" || len(s.remoteState.Downloads) != 1 {
		t.Fatal(s.remoteState)
	}
	item := s.remoteState.Downloads[0]
	if w := call("analyze", map[string]any{"uploads": []string{item.ID}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.err != "" || s.Report.Events != 1 {
		t.Fatal(s.err)
	}
	// Re-analysis after page reload can submit the report's source reference.
	if w := call("analyze", map[string]any{"paths": []string{s.Report.Sources[0].Path}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.err != "" {
		t.Fatal(s.err)
	}
	if entries, _ := os.ReadDir(filepath.Join(s.dir, "remote")); len(entries) > 0 {
		t.Fatal("direct attach created source copies")
	}
	if w := call("upload-name?id="+item.ID+"&name=alias.log", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	got, _ := os.ReadFile(source)
	if string(got) != text {
		t.Fatal("source modified")
	}
}

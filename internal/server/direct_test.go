package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log-reviewer/internal/remote"
	"log-reviewer/internal/testsupport/sftpfixture"
	"net"
	"net/http/httptest"
	"net/url"
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
	s.remoteDir = filepath.Join(t.TempDir(), "remote")
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
	wantDigest, err := fileDigest(source)
	if err != nil || item.Digest != wantDigest || item.Digest == "" {
		t.Fatalf("direct import must use the same content digest as local files: %v, %+v", err, item)
	}
	if w := call("remote-fetch", remoteRequest{Action: "preview", Options: remote.Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: fixture.Fingerprint, Path: "app.log"}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if w := call("remote-fetch", remoteRequest{Action: "attach", PreviewID: s.remoteState.PreviewID, SelectedPaths: []string{"app.log"}}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.jobs.Wait()
	if s.remoteState.Error != "" || len(s.remoteState.Downloads) != 1 || s.remoteState.Downloads[0].ID != item.ID || len(s.uploads) != 1 {
		t.Fatalf("repeated attach must reuse its source reference: %+v", s.remoteState)
	}
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
	for _, mode := range []string{"merged", "files"} {
		query := url.Values{"from": {"2026-10-15T12:30:00"}, "to": {"2026-10-15T12:30:00"}, "mode": {mode}}
		r := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/time-export?"+query.Encode(), nil)
		r.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s export: %d %s", mode, w.Code, w.Body.String())
		}
		data := w.Body.Bytes()
		if mode == "files" {
			archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil || len(archive.File) != 1 {
				t.Fatalf("ZIP export: %v", err)
			}
			if !strings.Contains(archive.File[0].Name, "app.log") {
				t.Fatal("ZIP filename lost remote source name")
			}
			entry, err := archive.File[0].Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err = io.ReadAll(entry)
			entry.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		if string(data) != text {
			t.Fatalf("%s export changed source text: %q", mode, data)
		}
	}
	if entries, _ := os.ReadDir(s.remoteDir); len(entries) > 0 {
		t.Fatal("direct attach created source copies")
	}
	if w := call("upload-name?id="+item.ID+"&name=alias.log", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	got, _ := os.ReadFile(source)
	if string(got) != text {
		t.Fatal("source modified")
	}
	s.Close()
	if s.readers != nil || s.Report != nil || s.sourceContext.Err() == nil {
		t.Fatal("server shutdown retained source references or live context")
	}
}

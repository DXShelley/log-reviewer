package server

import (
	"archive/zip"
	"bytes"
	"io"
	"log-reviewer/internal/logs"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimeExportAPI(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	defer s.Close()
	paths := []string{}
	for _, name := range []string{"a.log", "b.log"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("2026-10-14 12:00:00 INFO app : "+name+"\nstack\n2026-10-14 13:00:00 ERROR app : excluded\n"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	var err error
	s.Report, err = logs.Analyze(paths, dir, logs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler("127.0.0.1:8765")
	call := func(mode string) *httptest.ResponseRecorder {
		q := url.Values{"from": {"2026-10-14T12:00:00"}, "to": {"2026-10-14T12:00:00"}, "mode": {mode}}
		req := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/time-export?"+q.Encode(), nil)
		req.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	merged := call("merged")
	if merged.Code != 200 || strings.Contains(merged.Body.String(), "excluded") || !strings.Contains(merged.Body.String(), "INFO") || strings.Index(merged.Body.String(), "a.log") > strings.Index(merged.Body.String(), "b.log") {
		t.Fatalf("merged: %d %s", merged.Code, merged.Body.String())
	}
	split := call("files")
	if split.Code != 200 {
		t.Fatal(split.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(split.Body.Bytes()), int64(split.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(z.File) != 2 {
		t.Fatal("wrong ZIP count")
	}
	for _, file := range z.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || !bytes.Contains(b, []byte("stack")) || bytes.Contains(b, []byte("excluded")) {
			t.Fatal("wrong ZIP content")
		}
	}
	if call("bad").Code != 400 {
		t.Fatal("invalid mode accepted")
	}
}

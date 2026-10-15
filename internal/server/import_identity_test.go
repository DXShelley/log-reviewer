package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log-reviewer/internal/logs"
)

func TestPathImportIdentityIsReadOnly(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i, text := range []string{"same", "same", "different"} {
		dir := filepath.Join(root, string(rune('a'+i)))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, "app.log")
		if err := os.WriteFile(p, []byte(text), 0400); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	s := New(t.TempDir())
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"paths": paths, "identify": true})
	r := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/paths", strings.NewReader(string(body)))
	r.Header.Set("Authorization", "Bearer "+s.Token())
	w := httptest.NewRecorder()
	s.Handler("127.0.0.1:8765").ServeHTTP(w, r)
	var result struct {
		Imports []pathImport `json:"imports"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Imports) != 3 {
		t.Fatal(w.Code, w.Body.String())
	}
	a, b, c := result.Imports[0], result.Imports[1], result.Imports[2]
	if a.Digest == "" || a.Digest != b.Digest || a.Digest == c.Digest || a.Path == b.Path || a.Name != "app.log" {
		t.Fatal(result)
	}
	if len(s.uploads) != 0 {
		t.Fatal("path identity created upload copies")
	}
	for i, p := range paths {
		got, err := os.ReadFile(p)
		want := "same"
		if i == 2 {
			want = "different"
		}
		if err != nil || string(got) != want {
			t.Fatal("source changed", p, err)
		}
	}
}

func TestSourceDigestCancellation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "source.log")
	if err := os.WriteFile(p, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := sourceDigest(ctx, func() (logs.ReadFile, error) { return os.Open(p) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled hash: %v", err)
	}
}

type changingDigestFile struct {
	*os.File
	changed bool
}

func (f *changingDigestFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	if !f.changed {
		f.changed = true
		if e := os.WriteFile(f.Name(), []byte("changed content"), 0600); e != nil {
			return 0, e
		}
	}
	return n, err
}

func TestSourceDigestRejectsChangedFiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "source.log")
	if err := os.WriteFile(p, []byte("initial"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := sourceDigest(context.Background(), func() (logs.ReadFile, error) { f, e := os.Open(p); return &changingDigestFile{File: f}, e })
	if err == nil || !strings.Contains(err.Error(), "变化") {
		t.Fatalf("changed file accepted: %v", err)
	}
}

package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log-reviewer/internal/logs"
)

func TestUploadKeepsLocalCopyWhenRemoteDigestMatches(t *testing.T) {
	s := New(t.TempDir())
	defer s.Close()
	s.uploads["remote-test"] = "sftp:remote-test"
	s.uploadDigests = map[string]string{"remote-test": "same"}
	s.readers = map[string]logs.SourceOpener{"sftp:remote-test": func() (logs.ReadFile, error) {
		return nil, errors.New("remote offline")
	}}
	p := filepath.Join(t.TempDir(), "upload.log")
	if err := os.WriteFile(p, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	id := s.registerImport("upload", p, "same")
	if id != "upload" || s.uploads[id] != p {
		t.Fatalf("upload replaced with remote reference: %s", id)
	}
	if data, err := os.ReadFile(p); err != nil || string(data) != "retained" {
		t.Fatalf("upload copy lost: %q %v", data, err)
	}
	second := filepath.Join(t.TempDir(), "second.log")
	if err := os.WriteFile(second, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := s.registerImport("second", second, "same"); got != id {
		t.Fatalf("local cache dedup failed: %s", got)
	}
}

type observedDigestFile struct {
	*os.File
	readErr error
	statErr error
	cancel  context.CancelFunc
	closed  bool
}

func (f *observedDigestFile) Read(p []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	n, err := f.File.Read(p)
	if f.cancel != nil {
		f.cancel()
	}
	return n, err
}

func (f *observedDigestFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.File.Stat()
}

func (f *observedDigestFile) Close() error {
	f.closed = true
	return f.File.Close()
}

func TestSourceDigestBoundaries(t *testing.T) {
	sentinel := errors.New("source unavailable")
	for _, name := range []string{"empty", "read failure", "stat failure", "cancel during read"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "source.log")
			content := ""
			if name != "empty" {
				content = strings.Repeat("x", 65536)
			}
			if err := os.WriteFile(p, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			observed := &observedDigestFile{File: f}
			var wantErr error
			switch name {
			case "read failure":
				observed.readErr, wantErr = sentinel, sentinel
			case "stat failure":
				observed.statErr, wantErr = sentinel, sentinel
			case "cancel during read":
				observed.cancel, wantErr = cancel, context.Canceled
			}
			digest, err := sourceDigest(ctx, func() (logs.ReadFile, error) { return observed, nil })
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			if wantErr == nil && digest != fmt.Sprintf("%x", sha256.Sum256(nil)) {
				t.Fatalf("incorrect empty file digest: %s", digest)
			}
			if wantErr != nil && digest != "" {
				t.Fatalf("failed read returned usable digest: %s", digest)
			}
			if !observed.closed {
				t.Fatal("source handle was not closed")
			}
		})
	}
	t.Run("open failure", func(t *testing.T) {
		digest, err := sourceDigest(context.Background(), func() (logs.ReadFile, error) { return nil, sentinel })
		if !errors.Is(err, sentinel) || digest != "" {
			t.Fatalf("digest = %q, error = %v", digest, err)
		}
	})
}

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

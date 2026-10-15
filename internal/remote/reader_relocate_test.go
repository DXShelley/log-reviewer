package remote

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachedSourceAfterRelocation(t *testing.T) {
	oldRoot, newRoot := filepath.Join(t.TempDir(), "old"), filepath.Join(t.TempDir(), "new")
	oldCache := filepath.Join(oldRoot, "task", "source")
	if err := os.MkdirAll(oldCache, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(oldCache, "service.text")
	text := "cached snapshot remains readable"
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	source := &DirectSource{CacheDir: oldCache, cached: file, File: PreviewFile{Path: "/var/log/service.text", Modified: time.Now().Format(time.RFC3339)}}
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	source.RelocateCache(oldRoot, newRoot)
	if source.CacheDir != filepath.Join(newRoot, "task", "source") {
		t.Fatal(source.CacheDir)
	}
	f, err := source.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil || string(body) != text {
		t.Fatal(string(body), err)
	}
}

package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterImportContentDeduplication(t *testing.T) {
	s := New(t.TempDir())
	add := func(id, name, text string) string {
		t.Helper()
		dir := filepath.Join(s.dir, id)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		digest, err := fileDigest(p)
		if err != nil {
			t.Fatal(err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.registerImport(id, p, digest)
	}
	if add("local", "a.log", "same") != "local" {
		t.Fatal("first import")
	}
	if add("remote", "b.log", "same") != "local" {
		t.Fatal("identical content was imported twice")
	}
	if _, err := os.Stat(filepath.Join(s.dir, "remote", "b.log")); !os.IsNotExist(err) {
		t.Fatal("duplicate cache retained")
	}
	if add("changed", "a.log", "different") != "changed" {
		t.Fatal("different content discarded")
	}
	if len(s.uploads) != 2 {
		t.Fatalf("got %d uploads", len(s.uploads))
	}
}

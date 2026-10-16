package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitFilesRespectFormats(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	defer s.Close()
	allowed, blocked := filepath.Join(dir, "app.log"), filepath.Join(dir, "secret.csv")
	for _, p := range []string{allowed, blocked} {
		if err := os.WriteFile(p, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := s.expandImportPaths([]string{allowed, blocked})
	if err != nil || len(paths) != 1 || paths[0] != allowed {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	s.access.config.FilePatterns = []string{"*.csv"}
	paths, err = s.expandImportPaths([]string{allowed, blocked})
	if err != nil || len(paths) != 1 || paths[0] != blocked {
		t.Fatalf("custom paths=%v err=%v", paths, err)
	}
}

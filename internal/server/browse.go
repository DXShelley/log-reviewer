package server

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

type browseEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
}

func (s *Server) browseFiles(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		fail(w, 400, err)
		return
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		fail(w, 400, err)
		return
	}
	roots := []string{string(filepath.Separator)}
	if runtime.GOOS == "windows" {
		roots = nil
		for drive := 'A'; drive <= 'Z'; drive++ {
			root := string(drive) + ":\\"
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				roots = append(roots, root)
			}
		}
	}
	entries := make([]browseEntry, 0, len(items))
	patterns := s.filePatterns()
	for _, item := range items {
		if r.Context().Err() != nil {
			return
		}
		path := filepath.Join(dir, item.Name())
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() && (!info.Mode().IsRegular() || !matchesFile(item.Name(), patterns)) {
			continue
		}
		entries = append(entries, browseEntry{Name: item.Name(), Path: path, Directory: info.IsDir(), Size: info.Size()})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Directory != entries[j].Directory {
			return entries[i].Directory
		}
		return entries[i].Name < entries[j].Name
	})
	output(w, map[string]any{"path": dir, "parent": filepath.Dir(dir), "roots": roots, "entries": entries})
}

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Caller holds s.mu so simultaneous imports share a single cached file.
func (s *Server) registerImport(id, path, digest string) string {
	if s.uploadDigests == nil {
		s.uploadDigests = make(map[string]string)
	}
	for existingID, existingDigest := range s.uploadDigests {
		if existingDigest == digest {
			if s.uploads[existingID] != path {
				os.Remove(path)
			}
			return existingID
		}
	}
	s.uploads[id] = path
	s.uploadDigests[id] = digest
	return id
}

func (s *Server) renameUpload(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	id, name := r.URL.Query().Get("id"), r.URL.Query().Get("name")
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		fail(w, 400, errors.New("invalid filename"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.uploads[id]
	if !ok {
		fail(w, 400, errors.New("unknown upload"))
		return
	}
	if s.readers[old] != nil {
		// A remote reference has a display alias only. Never rename a source.
		output(w, map[string]string{"id": id, "name": name})
		return
	}
	next := filepath.Join(filepath.Dir(old), name)
	if next != old {
		if _, err := os.Stat(next); !os.IsNotExist(err) {
			fail(w, 409, errors.New("filename already exists"))
			return
		}
		if err := os.Rename(old, next); err != nil {
			fail(w, 500, err)
			return
		}
		s.uploads[id] = next
		if source, exists := s.remoteSources[old]; exists {
			s.remoteSources[next] = source
			delete(s.remoteSources, old)
		}
	}
	output(w, map[string]string{"id": id, "name": name})
}

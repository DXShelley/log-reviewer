package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"log-reviewer/internal/logs"
)

func fileDigest(path string) (string, error) {
	return sourceDigest(context.Background(), func() (logs.ReadFile, error) { return os.Open(path) })
}

type digestReader struct {
	ctx context.Context
	io.Reader
}

func (r digestReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

// Hash a bounded, read-only stream and reject a source that changes while read.
func sourceDigest(ctx context.Context, open logs.SourceOpener) (string, error) {
	f, err := open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	h := sha256.New()
	n, err := io.Copy(h, digestReader{ctx, io.LimitReader(f, before.Size()+1)})
	if err != nil {
		return "", err
	}
	after, err := f.Stat()
	if err != nil {
		return "", err
	}
	if n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return "", errors.New("文件在内容校验期间发生变化，请重新导入")
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
			existingPath := s.uploads[existingID]
			// A network reference must never replace a completed local upload.
			if strings.HasPrefix(existingPath, "sftp:") || s.readers[existingPath] != nil {
				continue
			}
			if info, err := os.Stat(existingPath); err != nil || !info.Mode().IsRegular() {
				continue
			}
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

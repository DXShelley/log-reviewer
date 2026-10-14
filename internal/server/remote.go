package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/remote"
)

type remoteDownload struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RemotePath string `json:"remote_path"`
}
type remoteState struct {
	remote.Progress
	ID        string           `json:"id"`
	Busy      bool             `json:"busy"`
	Error     string           `json:"error,omitempty"`
	Downloads []remoteDownload `json:"downloads"`
	Warnings  []string         `json:"warnings"`
}

func (s *Server) remoteFetch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		state := s.remoteState
		s.mu.RUnlock()
		output(w, state)
	case http.MethodDelete:
		s.mu.Lock()
		if s.remoteCancel != nil {
			s.remoteCancel()
		}
		s.mu.Unlock()
		output(w, map[string]bool{"accepted": true})
	case http.MethodPost:
		var options remote.Options
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&options); err != nil {
			fail(w, 400, err)
			return
		}
		if err := options.Validate(); err != nil {
			fail(w, 400, err)
			return
		}
		s.mu.Lock()
		if s.closing || s.busy || s.remoteState.Busy {
			s.mu.Unlock()
			fail(w, 409, errors.New("analysis or remote download is already running"))
			return
		}
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			s.mu.Unlock()
			fail(w, 500, err)
			return
		}
		id := hex.EncodeToString(bytes)
		ctx, cancel := context.WithCancel(context.Background())
		s.remoteCancel = cancel
		s.remoteState = remoteState{ID: id, Busy: true, Progress: remote.Progress{Stage: "connecting"}, Downloads: []remoteDownload{}, Warnings: []string{}}
		s.jobs.Add(1)
		s.mu.Unlock()
		go s.downloadRemote(ctx, cancel, id, options)
		output(w, map[string]string{"id": id})
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		fail(w, 405, errors.New("method not allowed"))
	}
}

func (s *Server) downloadRemote(ctx context.Context, cancel context.CancelFunc, id string, options remote.Options) {
	defer s.jobs.Done()
	defer cancel()
	destination := filepath.Join(s.dir, "remote", id)
	manifest, err := remote.Download(ctx, options, destination, func(p remote.Progress) { s.mu.Lock(); s.remoteState.Progress = p; s.mu.Unlock() })
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remoteState.Busy = false
	s.remoteCancel = nil
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		s.remoteState.Error = err.Error()
		s.remoteState.Stage = "failed"
		os.RemoveAll(destination)
		return
	}
	for i, file := range manifest.Files {
		uploadID := id + "-" + strconv.Itoa(i)
		s.uploads[uploadID] = file.LocalPath
		s.remoteSources[file.LocalPath] = logs.RemoteSource{Protocol: "sftp", Host: manifest.Host, Port: manifest.Port, Path: file.RemotePath, CapturedAt: manifest.CapturedAt, Changed: file.Changed, Modified: file.Modified}
		s.remoteState.Downloads = append(s.remoteState.Downloads, remoteDownload{ID: uploadID, Name: path.Base(file.RemotePath), RemotePath: file.RemotePath})
	}
	s.remoteState.Warnings = manifest.Warnings
	s.remoteState.Stage = "complete"
}

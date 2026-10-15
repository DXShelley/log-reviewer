package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/remote"
)

type remoteDownload struct {
	SourcePath string `json:"source_path,omitempty"`
	Digest     string `json:"digest"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	RemotePath string `json:"remote_path"`
}
type remoteState struct {
	remote.Progress
	ID        string               `json:"id"`
	Busy      bool                 `json:"busy"`
	Error     string               `json:"error,omitempty"`
	Downloads []remoteDownload     `json:"downloads"`
	Warnings  []string             `json:"warnings"`
	HostKey   *remote.HostKeyError `json:"host_key,omitempty"`
	PreviewID string               `json:"preview_id,omitempty"`
	Files     []remote.PreviewFile `json:"preview_files,omitempty"`
}

type remotePreview struct {
	id      string
	options remote.Options
	files   []remote.PreviewFile
	expires time.Time
	timer   *time.Timer
}

type remoteRequest struct {
	remote.Options
	Action        string   `json:"action,omitempty"`
	PreviewID     string   `json:"preview_id,omitempty"`
	SelectedPaths []string `json:"selected_paths,omitempty"`
}

func (s *Server) clearRemotePreview() {
	if s.remotePreview != nil {
		s.remotePreview.timer.Stop()
		s.remotePreview = nil
	}
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
		s.clearRemotePreview()
		if !s.remoteState.Busy {
			s.remoteState.PreviewID = ""
			s.remoteState.Files = nil
			s.remoteState.Stage = "canceled"
		}
		if s.remoteCancel != nil {
			s.remoteCancel()
		}
		s.mu.Unlock()
		output(w, map[string]bool{"accepted": true})
	case http.MethodPost:
		var request remoteRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			fail(w, 400, err)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			fail(w, 400, errors.New("expected one JSON object"))
			return
		}
		if request.Action != "" && request.Action != "preview" && request.Action != "download" && request.Action != "attach" {
			fail(w, 400, errors.New("invalid remote action"))
			return
		}
		options := request.Options
		if options.Pattern == "" && request.Action != "download" && request.Action != "attach" {
			options.Pattern = strings.Join(s.filePatterns(), "\n")
		}
		if request.Action != "download" && request.Action != "attach" {
			if err := options.Validate(); err != nil {
				fail(w, 400, err)
				return
			}
		}
		s.mu.Lock()
		if s.closing || s.busy || s.remoteState.Busy {
			s.mu.Unlock()
			fail(w, 409, errors.New("analysis or remote download is already running"))
			return
		}
		var selection []remote.PreviewFile
		if request.Action == "download" || request.Action == "attach" {
			preview := s.remotePreview
			if preview == nil || preview.id != request.PreviewID || time.Now().After(preview.expires) {
				s.mu.Unlock()
				fail(w, 400, errors.New("预览已过期，请重新连接并预览"))
				return
			}
			options = preview.options
			if len(request.SelectedPaths) == 0 || len(request.SelectedPaths) > options.MaxFiles {
				s.mu.Unlock()
				fail(w, 400, errors.New("请选择文件，且不能超过最多文件数"))
				return
			}
			available := map[string]remote.PreviewFile{}
			for _, file := range preview.files {
				available[file.Path] = file
			}
			var bytes int64
			for _, name := range request.SelectedPaths {
				file, ok := available[name]
				if !ok {
					s.mu.Unlock()
					fail(w, 400, errors.New("所选文件不属于当前预览或存在重复项"))
					return
				}
				delete(available, name)
				bytes += file.Size
				selection = append(selection, file)
			}
			if bytes > options.MaxBytes {
				s.mu.Unlock()
				fail(w, 400, errors.New("所选文件总大小超出限制"))
				return
			}
		}
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			s.mu.Unlock()
			fail(w, 500, err)
			return
		}
		id := hex.EncodeToString(bytes)
		s.clearRemotePreview()
		ctx, cancel := context.WithCancel(context.Background())
		s.remoteCancel = cancel
		s.remoteState = remoteState{ID: id, Busy: true, Progress: remote.Progress{Stage: "connecting"}, Downloads: []remoteDownload{}, Warnings: []string{}}
		s.jobs.Add(1)
		s.mu.Unlock()
		go s.downloadRemote(ctx, cancel, id, options, request.Action, selection)
		output(w, map[string]string{"id": id})
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		fail(w, 405, errors.New("method not allowed"))
	}
}

func (s *Server) downloadRemote(ctx context.Context, cancel context.CancelFunc, id string, options remote.Options, action string, selection []remote.PreviewFile) {
	defer s.jobs.Done()
	defer cancel()
	destination := filepath.Join(s.remoteDir, id)
	update := func(p remote.Progress) { s.mu.Lock(); s.remoteState.Progress = p; s.mu.Unlock() }
	var manifest *remote.Manifest
	var files []remote.PreviewFile
	var sources []*remote.DirectSource
	var digests []string
	var err error
	patterns := s.filePatterns()
	filter := func(input []remote.PreviewFile) []remote.PreviewFile {
		result := make([]remote.PreviewFile, 0, len(input))
		for _, file := range input {
			if matchesFile(path.Base(file.Path), patterns) {
				result = append(result, file)
			}
		}
		return result
	}
	for _, file := range selection {
		if !matchesFile(path.Base(file.Path), patterns) {
			err = errors.New("远程文件不符合设置中的识别格式")
			break
		}
	}
	if err != nil {
		s.mu.Lock()
		s.remoteState.Busy = false
		s.remoteCancel = nil
		s.remoteState.Error = err.Error()
		s.remoteState.Stage = "failed"
		s.mu.Unlock()
		return
	}
	if action == "preview" {
		var fingerprint string
		files, fingerprint, err = remote.Preview(ctx, options, update)
		files = filter(files)
		options.Fingerprint = fingerprint
	} else if action == "attach" {
		// Revalidate metadata before registering references; never download here.
		files, _, err = remote.Preview(ctx, options, update)
		if err == nil {
			available := map[string]remote.PreviewFile{}
			for _, file := range files {
				available[file.Path] = file
			}
			for _, file := range selection {
				if available[file.Path] != file {
					err = errors.New("远程文件已变化，请重新预览")
					break
				}
			}
		}
		if err == nil {
			for i, file := range selection {
				source := &remote.DirectSource{Context: ctx, Options: options, File: file, CacheDir: filepath.Join(destination, strconv.Itoa(i))}
				update(remote.Progress{Stage: "hashing", Current: file.Path, Files: i, TotalFiles: len(selection)})
				digest, hashErr := sourceDigest(ctx, source.Open)
				if hashErr != nil {
					err = fmt.Errorf("内容校验失败 %q：%w", file.Path, hashErr)
					break
				}
				source.Context = s.sourceContext
				sources = append(sources, source)
				digests = append(digests, digest)
			}
		}
	} else if action == "download" {
		manifest, err = remote.DownloadSelected(ctx, options, selection, destination, update)
	} else {
		files, _, err = remote.Preview(ctx, options, update)
		if err == nil {
			manifest, err = remote.DownloadSelected(ctx, options, filter(files), destination, update)
		}
	}
	var saveError error
	if err == nil {
		saveError = s.rememberRemote(options)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remoteState.Busy = false
	s.remoteCancel = nil
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		var hostKey *remote.HostKeyError
		if errors.As(err, &hostKey) {
			s.remoteState.HostKey = hostKey
		}
		s.remoteState.Error = err.Error()
		s.remoteState.Stage = "failed"
		os.RemoveAll(destination)
		return
	}
	if saveError != nil {
		s.remoteState.Warnings = append(s.remoteState.Warnings, "主机记录保存失败："+saveError.Error())
	}
	if action == "preview" {
		preview := &remotePreview{id: id, options: options, files: files, expires: time.Now().Add(15 * time.Minute)}
		preview.timer = time.AfterFunc(15*time.Minute, func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.remotePreview == preview {
				s.remotePreview = nil
			}
		})
		s.remotePreview = preview
		s.remoteState.PreviewID, s.remoteState.Files, s.remoteState.Stage = id, files, "preview"
		return
	}
	if action == "attach" {
		if s.readers == nil {
			s.readers = map[string]logs.SourceOpener{}
		}
		if s.uploadDigests == nil {
			s.uploadDigests = map[string]string{}
		}
		for i, file := range selection {
			identity, _ := json.Marshal([]any{options.Host, options.Port, options.User, options.Fingerprint, file, digests[i]})
			sum := sha256.Sum256(identity)
			uploadID := "remote-" + hex.EncodeToString(sum[:])
			key := "sftp:" + uploadID
			if s.readers[key] == nil {
				source := sources[i]
				s.readers[key] = source.Open
				s.directSources[key] = source
			} else {
				s.directSources[key].RefreshOptions(options)
			}
			s.uploads[uploadID] = key
			s.uploadDigests[uploadID] = digests[i]
			s.remoteSources[key] = logs.RemoteSource{Protocol: "sftp", Host: options.Host, Port: options.Port, Path: file.Path, Modified: file.Modified, CapturedAt: time.Now().Format(time.RFC3339)}
			s.remoteState.Downloads = append(s.remoteState.Downloads, remoteDownload{ID: uploadID, Name: path.Base(file.Path), RemotePath: file.Path, Digest: digests[i], SourcePath: key})
		}
		s.remoteState.Stage = "complete"
		return
	}
	for i, file := range manifest.Files {
		digest, hashErr := fileDigest(file.LocalPath)
		if hashErr != nil {
			s.remoteState.Warnings = append(s.remoteState.Warnings, hashErr.Error())
			continue
		}
		uploadID := id + "-" + strconv.Itoa(i)
		uploadID = s.registerImport(uploadID, file.LocalPath, digest)
		if s.uploads[uploadID] != file.LocalPath {
			s.remoteState.Downloads = append(s.remoteState.Downloads, remoteDownload{ID: uploadID, Name: path.Base(file.RemotePath), RemotePath: file.RemotePath, Digest: digest})
			continue
		}
		s.remoteSources[file.LocalPath] = logs.RemoteSource{Protocol: "sftp", Host: manifest.Host, Port: manifest.Port, Path: file.RemotePath, CapturedAt: manifest.CapturedAt, Changed: file.Changed, Modified: file.Modified}
		s.remoteState.Downloads = append(s.remoteState.Downloads, remoteDownload{ID: uploadID, Name: path.Base(file.RemotePath), RemotePath: file.RemotePath, Digest: digest})
	}
	s.remoteState.Warnings = append(s.remoteState.Warnings, manifest.Warnings...)
	s.remoteState.Stage = "complete"
}

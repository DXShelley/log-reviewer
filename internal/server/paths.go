package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"log-reviewer/internal/logs"
)

type pathImport struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Digest     string `json:"digest"`
	RemotePath string `json:"remote_path,omitempty"`
}

func (s *Server) localPaths(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		fail(w, 405, errors.New("method not allowed"))
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		fail(w, 500, err)
		return
	}
	host, _ := os.Hostname()
	result := map[string]any{"hostname": host, "os": runtime.GOOS, "working_directory": cwd}
	result["native_picker"] = localPickerAvailable(r)
	result["local_access"] = localPeer(r)
	if r.Method == http.MethodPost {
		var request struct {
			Paths    []string `json:"paths"`
			Identify bool     `json:"identify,omitempty"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			fail(w, 400, err)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			fail(w, 400, errors.New("expected one JSON object"))
			return
		}
		if len(request.Paths) == 0 || len(request.Paths) > 1000 {
			fail(w, 400, errors.New("请填写 1–1000 条文件、目录或通配符路径，每行一条"))
			return
		}
		var local, paths []string
		s.mu.RLock()
		for _, p := range request.Paths {
			if request.Identify && s.readers[p] != nil {
				paths = append(paths, p)
			} else {
				local = append(local, p)
			}
		}
		s.mu.RUnlock()
		if len(local) > 0 {
			var expanded []string
			expanded, err = s.expandImportPaths(local)
			paths = append(paths, expanded...)
		}
		if err != nil {
			fail(w, 400, fmt.Errorf("路径解析失败（主机 %s；工作目录 %s）：%w；目录按文件设置识别当前层文件，不递归子目录", host, cwd, err))
			return
		}
		result["paths"] = paths
		if request.Identify {
			items := make([]pathImport, 0, len(paths))
			for _, p := range paths {
				s.mu.RLock()
				open, origin := s.readers[p], s.remoteSources[p]
				s.mu.RUnlock()
				if open == nil {
					open = func() (logs.ReadFile, error) { return os.Open(p) }
				}
				digest, hashErr := sourceDigest(r.Context(), open)
				if hashErr != nil {
					fail(w, 400, fmt.Errorf("内容校验失败 %q：%w", p, hashErr))
					return
				}
				name := filepath.Base(p)
				if origin.Path != "" {
					name = filepath.Base(origin.Path)
				}
				items = append(items, pathImport{Path: p, Name: name, Digest: digest, RemotePath: origin.Path})
			}
			result["imports"] = items
		}
	}
	output(w, result)
}

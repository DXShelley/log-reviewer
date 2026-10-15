package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

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
			Paths []string `json:"paths"`
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
		paths, err := s.expandImportPaths(request.Paths)
		if err != nil {
			fail(w, 400, fmt.Errorf("路径解析失败（主机 %s；工作目录 %s）：%w；目录按文件设置识别当前层文件，不递归子目录", host, cwd, err))
			return
		}
		result["paths"] = paths
	}
	output(w, result)
}

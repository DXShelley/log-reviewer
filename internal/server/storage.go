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
	"strings"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/remote"
)

var defaultFilePatterns = []string{"*.log", "*.log.*", "*.txt", "*.text", "*.out", "*.err", "*.trace"}

func defaultRemoteDirectory() string {
	home, _ := os.UserHomeDir()
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" || !filepath.IsAbs(base) {
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, "LogReviewer", "remote")
}

func (s *Server) sessionRemoteDirectory(base string) string {
	return filepath.Join(base, filepath.Base(s.dir)+"-"+s.token[:12])
}

func (s *Server) filePatterns() []string {
	s.access.mu.RLock()
	defer s.access.mu.RUnlock()
	if len(s.access.config.FilePatterns) == 0 {
		return append([]string(nil), defaultFilePatterns...)
	}
	return append([]string(nil), s.access.config.FilePatterns...)
}

func matchesFile(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(name)); ok {
			return true
		}
	}
	return false
}

// Explicit files remain selectable; only directory discovery uses format rules.
func (s *Server) expandImportPaths(paths []string) ([]string, error) {
	return logs.ExpandPaths(paths, s.filePatterns()...)
}

type importSettings struct {
	RemoteDirectory string   `json:"remote_directory"`
	FilePatterns    []string `json:"file_patterns"`
}

func (s *Server) importSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.access.mu.RLock()
		base := s.access.config.RemoteDirectory
		s.access.mu.RUnlock()
		if base == "" {
			base = defaultRemoteDirectory()
		}
		output(w, map[string]any{"remote_directory": base, "session_directory": s.remoteDir, "default_remote_directory": defaultRemoteDirectory(), "file_patterns": s.filePatterns(), "default_file_patterns": defaultFilePatterns, "os": runtime.GOOS})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		fail(w, 405, errors.New("method not allowed"))
		return
	}
	var request importSettings
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		fail(w, 400, err)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		fail(w, 400, errors.New("expected one JSON object"))
		return
	}
	request.RemoteDirectory = strings.TrimSpace(request.RemoteDirectory)
	if !filepath.IsAbs(request.RemoteDirectory) {
		fail(w, 400, errors.New("保存目录必须是工具所在机器上的绝对路径"))
		return
	}
	request.RemoteDirectory = filepath.Clean(request.RemoteDirectory)
	if len(request.FilePatterns) == 0 || len(request.FilePatterns) > 64 {
		fail(w, 400, errors.New("请填写 1–64 条文件名通配符"))
		return
	}
	for i, pattern := range request.FilePatterns {
		pattern = strings.TrimSpace(pattern)
		if _, err := filepath.Match(pattern, "test.log"); err != nil || pattern == "" || strings.ContainsAny(pattern, "/\\\x00\r\n") {
			fail(w, 400, errors.New("文件识别格式必须是有效的文件名通配符"))
			return
		}
		request.FilePatterns[i] = pattern
	}
	// Drain active detail/export requests before changing any physical paths.
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy || s.remoteState.Busy || s.closing {
		fail(w, 409, errors.New("请等待当前分析或远程获取完成后保存文件设置"))
		return
	}
	oldRoot, newRoot := s.remoteDir, s.sessionRemoteDirectory(request.RemoteDirectory)
	if insidePath(oldRoot, newRoot) && oldRoot != newRoot || insidePath(newRoot, oldRoot) && oldRoot != newRoot {
		fail(w, 400, errors.New("新旧保存目录不能互相包含"))
		return
	}
	if oldRoot != newRoot {
		if err := copyRemoteTree(oldRoot, newRoot); err != nil {
			fail(w, 500, fmt.Errorf("迁移失败，原文件和配置已保留：%w", err))
			return
		}
	}
	s.access.mu.Lock()
	cfg := s.access.config
	cfg.RemoteDirectory, cfg.FilePatterns = request.RemoteDirectory, request.FilePatterns
	if s.access.path != "" {
		if err := s.access.save(cfg); err != nil {
			s.access.mu.Unlock()
			if oldRoot != newRoot {
				os.RemoveAll(newRoot)
			}
			fail(w, 500, fmt.Errorf("配置保存失败，原文件已保留：%w", err))
			return
		}
	}
	s.access.config = cfg
	s.access.mu.Unlock()
	pathChanges := map[string]string{}
	if oldRoot != newRoot {
		for id, p := range s.uploads {
			next := movedPath(p, oldRoot, newRoot)
			s.uploads[id] = next
			if next != p {
				pathChanges[p] = next
			}
		}
		origins := make(map[string]logs.RemoteSource, len(s.remoteSources))
		for p, origin := range s.remoteSources {
			origins[movedPath(p, oldRoot, newRoot)] = origin
		}
		s.remoteSources = origins
		for _, source := range s.directSources {
			source.RelocateCache(oldRoot, newRoot)
		}
		if s.Report != nil {
			for i := range s.Report.Sources {
				p := s.Report.Sources[i].Path
				next := movedPath(p, oldRoot, newRoot)
				s.Report.Sources[i].Path = next
				if next != p {
					pathChanges[p] = next
				}
			}
			// The index stays in the session cache; only its source files move.
			logs.SaveReport(s.Report, filepath.Join(s.dir, "latest.json"))
		}
		s.remoteDir = newRoot
	}
	result := map[string]any{"remote_directory": request.RemoteDirectory, "session_directory": s.remoteDir, "file_patterns": request.FilePatterns, "path_changes": pathChanges}
	if oldRoot != newRoot {
		if err := os.RemoveAll(oldRoot); err != nil {
			result["warning"] = "迁移已完成，旧目录清理失败：" + err.Error()
		}
	}
	output(w, result)
}

func insidePath(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func movedPath(p, oldRoot, newRoot string) string {
	if !insidePath(oldRoot, p) {
		return p
	}
	rel, _ := filepath.Rel(oldRoot, p)
	return filepath.Join(newRoot, rel)
}

// Copy into a staging directory on the destination volume and preserve source
// timestamps so existing report offsets and source validation remain valid.
func copyRemoteTree(oldRoot, newRoot string) error {
	if err := os.MkdirAll(filepath.Dir(newRoot), 0700); err != nil {
		return err
	}
	sourceRoot, sourceErr := filepath.EvalSymlinks(oldRoot)
	targetParent, targetErr := filepath.EvalSymlinks(filepath.Dir(newRoot))
	if targetErr != nil {
		return targetErr
	}
	if sourceErr == nil {
		targetRoot := filepath.Join(targetParent, filepath.Base(newRoot))
		if insidePath(sourceRoot, targetRoot) || insidePath(targetRoot, sourceRoot) {
			return errors.New("新旧保存目录解析后不能相同或互相包含")
		}
	}
	if _, err := os.Lstat(newRoot); !os.IsNotExist(err) {
		return errors.New("目标会话目录已存在或不可访问")
	}
	stage, err := os.MkdirTemp(filepath.Dir(newRoot), ".log-reviewer-move-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	err = filepath.WalkDir(oldRoot, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p == oldRoot && os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		rel, _ := filepath.Rel(oldRoot, p)
		target := filepath.Join(stage, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("不支持迁移非普通文件：%s", p)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		src.Close()
		if copyErr == nil {
			copyErr = dst.Sync()
		}
		closeErr := dst.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if entry.Name() == "manifest.json" {
			data, err := os.ReadFile(target)
			if err != nil {
				return err
			}
			var manifest remote.Manifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return err
			}
			for i := range manifest.Files {
				manifest.Files[i].LocalPath = movedPath(manifest.Files[i].LocalPath, oldRoot, newRoot)
			}
			data, err = json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0600); err != nil {
				return err
			}
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	})
	if err != nil {
		return err
	}
	return os.Rename(stage, newRoot)
}

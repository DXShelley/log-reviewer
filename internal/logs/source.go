package logs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ReadFile deliberately exposes no operations that can change a source.
type ReadFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	Stat() (os.FileInfo, error)
}
type SourceOpener func() (ReadFile, error)

func openRead(path string, readers map[string]SourceOpener) (ReadFile, error) {
	if open := readers[path]; open != nil {
		return open()
	}
	if strings.HasPrefix(path, "sftp:") {
		return nil, fmt.Errorf("remote source is not connected; reconnect and analyze again")
	}
	return os.Open(path)
}

func expandSources(paths []string, readers map[string]SourceOpener) ([]string, error) {
	var local, result []string
	seen := map[string]bool{}
	for _, path := range paths {
		if readers[path] != nil {
			if !seen[path] {
				result = append(result, path)
				seen[path] = true
			}
			continue
		}
		local = append(local, path)
	}
	if len(local) > 0 {
		expanded, err := ExpandPaths(local)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no input files")
	}
	return result, nil
}

func normalizePath(input string) (string, error) {
	p := strings.TrimSpace(input)
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		var decoded string
		literal := p[1 : len(p)-1]
		if _, err := os.Stat(literal); err == nil {
			p = literal
		} else if json.Unmarshal([]byte(p), &decoded) == nil {
			p = decoded
		} else {
			p = p[1 : len(p)-1]
		}
	} else if len(p) >= 2 && p[0] == '\'' && p[len(p)-1] == '\'' {
		p = p[1 : len(p)-1]
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("path contains NUL")
	}
	if strings.HasPrefix(strings.ToLower(p), "file:") {
		p = strings.ReplaceAll(p, "\\", "/")
		u, err := url.Parse(p)
		if err != nil {
			return "", err
		}
		if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("file URI must not contain credentials, query or fragment; encode # as %%23 and ? as %%3F")
		}
		p = u.Path
		if p == "" {
			p, err = url.PathUnescape(u.Opaque)
			if err != nil {
				return "", err
			}
		}
		if runtime.GOOS == "windows" && len(u.Host) == 2 && u.Host[1] == ':' {
			p = u.Host + p
		} else if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			if runtime.GOOS != "windows" {
				return "", fmt.Errorf("remote file URI requires a mounted local path or remote fetch")
			}
			p = "//" + u.Host + "/" + strings.TrimPrefix(p, "/")
		} else if runtime.GOOS == "windows" && len(p) > 2 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		p = filepath.FromSlash(p)
	} else if strings.Contains(p, "://") {
		return "", fmt.Errorf("unsupported path protocol; use file:// or remote fetch")
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("path contains NUL")
	}
	return p, nil
}

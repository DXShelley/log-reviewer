package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log-reviewer/internal/logs"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func localPickerAvailable(r *http.Request) bool {
	return localPeer(r) && runtime.GOOS == "windows"
}

func localPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return false
	}
	if peer.IsLoopback() {
		return true
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err == nil && ip.Equal(peer) {
			return true
		}
	}
	return false
}

func (s *Server) pickPaths(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	if !localPickerAvailable(r) {
		fail(w, 403, errors.New("本机系统选择器仅支持 Windows 本机直连；请使用输入路径或浏览器上传"))
		return
	}
	if !s.pickerMu.TryLock() {
		fail(w, 409, errors.New("系统选择器已打开"))
		return
	}
	defer s.pickerMu.Unlock()
	kind := r.URL.Query().Get("kind")
	script := "Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.OpenFileDialog; $d.Multiselect=$true; $d.Title='选择日志文件（只读分析）'; if($d.ShowDialog() -eq 'OK'){ConvertTo-Json -Compress -InputObject @($d.FileNames)}else{'[]'}"
	if kind == "directory" {
		script = "Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.FolderBrowserDialog; $d.Description='选择日志目录（只读，当前层）'; $d.ShowNewFolderButton=$false; if($d.ShowDialog() -eq 'OK'){ConvertTo-Json -Compress -InputObject @($d.SelectedPath)}else{'[]'}"
	} else if kind != "files" {
		fail(w, 400, errors.New("invalid picker kind"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-WindowStyle", "Hidden", "-Command", "[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new(); "+script)
	data, err := command.Output()
	if err != nil {
		fail(w, 500, errors.New("系统选择器未完成，请使用输入路径"))
		return
	}
	var paths []string
	if err = json.Unmarshal(data, &paths); err != nil {
		fail(w, 500, err)
		return
	}
	if len(paths) > 0 && kind == "directory" {
		root := paths[0]
		paths = nil
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				if len(paths) >= 10000 {
					return errors.New("目录超过 10000 个文件，请使用输入路径限定范围")
				}
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			fail(w, 400, err)
			return
		}
	}
	if len(paths) > 0 {
		paths, err = logs.ExpandPaths(paths)
		if err != nil {
			fail(w, 400, err)
			return
		}
	}
	output(w, map[string]any{"paths": paths})
}

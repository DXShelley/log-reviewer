package server

import (
	"net/http"
	"time"

	"log-reviewer/internal/remote"
)

type remoteHost struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Auth          string `json:"auth"`
	Fingerprint   string `json:"fingerprint"`
	SSHDir        string `json:"ssh_dir,omitempty"`
	Path          string `json:"path"`
	Pattern       string `json:"pattern"`
	Recursive     bool   `json:"recursive"`
	MaxFiles      int    `json:"max_files"`
	MaxBytes      int64  `json:"max_bytes"`
	LastConnected string `json:"last_connected"`
}

func (s *Server) remoteHosts(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	s.access.mu.RLock()
	defer s.access.mu.RUnlock()
	hosts := s.access.config.RemoteHosts
	if hosts == nil {
		hosts = []remoteHost{}
	}
	output(w, map[string]any{"hosts": hosts})
}

func (s *Server) rememberRemote(options remote.Options) error {
	auth := "password"
	if options.PrivateKey != "" {
		auth = "key"
	}
	host := remoteHost{Host: options.Host, Port: options.Port, User: options.User, Auth: auth, Fingerprint: options.Fingerprint, SSHDir: options.SSHDir, Path: options.Path, Pattern: options.Pattern, Recursive: options.Recursive, MaxFiles: options.MaxFiles, MaxBytes: options.MaxBytes, LastConnected: time.Now().Format(time.RFC3339)}
	s.access.mu.Lock()
	defer s.access.mu.Unlock()
	cfg := s.access.config
	cfg.RemoteHosts = []remoteHost{host}
	for _, previous := range s.access.config.RemoteHosts {
		if previous.Host == host.Host && previous.Port == host.Port && previous.User == host.User && previous.Auth == host.Auth {
			continue
		}
		if len(cfg.RemoteHosts) >= 20 {
			break
		}
		cfg.RemoteHosts = append(cfg.RemoteHosts, previous)
	}
	if s.access.path != "" {
		if err := s.access.save(cfg); err != nil {
			return err
		}
	}
	s.access.config = cfg
	return nil
}

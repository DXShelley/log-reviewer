package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type accessConfig struct {
	IncludeAll       bool         `json:"include_all,omitempty"`
	RemoteDirectory  string       `json:"remote_directory,omitempty"`
	FilePatterns     []string     `json:"file_patterns,omitempty"`
	AllowedClientIPs []string     `json:"allowed_client_ips"`
	RemoteHosts      []remoteHost `json:"remote_hosts,omitempty"`
}

type accessPolicy struct {
	mu       sync.RWMutex
	path     string
	config   accessConfig
	prefixes []netip.Prefix
}

func parseClientIPs(values []string) ([]netip.Prefix, error) {
	if len(values) > 256 {
		return nil, errors.New("at most 256 IP rules are allowed")
	}
	var prefixes []netip.Prefix
	for _, value := range values {
		value = strings.TrimSpace(value)
		if addr, err := netip.ParseAddr(value); err == nil && addr.Zone() == "" {
			addr = addr.Unmap()
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("invalid client IP or CIDR: %q", value)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func clientAllowed(remote string, prefixes []netip.Prefix) bool {
	if len(prefixes) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.WithZone("").Unmap()
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ConfigureAccess loads persisted policy before serving; a non-nil override is saved.
func (s *Server) ConfigureAccess(path string, override []string) error {
	p := &s.access
	p.path = path
	cfg := accessConfig{AllowedClientIPs: []string{}}
	data, err := os.ReadFile(path)
	if err == nil {
		var stored accessConfig
		if err = json.Unmarshal(data, &stored); err != nil {
			return fmt.Errorf("invalid access state: %w", err)
		}
		if stored.AllowedClientIPs == nil {
			return errors.New("access state must contain allowed_client_ips array")
		}
		cfg = stored
	} else if !os.IsNotExist(err) {
		return err
	}
	if override != nil {
		cfg.AllowedClientIPs = override
	}
	prefixes, err := parseClientIPs(cfg.AllowedClientIPs)
	if err != nil {
		return err
	}
	if override != nil {
		if err := p.save(cfg); err != nil {
			return err
		}
	}
	p.config, p.prefixes = cfg, prefixes
	base := cfg.RemoteDirectory
	if base == "" {
		base = defaultRemoteDirectory()
	}
	s.remoteDir = s.sessionRemoteDirectory(base)
	return nil
}

func (p *accessPolicy) save(cfg accessConfig) error {
	if p.path == "" {
		return errors.New("access state file is not configured")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p.path), ".log-reviewer-access-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), p.path)
}

func (s *Server) allowedClient(remote string) bool {
	s.access.mu.RLock()
	defer s.access.mu.RUnlock()
	return clientAllowed(remote, s.access.prefixes)
}

func (s *Server) accessSettings(w http.ResponseWriter, r *http.Request) {
	s.access.mu.Lock()
	defer s.access.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		output(w, map[string]any{"allowed_client_ips": s.access.config.AllowedClientIPs, "remote_addr": r.RemoteAddr})
	case http.MethodPut:
		var cfg accessConfig
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			fail(w, 400, err)
			return
		}
		if cfg.AllowedClientIPs == nil {
			fail(w, 400, errors.New("allowed_client_ips array is required"))
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			fail(w, 400, errors.New("expected one JSON object"))
			return
		}
		prefixes, err := parseClientIPs(cfg.AllowedClientIPs)
		if err != nil {
			fail(w, 400, err)
			return
		}
		if !clientAllowed(r.RemoteAddr, prefixes) {
			fail(w, 400, errors.New("rules would block your current connection"))
			return
		}
		cfg.RemoteHosts = s.access.config.RemoteHosts
		cfg.RemoteDirectory = s.access.config.RemoteDirectory
		cfg.FilePatterns = s.access.config.FilePatterns
		cfg.IncludeAll = s.access.config.IncludeAll
		if err := s.access.save(cfg); err != nil {
			fail(w, 500, errors.New("cannot persist access settings"))
			return
		}
		s.access.config, s.access.prefixes = cfg, prefixes
		output(w, cfg)
	default:
		w.Header().Set("Allow", "GET, PUT")
		fail(w, 405, errors.New("method not allowed"))
	}
}

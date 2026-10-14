package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"log-reviewer/internal/logs"
)

//go:embed web/*
var assets embed.FS

type Server struct {
	rules            *logs.RuleStore
	mu               sync.RWMutex
	jobs             sync.WaitGroup
	Report           *logs.Report
	Progress         *logs.Progress
	busy             bool
	closing          bool
	err              string
	dir, token, addr string
	uploads          map[string]string
	allowedHosts     []string
	remoteState      remoteState
	remoteCancel     context.CancelFunc
	remoteSources    map[string]logs.RemoteSource
}

func New(dir string) *Server {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return &Server{dir: dir, token: hex.EncodeToString(b), uploads: map[string]string{}, remoteSources: map[string]logs.RemoteSource{}, rules: &logs.RuleStore{Dir: filepath.Join(dir, "rules")}}
}
func (s *Server) Token() string { return s.token }
func (s *Server) Handler(addr string) http.Handler {
	s.addr = addr
	m := http.NewServeMux()
	sub, _ := fs.Sub(assets, "web")
	m.Handle("/", http.FileServer(http.FS(sub)))
	m.HandleFunc("/api/status", s.status)
	m.HandleFunc("/api/analyze", s.analyze)
	m.HandleFunc("/api/cancel", s.cancel)
	m.HandleFunc("/api/groups", s.groups)
	m.HandleFunc("/api/details", s.details)
	m.HandleFunc("/api/export", s.export)
	m.HandleFunc("/api/report", s.report)
	m.HandleFunc("/api/browse", s.browse)
	m.HandleFunc("/api/upload", s.upload)
	m.HandleFunc("/api/language-rules", s.languageRules)
	m.HandleFunc("/api/remote-fetch", s.remoteFetch)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		if !s.validHost(r.Host, addr) {
			fail(w, 403, errors.New("invalid host"))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				fail(w, 403, errors.New("invalid origin"))
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
				fail(w, 401, errors.New("session token required"))
				return
			}
			w.Header().Set("Cache-Control", "no-store")
		}
		m.ServeHTTP(w, r)
	})
}
func (s *Server) validHost(authority, addr string) bool {
	host := authority
	if h, _, err := net.SplitHostPort(authority); err == nil {
		host = h
	} else if strings.Contains(authority, ":") {
		return false
	}
	bound, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	for _, allowed := range s.allowedHosts {
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	ipHost, _, _ := strings.Cut(host, "%")
	ip, listenIP := net.ParseIP(ipHost), net.ParseIP(bound)
	if listenIP == nil {
		return strings.EqualFold(host, bound)
	}
	if listenIP.IsUnspecified() {
		return ip != nil && !ip.IsUnspecified() || strings.EqualFold(host, "localhost")
	}
	return ip != nil && ip.Equal(listenIP) || listenIP.IsLoopback() && strings.EqualFold(host, "localhost")
}

func Serve(s *Server, port int) (net.Listener, *http.Server, error) {
	return ServeHost(s, "127.0.0.1", port, nil)
}

func ServeHost(s *Server, host string, port int, allowedHosts []string) (net.Listener, *http.Server, error) {
	if host == "" || port < 0 || port > 65535 {
		return nil, nil, errors.New("invalid host or port")
	}
	s.allowedHosts = allowedHosts
	network := "tcp"
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			network = "tcp4"
		} else {
			network = "tcp6"
		}
	}
	l, err := net.Listen(network, net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, nil, err
	}
	srv := &http.Server{Handler: s.Handler(l.Addr().String()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024}
	return l, srv, nil
}
func output(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
func method(w http.ResponseWriter, r *http.Request, m string) bool {
	if r.Method != m {
		w.Header().Set("Allow", m)
		fail(w, 405, errors.New("method not allowed"))
		return false
	}
	return true
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := map[string]any{"busy": s.busy, "error": s.err, "ready": s.Report != nil}
	if s.Progress != nil {
		v["bytes"] = s.Progress.Bytes.Load()
		v["total"] = s.Progress.Total.Load()
		v["events"] = s.Progress.Events.Load()
		v["lines"] = s.Progress.Lines.Load()
	}
	if s.Report != nil {
		v["report"] = s.Report
	}
	output(w, v)
}
func (s *Server) Start(paths []string, all bool, rules []logs.Rule, languages ...logs.LanguageRule) error {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return errors.New("server is closing")
	}
	if s.busy || s.remoteState.Busy {
		s.mu.Unlock()
		return errors.New("analysis already running")
	}
	stored, err := s.rules.Overrides()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	languages = append(stored, languages...)
	s.busy = true
	s.jobs.Add(1)
	s.err = ""
	p := &logs.Progress{}
	s.Progress = p
	origins := make(map[string]logs.RemoteSource, len(s.remoteSources))
	for path, origin := range s.remoteSources {
		origins[path] = origin
	}
	s.mu.Unlock()
	go func() {
		defer s.jobs.Done()
		r, err := logs.Analyze(paths, s.dir, logs.Options{All: all, Rules: rules, Languages: languages, Progress: p, RemoteSources: origins})
		s.mu.Lock()
		defer s.mu.Unlock()
		s.busy = false
		if err != nil {
			s.err = err.Error()
			return
		}
		s.Report = r
		logs.SaveReport(r, filepath.Join(s.dir, "latest.json"))
	}()
	return nil
}
func (s *Server) Close() {
	s.mu.Lock()
	s.closing = true
	if s.remoteCancel != nil {
		s.remoteCancel()
	}
	if s.Progress != nil {
		s.Progress.Cancel.Store(true)
	}
	s.mu.Unlock()
	s.jobs.Wait()
}
func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var req struct {
		Paths     []string            `json:"paths"`
		Uploads   []string            `json:"uploads"`
		All       bool                `json:"all"`
		Rules     []logs.Rule         `json:"rules"`
		Languages []logs.LanguageRule `json:"languages"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&req); err != nil {
		fail(w, 400, err)
		return
	}
	s.mu.RLock()
	for _, id := range req.Uploads {
		p, ok := s.uploads[id]
		if !ok {
			s.mu.RUnlock()
			fail(w, 400, errors.New("unknown upload"))
			return
		}
		req.Paths = append(req.Paths, p)
	}
	s.mu.RUnlock()
	if _, err := logs.ExpandPaths(req.Paths); err != nil {
		fail(w, 400, err)
		return
	}
	if _, err := logs.NewParser(req.Rules); err != nil {
		fail(w, 400, err)
		return
	}
	if err := s.Start(req.Paths, req.All, req.Rules, req.Languages...); err != nil {
		fail(w, 409, err)
		return
	}
	output(w, map[string]bool{"accepted": true})
}
func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Progress != nil {
		s.Progress.Cancel.Store(true)
	}
	output(w, map[string]bool{"ok": true})
}
func (s *Server) getReport() (*logs.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Report == nil {
		return nil, errors.New("analyze files first")
	}
	return s.Report, nil
}
func filter(v url.Values) (logs.Filter, error) {
	f := logs.Filter{Group: v.Get("group"), Level: v.Get("level"), Keyword: v.Get("keyword"), Regex: v.Get("regex"), From: v.Get("from"), To: v.Get("to"), Source: -1}
	if str := v.Get("source"); str != "" {
		n, err := strconv.Atoi(str)
		if err != nil {
			return f, err
		}
		f.Source = n
	}
	return f, nil
}
func (s *Server) groups(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	report, err := s.getReport()
	if err != nil {
		fail(w, 409, err)
		return
	}
	f, err := filter(r.URL.Query())
	if err != nil {
		fail(w, 400, err)
		return
	}
	g, err := logs.Summarize(report, f)
	if err != nil {
		fail(w, 400, err)
		return
	}
	if g == nil {
		g = []*logs.Group{}
	}
	output(w, map[string]any{"groups": g, "sources": report.Sources})
}
func (s *Server) details(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	report, err := s.getReport()
	if err != nil {
		fail(w, 409, err)
		return
	}
	f, err := filter(r.URL.Query())
	if err != nil {
		fail(w, 400, err)
		return
	}
	offset := 0
	limit := 20
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil {
			fail(w, 400, err)
			return
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil {
			fail(w, 400, err)
			return
		}
	}
	context := logs.ContextOptions{Before: 20, After: 100}
	for key, target := range map[string]*int{"before": &context.Before, "after": &context.After} {
		if value := r.URL.Query().Get(key); value != "" {
			*target, err = strconv.Atoi(value)
			if err != nil {
				fail(w, 400, err)
				return
			}
		}
	}
	d, more, err := logs.Details(report, f, offset, limit, context)
	if err != nil {
		fail(w, 400, err)
		return
	}
	output(w, map[string]any{"details": d, "more": more, "offset": offset})
}
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	report, err := s.getReport()
	if err != nil {
		fail(w, 409, err)
		return
	}
	f, err := filter(r.URL.Query())
	if err != nil {
		fail(w, 400, err)
		return
	} // Stage export so read failures cannot produce a success-looking partial download.
	tmp, err := os.CreateTemp(s.dir, "export-*.txt")
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err = logs.Export(report, f, tmp); err != nil {
		fail(w, 400, err)
		return
	}
	tmp.Seek(0, 0)
	w.Header().Set("Content-Disposition", "attachment; filename=log-reviewer-export.txt")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if st, err := tmp.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	}
	io.Copy(w, tmp)
}
func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	report, err := s.getReport()
	if err != nil {
		fail(w, 409, err)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=log-reviewer-report.json")
	output(w, report)
}

type entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		path, _ = os.Getwd()
	}
	if path == "roots" {
		roots := []entry{}
		if runtime.GOOS == "windows" {
			for c := 'A'; c <= 'Z'; c++ {
				p := string(c) + ":\\"
				if _, err := os.Stat(p); err == nil {
					roots = append(roots, entry{Name: p, Path: p, Dir: true})
				}
			}
		} else {
			roots = append(roots, entry{Name: "/", Path: "/", Dir: true})
		}
		output(w, map[string]any{"path": "roots", "parent": "roots", "entries": roots})
		return
	}
	path, err := filepath.Abs(path)
	if err != nil {
		fail(w, 400, err)
		return
	}
	items, err := os.ReadDir(path)
	if err != nil {
		fail(w, 400, err)
		return
	}
	out := []entry{}
	for _, item := range items {
		if strings.HasPrefix(item.Name(), ".") {
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		if !item.IsDir() && !info.Mode().IsRegular() {
			continue
		}
		out = append(out, entry{Name: item.Name(), Path: filepath.Join(path, item.Name()), Dir: item.IsDir(), Size: info.Size()})
	}
	output(w, map[string]any{"path": path, "parent": filepath.Dir(path), "entries": out})
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	name := filepath.Base(r.URL.Query().Get("name"))
	if name == "." || name == "" {
		fail(w, 400, errors.New("filename required"))
		return
	}
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)
	dir := filepath.Join(s.dir, "uploads", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		fail(w, 500, err)
		return
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		fail(w, 500, err)
		return
	}
	_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, 10*1024*1024*1024))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		os.Remove(path)
		if err == nil {
			err = closeErr
		}
		fail(w, 400, err)
		return
	}
	s.mu.Lock()
	s.uploads[id] = path
	s.mu.Unlock()
	output(w, map[string]string{"id": id, "name": name})
}

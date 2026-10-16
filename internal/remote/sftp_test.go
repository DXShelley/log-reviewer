package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh/knownhosts"
	"log-reviewer/internal/testsupport/sftpfixture"
)

type deniedFS struct {
	remoteFS
	err error
}

func (f deniedFS) Lstat(string) (os.FileInfo, error) { return nil, f.err }

func TestRemotePathDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{os.ErrPermission, "权限不足"},
		{os.ErrNotExist, "无法确认路径不存在"},
	} {
		_, err := collectFiles(context.Background(), deniedFS{err: tc.err}, Options{Path: "/home/dxshelley/docker", Pattern: "*.log"}, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "读取路径属性") || !strings.Contains(err.Error(), "/home/dxshelley/docker") || !errors.Is(err, tc.err) {
			t.Fatalf("missing diagnostic or original cause: %v", err)
		}
	}
}

func TestSFTPDownloadAndTrust(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "logs", "nested"), 0700)
	os.WriteFile(filepath.Join(root, "logs", "a.log"), []byte("ERROR a\n"), 0600)
	os.WriteFile(filepath.Join(root, "logs", "skip.csv"), []byte("ignored"), 0600)
	os.WriteFile(filepath.Join(root, "logs", "nested", "b.log"), []byte("WARN b\n"), 0600)
	server, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	options := Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: server.Fingerprint, Path: "logs"}
	destination := t.TempDir()
	manifest, err := Download(context.Background(), options, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].RemotePath != "logs/a.log" {
		t.Fatalf("%+v", manifest)
	}
	body, _ := os.ReadFile(manifest.Files[0].LocalPath)
	if string(body) != "ERROR a\n" {
		t.Fatal(string(body))
	}
	saved, _ := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if strings.Contains(string(saved), "fixture-password") {
		t.Fatal("credential leaked to manifest")
	}
	options.Recursive = true
	manifest, err = Download(context.Background(), options, t.TempDir(), nil)
	if err != nil || len(manifest.Files) != 2 {
		t.Fatalf("%+v %v", manifest, err)
	}
	options.Path = "logs/a.log"
	options.Path = "logs\nlogs/nested\nlogs/a.log\n"
	options.MaxFiles = 2
	manifest, err = Download(context.Background(), options, t.TempDir(), nil)
	if err != nil || len(manifest.Files) != 2 {
		t.Fatalf("multiple overlapping paths must yield two unique files: %+v %v", manifest, err)
	}
	options.Path = "logs/a.log"
	options.Password = ""
	options.PrivateKey = server.PrivateKey
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err != nil {
		t.Fatal("private key", err)
	}
	options.PrivateKey = server.EncryptedKey
	options.Passphrase = "fixture-passphrase"
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err != nil {
		t.Fatal("encrypted key", err)
	}
	known := filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(known, []byte(knownhosts.Line([]string{server.Listener.Addr().String()}, server.HostKey)+"\n"), 0600)
	options.Fingerprint = ""
	options.KnownHosts = known
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err != nil {
		t.Fatal("known hosts", err)
	}
	options.KnownHosts = ""
	options.SSHDir = filepath.Dir(known)
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err != nil {
		t.Fatal("ssh directory", err)
	}
	options.Fingerprint = "SHA256:wrong"
	emptyKnown := filepath.Join(t.TempDir(), "known_hosts")
	unknown := options
	unknown.Fingerprint = ""
	unknown.KnownHosts = emptyKnown
	for _, missing := range []bool{true, false} {
		if !missing {
			os.WriteFile(emptyKnown, nil, 0600)
		}
		if _, err = Download(context.Background(), unknown, t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), server.Fingerprint) || !strings.Contains(err.Error(), "尚未受信任") {
			t.Fatalf("unknown host must report fingerprint without accepting it: %v", err)
		}
	}
	options.KnownHosts = ""
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err == nil {
		t.Fatal("wrong fingerprint accepted")
	}
	options.Fingerprint = server.Fingerprint
	options.PrivateKey = ""
	options.Passphrase = ""
	options.Password = "bad-password"
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err == nil {
		t.Fatal("bad password accepted")
	}
	options.Password = "fixture-password"
	options.Path = "logs"
	options.MaxFiles = 1
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err == nil {
		t.Fatal("file limit ignored")
	}
	options.MaxFiles = 1000
	options.MaxBytes = 1
	if _, err = Download(context.Background(), options, t.TempDir(), nil); err == nil {
		t.Fatal("byte limit ignored")
	}
}

type localFS struct {
	root  string
	short bool
}

func (f localFS) Lstat(p string) (os.FileInfo, error) { return os.Lstat(filepath.Join(f.root, p)) }
func (f localFS) Stat(p string) (os.FileInfo, error)  { return os.Stat(filepath.Join(f.root, p)) }
func (f localFS) ReadDir(p string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(filepath.Join(f.root, p))
	if err != nil {
		return nil, err
	}
	var out []os.FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}
func (f localFS) Open(p string) (io.ReadCloser, error) {
	file, err := os.Open(filepath.Join(f.root, p))
	if err != nil {
		return nil, err
	}
	if f.short {
		return &shortRead{Reader: io.LimitReader(file, 1), Closer: file}, nil
	}
	return file, nil
}

type shortRead struct {
	io.Reader
	io.Closer
}

func TestSnapshotChangeCancelAndCleanup(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "app.log")
	os.WriteFile(file, []byte("ERROR original\n"), 0600)
	options := Options{Path: "app.log", MaxFiles: 10, MaxBytes: 1000, Pattern: "*.log"}
	changed := false
	manifest, err := downloadFiles(context.Background(), localFS{root: root}, options, t.TempDir(), func(p Progress) {
		if !changed && p.Bytes > 0 {
			changed = true
			os.WriteFile(file, []byte("ERROR changed and longer\n"), 0600)
		}
	})
	if err != nil || !manifest.Files[0].Changed || len(manifest.Warnings) != 1 {
		t.Fatalf("%+v %v", manifest, err)
	}
	destination := t.TempDir()
	if _, err := downloadFiles(context.Background(), localFS{root: root, short: true}, options, destination, func(Progress) {}); err == nil {
		t.Fatal("truncated file accepted")
	}
	entries, _ := os.ReadDir(destination)
	if len(entries) != 0 {
		t.Fatal("partial files remained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := downloadFiles(ctx, localFS{root: root}, options, destination, func(p Progress) {
		if p.Bytes > 0 {
			cancel()
		}
	}); err == nil {
		t.Fatal("cancel ignored")
	}
	entries, _ = os.ReadDir(destination)
	if len(entries) != 0 {
		t.Fatal("cancel left files")
	}
}

func TestValidate(t *testing.T) {
	options := Options{Host: "localhost", User: "tester", Password: "secret", Path: "logs", SSHDir: t.TempDir()}
	if err := options.Validate(); err != nil {
		t.Fatalf("missing known_hosts should allow fingerprint discovery during handshake: %v", err)
	}
	if options.MaxFiles != 50 {
		t.Fatalf("default file limit: %d", options.MaxFiles)
	}
	for _, invalid := range []string{" \n\t", "logs\x00outside", strings.Repeat("x", 65537), strings.Repeat("logs\n", 101)} {
		candidate := options
		candidate.Path = invalid
		if candidate.Validate() == nil {
			t.Fatal("invalid remote paths accepted")
		}
	}
	options.Fingerprint = "SHA256:expected"
	options.Pattern = "["
	if options.Validate() == nil {
		t.Fatal("invalid pattern accepted")
	}
	options.Pattern = "*.log"
	options.MaxFiles = 10001
	if options.Validate() == nil {
		t.Fatal("invalid limit accepted")
	}
}

func TestFilenameMatchers(t *testing.T) {
	for _, tc := range []struct {
		rules, name string
		want        bool
	}{
		{"*.log\n*.txt", "app.log", true},
		{"*.log\r\n\n*.txt", "notes.txt", true},
		{"*.log,*.txt", "app.csv", false},
		{"regex:^app-\\d{4}-\\d{2}-\\d{2}\\.log$", "app-2026-10-15.log", true},
		{"regex:^app-\\d{4}-\\d{2}-\\d{2}\\.log$", "prefix-app-2026-10-15.log", false},
		{"regex:(?i)\\.(log|txt)$", "APP.LOG", true},
		{"regex:^a{1,3}\\.log$", "aaa.log", true},
		{"*.txt\nregex:^app\\.log$", "app.log", true},
	} {
		matchers, err := filenameMatchers(tc.rules)
		if err != nil {
			t.Fatal(err)
		}
		got := false
		for _, match := range matchers {
			got = got || match(tc.name)
		}
		if got != tc.want {
			t.Errorf("%q / %q: got %v", tc.rules, tc.name, got)
		}
	}
	for _, rules := range []string{"regex:[", "regex:", "[", ",,\n"} {
		if _, err := filenameMatchers(rules); err == nil {
			t.Errorf("invalid rules accepted: %q", rules)
		}
	}
}

func TestExcludeDirectories(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"app.log", "backup/old.log", "live/worker.log", "live/cache-1/skip.log", "live/backup/skip.log"} {
		full := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("ERROR example\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	options := Options{Path: ".", Pattern: "*.log\nexclude-dir:backup\nexclude-dir:cache*", Recursive: true, MaxFiles: 2, MaxBytes: 1000}
	manifest, err := downloadFiles(context.Background(), localFS{root: root}, options, t.TempDir(), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 2 || manifest.Files[0].RemotePath != "app.log" || manifest.Files[1].RemotePath != "live/worker.log" {
		t.Fatalf("unexpected files: %+v", manifest.Files)
	}
	for _, rule := range []string{"exclude-dir:", "exclude-dir:[", "exclude-dir:parent/child"} {
		if _, err := excludedDirectoryMatchers(rule); err == nil {
			t.Errorf("invalid directory rule accepted: %q", rule)
		}
	}
}

type linkInfo struct{ os.FileInfo }

func (linkInfo) Name() string      { return "link.log" }
func (linkInfo) Mode() os.FileMode { return os.ModeSymlink | 0600 }

type linkFS struct{ localFS }

func (f linkFS) Lstat(p string) (os.FileInfo, error) {
	if p == "." {
		info, err := f.localFS.Lstat(p)
		return linkInfo{info}, err
	}
	if p == "link.log" {
		info, err := f.localFS.Lstat("app.log")
		return linkInfo{info}, err
	}
	return f.localFS.Lstat(p)
}
func (f linkFS) Stat(p string) (os.FileInfo, error) {
	if p == "link.log" {
		p = "app.log"
	}
	return f.localFS.Stat(p)
}
func (f linkFS) Open(p string) (io.ReadCloser, error) {
	if p == "link.log" {
		p = "app.log"
	}
	return f.localFS.Open(p)
}
func (f linkFS) ReadDir(p string) ([]os.FileInfo, error) {
	items, err := f.localFS.ReadDir(p)
	if err == nil && len(items) > 0 {
		items = append(items, linkInfo{items[0]})
	}
	return items, err
}

func TestSymlinkAndDepthBound(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "app.log"), []byte("ERROR example\n"), 0600)
	options := Options{Path: ".", Pattern: "*.log", MaxFiles: 10, MaxBytes: 1000}
	manifest, err := downloadFiles(context.Background(), linkFS{localFS{root: root}}, options, t.TempDir(), func(Progress) {})
	if err != nil || len(manifest.Files) != 1 {
		t.Fatalf("%+v %v", manifest, err)
	}
	options.Path = "link.log"
	if manifest, err := downloadFiles(context.Background(), linkFS{localFS{root: root}}, options, t.TempDir(), func(Progress) {}); err != nil || len(manifest.Files) != 1 || manifest.Files[0].RemotePath != "link.log" {
		t.Fatalf("explicit symlink must resolve: %+v %v", manifest, err)
	}
	nested := root
	for i := 0; i < 33; i++ {
		nested = filepath.Join(nested, "d")
	}
	os.MkdirAll(nested, 0700)
	options.Path = "."
	options.Recursive = true
	if _, err := downloadFiles(context.Background(), localFS{root: root}, options, t.TempDir(), func(Progress) {}); err == nil || !strings.Contains(err.Error(), "32 levels") {
		t.Fatal("depth bound", err)
	}
}

package remote

import (
	"context"
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
	if options.Validate() == nil {
		t.Fatal("unverified host accepted")
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

type linkInfo struct{ os.FileInfo }

func (linkInfo) Name() string      { return "link.log" }
func (linkInfo) Mode() os.FileMode { return os.ModeSymlink | 0600 }

type linkFS struct{ localFS }

func (f linkFS) Lstat(p string) (os.FileInfo, error) {
	if p == "link.log" {
		info, err := f.localFS.Lstat("app.log")
		return linkInfo{info}, err
	}
	return f.localFS.Lstat(p)
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
	if _, err := downloadFiles(context.Background(), linkFS{localFS{root: root}}, options, t.TempDir(), func(Progress) {}); err == nil {
		t.Fatal("selected symlink accepted")
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

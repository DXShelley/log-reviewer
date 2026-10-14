package remote

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Options struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	Passphrase  string `json:"passphrase,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	KnownHosts  string `json:"known_hosts,omitempty"`
	Path        string `json:"path"`
	Pattern     string `json:"pattern"`
	Recursive   bool   `json:"recursive"`
	MaxFiles    int    `json:"max_files"`
	MaxBytes    int64  `json:"max_bytes"`
}

type File struct {
	RemotePath string `json:"remote_path"`
	LocalPath  string `json:"local_path"`
	Size       int64  `json:"size"`
	Modified   string `json:"modified"`
	Changed    bool   `json:"changed"`
}
type Manifest struct {
	Protocol   string   `json:"protocol"`
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	User       string   `json:"user"`
	CapturedAt string   `json:"captured_at"`
	Files      []File   `json:"files"`
	Warnings   []string `json:"warnings"`
}
type Progress struct {
	Stage      string `json:"stage"`
	Bytes      int64  `json:"bytes"`
	Total      int64  `json:"total"`
	Files      int    `json:"files"`
	TotalFiles int    `json:"total_files"`
	Current    string `json:"current"`
}

func (o *Options) Validate() error {
	if o.Port == 0 {
		o.Port = 22
	}
	if o.MaxFiles == 0 {
		o.MaxFiles = 1000
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 10 << 30
	}
	if o.Pattern == "" {
		o.Pattern = "*.log,*.log.*,*.txt"
	}
	if strings.TrimSpace(o.Host) == "" || strings.ContainsAny(o.Host, "/\\\r\n") || o.Port < 1 || o.Port > 65535 || o.User == "" || o.Path == "" {
		return errors.New("host, user and remote path are required; port must be 1-65535")
	}
	if o.MaxFiles < 1 || o.MaxFiles > 10000 || o.MaxBytes < 1 || o.MaxBytes > 10<<30 {
		return errors.New("max_files must be 1-10000; max_bytes must be 1-10737418240")
	}
	if len(o.PrivateKey) > 1<<20 || len(o.Password) > 16384 || len(o.Passphrase) > 16384 {
		return errors.New("credential is too large")
	}
	if o.Password == "" && o.PrivateKey == "" {
		return errors.New("password or private key is required")
	}
	if o.Fingerprint == "" && o.KnownHosts == "" {
		return errors.New("verified SHA256 host fingerprint or known_hosts file is required")
	}
	if o.Fingerprint != "" && !strings.HasPrefix(o.Fingerprint, "SHA256:") {
		return errors.New("fingerprint must use SHA256: format")
	}
	for _, pattern := range strings.Split(o.Pattern, ",") {
		if _, err := path.Match(strings.TrimSpace(pattern), "probe.log"); err != nil {
			return fmt.Errorf("invalid filename pattern: %w", err)
		}
	}
	return nil
}

// Refresh deadlines on transport activity so large downloads can continue while stalled peers time out.
type timedConn struct{ net.Conn }

func (c timedConn) Read(b []byte) (int, error) {
	c.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Read(b)
}
func (c timedConn) Write(b []byte) (int, error) {
	c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Write(b)
}

func Download(ctx context.Context, options Options, destination string, update func(Progress)) (*Manifest, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if update == nil {
		update = func(Progress) {}
	}
	progress := Progress{Stage: "connecting"}
	update(progress)
	var auth []ssh.AuthMethod
	if options.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if options.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(options.PrivateKey), []byte(options.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(options.PrivateKey))
		}
		if err != nil {
			return nil, errors.New("cannot parse private key; verify format and passphrase")
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if options.Password != "" {
		auth = append(auth, ssh.Password(options.Password))
	}
	var check ssh.HostKeyCallback
	if options.Fingerprint != "" {
		check = func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if subtle.ConstantTimeCompare([]byte(options.Fingerprint), []byte(ssh.FingerprintSHA256(key))) != 1 {
				return errors.New("server host fingerprint mismatch")
			}
			return nil
		}
	} else {
		var err error
		check, err = knownhosts.New(options.KnownHosts)
		if err != nil {
			return nil, fmt.Errorf("cannot load known_hosts: %w", err)
		}
	}
	address := net.JoinHostPort(options.Host, strconv.Itoa(options.Port))
	conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	handshakeTimer := time.AfterFunc(30*time.Second, func() { conn.Close() })
	sshConn, channels, requests, err := ssh.NewClientConn(timedConn{conn}, address, &ssh.ClientConfig{User: options.User, Auth: auth, HostKeyCallback: check, Timeout: 30 * time.Second})
	handshakeTimer.Stop()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	client := ssh.NewClient(sshConn, channels, requests)
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("SFTP subsystem unavailable: %w", err)
	}
	defer sftpClient.Close()
	return downloadFiles(ctx, sftpFS{sftpClient}, options, destination, update)
}

type remoteFS interface {
	Lstat(string) (os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.FileInfo, error)
	Open(string) (io.ReadCloser, error)
}
type sftpFS struct{ *sftp.Client }

func (c sftpFS) Open(name string) (io.ReadCloser, error) { return c.Client.Open(name) }

type candidate struct {
	path string
	info os.FileInfo
}

func downloadFiles(ctx context.Context, client remoteFS, options Options, destination string, update func(Progress)) (*Manifest, error) {
	progress := Progress{Stage: "listing"}
	update(progress)
	selected := []candidate{}
	visited := 0
	var walk func(string, int) error
	walk = func(remotePath string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 32 {
			return errors.New("directory nesting exceeds 32 levels")
		}
		info, err := client.Lstat(remotePath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("selected path is a symbolic link")
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || len(selected) >= options.MaxFiles || info.Size() > options.MaxBytes-progress.Total {
				return errors.New("selected files exceed file count or byte limit")
			}
			selected = append(selected, candidate{remotePath, info})
			progress.Total += info.Size()
			return nil
		}
		if !info.IsDir() {
			return errors.New("selected path is not a regular file or directory")
		}
		items, err := client.ReadDir(remotePath)
		if err != nil {
			return err
		}
		for _, item := range items {
			visited++
			if visited > 100000 {
				return errors.New("directory scan exceeds 100000 entries")
			}
			name := item.Name()
			if name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
				continue
			}
			if item.Mode()&os.ModeSymlink != 0 {
				continue
			}
			next := path.Join(remotePath, name)
			if item.IsDir() {
				if options.Recursive {
					if err := walk(next, depth+1); err != nil {
						return err
					}
				}
				continue
			}
			if !item.Mode().IsRegular() {
				continue
			}
			matches := false
			for _, pattern := range strings.Split(options.Pattern, ",") {
				m, _ := path.Match(strings.TrimSpace(pattern), name)
				matches = matches || m
			}
			if matches {
				if err := walk(next, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(path.Clean(options.Path), 0); err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, errors.New("no matching regular files")
	}
	if err := os.MkdirAll(destination, 0700); err != nil {
		return nil, err
	}
	manifest := &Manifest{Protocol: "sftp", Host: options.Host, Port: options.Port, User: options.User, CapturedAt: time.Now().Format(time.RFC3339), Files: []File{}, Warnings: []string{}}
	progress.Stage = "downloading"
	progress.TotalFiles = len(selected)
	update(progress)
	success := false
	created := []string{}
	defer func() {
		if !success {
			for _, file := range created {
				os.Remove(file)
			}
		}
	}()
	for i, item := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress.Current = item.path
		update(progress)
		source, err := client.Open(item.path)
		if err != nil {
			return nil, err
		}
		local, err := os.CreateTemp(destination, fmt.Sprintf("%04d-*.part", i+1))
		if err != nil {
			source.Close()
			return nil, err
		}
		created = append(created, local.Name())
		// Read the size observed while listing, bounding growing logs and aggregate disk use.
		n, err := io.CopyBuffer(local, &progressReader{reader: io.LimitReader(source, item.info.Size()), ctx: ctx, progress: &progress, update: update}, make([]byte, 128<<10))
		source.Close()
		if err == nil {
			err = local.Sync()
		}
		closeErr := local.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}
		if n != item.info.Size() {
			return nil, fmt.Errorf("remote file truncated during download: %s", item.path)
		}
		after, err := client.Stat(item.path)
		if err != nil {
			return nil, fmt.Errorf("cannot verify remote file after download: %w", err)
		}
		changed := after.Size() != item.info.Size() || !after.ModTime().Equal(item.info.ModTime())
		if changed {
			manifest.Warnings = append(manifest.Warnings, "Remote file changed during download: "+item.path)
		}
		finalPath := strings.TrimSuffix(local.Name(), ".part") + ".log"
		if err := os.Rename(local.Name(), finalPath); err != nil {
			return nil, err
		}
		created = append(created, finalPath)
		manifest.Files = append(manifest.Files, File{RemotePath: item.path, LocalPath: finalPath, Size: n, Modified: item.info.ModTime().Format(time.RFC3339), Changed: changed})
		progress.Files++
		update(progress)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(destination, "manifest.json")
	mf, err := os.OpenFile(manifestPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	created = append(created, manifestPath)
	_, err = mf.Write(data)
	closeErr := mf.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	success = true
	progress.Stage = "complete"
	update(progress)
	return manifest, nil
}

type progressReader struct {
	reader   io.Reader
	ctx      context.Context
	progress *Progress
	update   func(Progress)
}

func (r *progressReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(b)
	r.progress.Bytes += int64(n)
	r.update(*r.progress)
	return n, err
}

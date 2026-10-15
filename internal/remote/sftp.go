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
	"regexp"
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
	SSHDir      string `json:"ssh_dir,omitempty"`
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

type HostKeyError struct {
	Host        string `json:"host"`
	Fingerprint string `json:"fingerprint"`
}

func (e *HostKeyError) Error() string {
	return fmt.Sprintf("服务器 %s 尚未受信任。主机指纹：%s。请核实指纹后确认使用，或先通过 ssh 连接建立 known_hosts 信任。", e.Host, e.Fingerprint)
}

// Each line is one rule; comma-separated globs remain compatible with the CLI.
func filenameMatchers(patterns string) ([]func(string) bool, error) {
	var matches []func(string) bool
	for line, text := range strings.Split(patterns, "\n") {
		text = strings.TrimSpace(text)
		if text == "" || strings.HasPrefix(text, "exclude-dir:") {
			continue
		}
		if strings.HasPrefix(text, "regex:") {
			expression := strings.TrimSpace(strings.TrimPrefix(text, "regex:"))
			if expression == "" {
				return nil, fmt.Errorf("第 %d 行正则表达式不能为空", line+1)
			}
			re, err := regexp.Compile(expression)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行正则表达式无效：%w", line+1, err)
			}
			matches = append(matches, re.MatchString)
			continue
		}
		for _, pattern := range strings.Split(text, ",") {
			pattern := strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			if _, err := path.Match(pattern, "probe.log"); err != nil {
				return nil, fmt.Errorf("第 %d 行文件名通配符无效：%w", line+1, err)
			}
			matches = append(matches, func(name string) bool { matched, _ := path.Match(pattern, name); return matched })
		}
	}
	if len(matches) == 0 {
		return nil, errors.New("请至少填写一条文件名过滤规则")
	}
	return matches, nil
}

func excludedDirectoryMatchers(patterns string) ([]func(string) bool, error) {
	var matches []func(string) bool
	for line, text := range strings.Split(patterns, "\n") {
		text = strings.TrimSpace(text)
		if !strings.HasPrefix(text, "exclude-dir:") {
			continue
		}
		pattern := strings.TrimSpace(strings.TrimPrefix(text, "exclude-dir:"))
		if pattern == "" || strings.ContainsAny(pattern, "/\\") {
			return nil, fmt.Errorf("第 %d 行忽略目录规则应填写目录名通配符，不含路径分隔符", line+1)
		}
		if _, err := path.Match(pattern, "backup"); err != nil {
			return nil, fmt.Errorf("第 %d 行忽略目录规则无效：%w", line+1, err)
		}
		matches = append(matches, func(name string) bool { matched, _ := path.Match(pattern, name); return matched })
	}
	return matches, nil
}

func (o *Options) Validate() error {
	if o.Fingerprint == "" && o.KnownHosts == "" {
		dir := o.SSHDir
		if dir == "" {
			if home, err := os.UserHomeDir(); err == nil {
				dir = filepath.Join(home, ".ssh")
			}
		}
		if dir != "" {
			o.KnownHosts = filepath.Join(dir, "known_hosts")
		}
	}
	if o.Port == 0 {
		o.Port = 22
	}
	if o.MaxFiles == 0 {
		o.MaxFiles = 50
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 10 << 30
	}
	if strings.TrimSpace(o.Pattern) == "" {
		o.Pattern = "*.log,*.log.*,*.txt,*.text,*.out,*.err,*.trace"
	}
	if strings.TrimSpace(o.Host) == "" || strings.ContainsAny(o.Host, "/\\\r\n") || o.Port < 1 || o.Port > 65535 || strings.TrimSpace(o.User) == "" || strings.TrimSpace(o.Path) == "" {
		return errors.New("host, user and remote path are required; port must be 1-65535")
	}
	if len(o.Path) > 65536 || len(strings.Split(o.Path, "\n")) > 100 || strings.ContainsRune(o.Path, '\x00') {
		return errors.New("remote paths must contain at most 100 lines and 64 KiB, without NUL characters")
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
	if o.Fingerprint == "" {
		if _, err := knownhosts.New(o.KnownHosts); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("load known_hosts: %w", err)
			}
		}
	}
	if o.Fingerprint != "" && !strings.HasPrefix(o.Fingerprint, "SHA256:") {
		return errors.New("fingerprint must use SHA256: format")
	}
	if _, err := excludedDirectoryMatchers(o.Pattern); err != nil {
		return err
	}
	_, err := filenameMatchers(o.Pattern)
	return err
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
	var manifest *Manifest
	err := withSFTP(ctx, options, update, func(client remoteFS, verified Options) error {
		var err error
		manifest, err = downloadFiles(ctx, client, verified, destination, update)
		return err
	})
	return manifest, err
}

type PreviewFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

func Preview(ctx context.Context, options Options, update func(Progress)) ([]PreviewFile, string, error) {
	files := []PreviewFile{}
	var fingerprint string
	err := withSFTP(ctx, options, update, func(client remoteFS, verified Options) error {
		fingerprint = verified.Fingerprint
		// Preview metadata independently of the eventual download selection limits.
		verified.MaxFiles, verified.MaxBytes = 10000, 1<<63-1
		items, err := collectFiles(ctx, client, verified, update)
		if err != nil && err.Error() == "no matching regular files" {
			return nil
		}
		for _, item := range items {
			files = append(files, PreviewFile{Path: item.path, Size: item.info.Size(), Modified: item.info.ModTime().Format(time.RFC3339)})
		}
		return err
	})
	return files, fingerprint, err
}

func DownloadSelected(ctx context.Context, options Options, files []PreviewFile, destination string, update func(Progress)) (*Manifest, error) {
	var manifest *Manifest
	err := withSFTP(ctx, options, update, func(client remoteFS, verified Options) error {
		if len(files) == 0 || len(files) > verified.MaxFiles {
			return errors.New("请选择文件，且不能超过最多文件数")
		}
		items := make([]candidate, 0, len(files))
		var total int64
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := client.Stat(file.Path)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > verified.MaxBytes-total {
				return errors.New("所选文件类型或总大小超出限制")
			}
			if info.Size() != file.Size || info.ModTime().Format(time.RFC3339) != file.Modified {
				return fmt.Errorf("远程文件已变化，请刷新预览：%s", file.Path)
			}
			total += info.Size()
			items = append(items, candidate{file.Path, info})
		}
		var err error
		manifest, err = transferFiles(ctx, client, verified, items, destination, update)
		return err
	})
	return manifest, err
}

func withSFTP(ctx context.Context, options Options, update func(Progress), run func(remoteFS, Options) error) error {
	if err := options.Validate(); err != nil {
		return err
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
			return fmt.Errorf("无法解析私钥，请检查格式和口令：%w", err)
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
			if !os.IsNotExist(err) {
				return fmt.Errorf("cannot load known_hosts: %w", err)
			}
			check = func(string, net.Addr, ssh.PublicKey) error { return &knownhosts.KeyError{} }
		}
		verify := check
		check = func(host string, address net.Addr, key ssh.PublicKey) error {
			err := verify(host, address, key)
			var keyError *knownhosts.KeyError
			if errors.As(err, &keyError) {
				if len(keyError.Want) == 0 {
					return &HostKeyError{Host: host, Fingerprint: ssh.FingerprintSHA256(key)}
				}
				return fmt.Errorf("服务器 %s 的主机密钥与 known_hosts 不一致，请核实服务器身份后更新记录：%s；当前指纹：%s", host, options.KnownHosts, ssh.FingerprintSHA256(key))
			}
			return err
		}
	}
	address := net.JoinHostPort(options.Host, strconv.Itoa(options.Port))
	verifyKey := check
	check = func(host string, addr net.Addr, key ssh.PublicKey) error {
		if err := verifyKey(host, addr, key); err != nil {
			return err
		}
		options.Fingerprint = ssh.FingerprintSHA256(key)
		return nil
	}
	conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
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
			return ctx.Err()
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return fmt.Errorf("SSH 认证失败：请检查用户名、密码（包括末尾标点）及服务器是否允许所选认证方式：%w", err)
		}
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	client := ssh.NewClient(sshConn, channels, requests)
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP subsystem unavailable: %w", err)
	}
	defer sftpClient.Close()
	return run(sftpFS{sftpClient}, options)
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

func remotePathError(operation, remotePath string, err error) error {
	reason := "远程访问失败"
	switch {
	case errors.Is(err, os.ErrPermission):
		reason = "权限不足：请检查 SFTP 登录账户对文件的读取权限、目录的读取权限及各级父目录的执行（穿越）权限"
	case errors.Is(err, os.ErrNotExist):
		reason = "SFTP 服务端返回路径不存在；无法确认路径不存在，也可能是服务端隐藏无权访问的路径、符号链接目标不可访问或 SFTP 根目录限制，请核对登录账户和目录权限"
	}
	return fmt.Errorf("%s %q：%s（原始错误：%w）", operation, remotePath, reason, err)
}

func collectFiles(ctx context.Context, client remoteFS, options Options, update func(Progress)) ([]candidate, error) {
	if update == nil {
		update = func(Progress) {}
	}
	matchers, err := filenameMatchers(options.Pattern)
	if err != nil {
		return nil, err
	}
	excludedDirs, err := excludedDirectoryMatchers(options.Pattern)
	if err != nil {
		return nil, err
	}
	progress := Progress{Stage: "listing"}
	update(progress)
	selected := []candidate{}
	seen := map[string]bool{}
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
			return remotePathError("读取路径属性", remotePath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if depth != 0 {
				return nil
			}
			info, err = client.Stat(remotePath)
			if err != nil {
				return remotePathError("解析所选路径的符号链接", remotePath, err)
			}
		}
		if info.Mode().IsRegular() {
			if seen[remotePath] {
				return nil
			}
			seen[remotePath] = true
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
			return remotePathError("列出目录", remotePath, err)
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
				excluded := false
				for _, match := range excludedDirs {
					if match(name) {
						excluded = true
						break
					}
				}
				if options.Recursive && !excluded {
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
			for _, match := range matchers {
				if match(name) {
					matches = true
					break
				}
			}
			if matches {
				if err := walk(next, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, entry := range strings.Split(options.Path, "\n") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if err := walk(path.Clean(entry), 0); err != nil {
			return nil, fmt.Errorf("%s: %w", entry, err)
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("no matching regular files")
	}
	return selected, nil
}

func downloadFiles(ctx context.Context, client remoteFS, options Options, destination string, update func(Progress)) (*Manifest, error) {
	selected, err := collectFiles(ctx, client, options, update)
	if err != nil {
		return nil, err
	}
	return transferFiles(ctx, client, options, selected, destination, update)
}

func transferFiles(ctx context.Context, client remoteFS, options Options, selected []candidate, destination string, update func(Progress)) (*Manifest, error) {
	if update == nil {
		update = func(Progress) {}
	}
	progress := Progress{}
	for _, item := range selected {
		progress.Total += item.info.Size()
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
			return nil, remotePathError("打开远程文件", item.path, err)
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
			return nil, remotePathError("下载后校验远程文件", item.path, err)
		}
		changed := after.Size() != item.info.Size() || !after.ModTime().Equal(item.info.ModTime())
		if changed {
			manifest.Warnings = append(manifest.Warnings, "Remote file changed during download: "+item.path)
		}
		finalPath, err := downloadFilename(destination, path.Base(item.path))
		if err != nil {
			return nil, err
		}
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

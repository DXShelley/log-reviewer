package remote

import (
	"context"
	"errors"
	"fmt"
	"github.com/pkg/sftp"
	"io"
	"log-reviewer/internal/logs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DirectSource keeps credentials in process memory and opens SFTP files read-only.
type DirectSource struct {
	Context  context.Context
	Options  Options
	File     PreviewFile
	CacheDir string
	mu       sync.Mutex
	cached   string
}

// RelocateCache updates private cache paths after the server moves its files.
func (s *DirectSource) RelocateCache(oldRoot, newRoot string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	move := func(p string) string {
		rel, err := filepath.Rel(oldRoot, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return p
		}
		return filepath.Join(newRoot, rel)
	}
	s.CacheDir = move(s.CacheDir)
	if s.cached != "" {
		s.cached = move(s.cached)
	}
}

func (s *DirectSource) context() context.Context {
	if s.Context != nil {
		return s.Context
	}
	return context.Background()
}

func networkFailure(err error) bool {
	var n net.Error
	var status *sftp.StatusError
	lost := errors.As(err, &status) && (status.Code == 6 || status.Code == 7)
	return err != nil && (lost || errors.As(err, &n) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed))
}

type sessionFile struct {
	logs.ReadFile
	cancel context.CancelFunc
	done   <-chan error
	once   sync.Once
}

func (f *sessionFile) Close() error {
	var err error
	f.once.Do(func() { err = f.ReadFile.Close(); f.cancel(); <-f.done })
	return err
}

func (s *DirectSource) connect() (logs.ReadFile, error) {
	ctx, cancel := context.WithCancel(s.context())
	ready := make(chan logs.ReadFile, 1)
	done := make(chan error, 1)
	go func() {
		done <- withSFTP(ctx, s.Options, nil, func(client remoteFS, _ Options) error {
			raw, err := client.Open(s.File.Path)
			if err != nil {
				return err
			}
			f, ok := raw.(logs.ReadFile)
			if !ok {
				raw.Close()
				return errors.New("SFTP file does not support random access")
			}
			st, err := f.Stat()
			if err != nil {
				f.Close()
				return err
			}
			if !st.Mode().IsRegular() || st.Size() != s.File.Size || st.ModTime().Format(time.RFC3339) != s.File.Modified {
				f.Close()
				return fmt.Errorf("远程文件已变化，请重新预览：%s", s.File.Path)
			}
			ready <- f
			<-ctx.Done()
			return nil
		})
	}()
	select {
	case f := <-ready:
		return &sessionFile{ReadFile: f, cancel: cancel, done: done}, nil
	case err := <-done:
		cancel()
		return nil, err
	}
}

func (s *DirectSource) cachedFile(cause error) (logs.ReadFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached == "" {
		if !networkFailure(cause) {
			return nil, cause
		}
		ctx, cancel := context.WithTimeout(s.context(), 5*time.Minute)
		defer cancel()
		manifest, err := DownloadSelected(ctx, s.Options, []PreviewFile{s.File}, s.CacheDir, nil)
		if err != nil {
			return nil, fmt.Errorf("远程直读失败（%v），缓存重试失败：%w", cause, err)
		}
		if manifest.Files[0].Changed {
			return nil, errors.New("远程文件在缓存过程中变化，请重新预览")
		}
		s.cached = manifest.Files[0].LocalPath
	}
	f, err := os.Open(s.cached)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	modified, err := time.Parse(time.RFC3339, s.File.Modified)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &cachedReadFile{ReadFile: f, info: cachedInfo{FileInfo: st, modified: modified}}, nil
}

type cachedInfo struct {
	os.FileInfo
	modified time.Time
}

func (i cachedInfo) ModTime() time.Time { return i.modified }

type cachedReadFile struct {
	logs.ReadFile
	info os.FileInfo
}

func (f *cachedReadFile) Stat() (os.FileInfo, error) { return f.info, nil }

func (s *DirectSource) Open() (logs.ReadFile, error) {
	s.mu.Lock()
	cached := s.cached != ""
	s.mu.Unlock()
	if cached {
		return s.cachedFile(nil)
	}
	f, err := s.connect()
	if err != nil {
		return s.cachedFile(err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &resilientFile{source: s, file: f, size: st.Size()}, nil
}

type resilientFile struct {
	source       *DirectSource
	file         logs.ReadFile
	offset, size int64
	mu           sync.Mutex
}

func (f *resilientFile) ReadAt(p []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.file.ReadAt(p, off)
	if networkFailure(err) && !(errors.Is(err, io.EOF) && off+int64(n) >= f.size) {
		replacement, retryErr := f.source.cachedFile(err)
		if retryErr != nil {
			return n, retryErr
		}
		f.file.Close()
		f.file = replacement
		return f.file.ReadAt(p, off)
	}
	return n, err
}
func (f *resilientFile) Read(p []byte) (int, error) {
	n, err := f.ReadAt(p, f.offset)
	f.offset += int64(n)
	return n, err
}
func (f *resilientFile) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		off += f.offset
	case io.SeekEnd:
		off += f.size
	default:
		return 0, errors.New("invalid whence")
	}
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	f.offset = off
	return off, nil
}
func (f *resilientFile) Stat() (os.FileInfo, error) {
	st, err := f.file.Stat()
	if networkFailure(err) {
		replacement, retryErr := f.source.cachedFile(err)
		if retryErr != nil {
			return nil, retryErr
		}
		f.file.Close()
		f.file = replacement
		return f.file.Stat()
	}
	return st, err
}
func (f *resilientFile) Close() error { return f.file.Close() }

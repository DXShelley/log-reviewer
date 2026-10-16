package remote

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/testsupport/sftpfixture"
)

func TestSFTPChineseEncoding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "export.log")
	raw := []byte("2026-10-16 12:00:00 ERROR demo : " + string([]byte{0xb5, 0xbc, 0xb3, 0xf6, 0xbd, 0xe1, 0xca, 0xf8}) + "\r\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	server, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: server.Fingerprint, Path: "export.log"}
	for _, mode := range []string{"download", "direct"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			key := path
			opt := logs.Options{}
			if mode == "download" {
				manifest, err := Download(ctx, options, t.TempDir(), nil)
				if err != nil || len(manifest.Files) != 1 {
					t.Fatalf("download: %+v %v", manifest, err)
				}
				key = manifest.Files[0].LocalPath
				got, err := os.ReadFile(key)
				if err != nil || !bytes.Equal(got, raw) {
					t.Fatalf("download bytes changed: %v", err)
				}
			} else {
				files, _, err := Preview(ctx, options, nil)
				if err != nil || len(files) != 1 {
					t.Fatalf("preview: %+v %v", files, err)
				}
				source := &DirectSource{Options: options, File: files[0], CacheDir: filepath.Join(t.TempDir(), "cache")}
				key = "sftp:encoding-test"
				opt.Readers = map[string]logs.SourceOpener{key: source.Open}
			}
			report, err := logs.Analyze([]string{key}, t.TempDir(), opt)
			if err != nil {
				t.Fatal(err)
			}
			d, _, err := logs.Details(report, logs.Filter{Source: -1, Keyword: "导出结束"}, 0, 10)
			expected := "2026-10-16 12:00:00 ERROR demo : 导出结束\r\n"
			if err != nil || len(d) != 1 || d[0].Text != expected || d[0].Length != uint64(len(raw)) {
				t.Fatalf("details=%+v err=%v", d, err)
			}
			var out bytes.Buffer
			if err := logs.Export(report, logs.Filter{Source: -1, Regex: "导出.*束"}, &out); err != nil || !strings.Contains(out.String(), expected) {
				t.Fatalf("export=%q err=%v", out.String(), err)
			}
			out.Reset()
			if err := logs.ExportTime(ctx, report, 0, "2026-10-16T12:00:00", "2026-10-16T12:00:00", &out); err != nil || out.String() != expected {
				t.Fatalf("time export=%q err=%v", out.String(), err)
			}
		})
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("remote source modified: %v", err)
	}
}

package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pkg/sftp"
	"io"
	"log-reviewer/internal/logs"
	"log-reviewer/internal/testsupport/sftpfixture"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type brokenRead struct{ logs.ReadFile }

func (f brokenRead) ReadAt([]byte, int64) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestDirectSourceAndNetworkFallback(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "app.log")
	content := []byte("2026-10-15 12:30:00 ERROR app : failed\n")
	if err := os.WriteFile(sourcePath, content, 0400); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(sourcePath)
	server, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	options := Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: server.Fingerprint, Path: "app.log"}
	files, _, err := Preview(context.Background(), options, nil)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	source := &DirectSource{Options: options, File: files[0], CacheDir: cache}
	key := "sftp:test"
	report, err := logs.Analyze([]string{key}, t.TempDir(), logs.Options{Readers: map[string]logs.SourceOpener{key: source.Open}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Events != 1 {
		t.Fatal(report.Events)
	}
	details, _, err := logs.Details(report, logs.Filter{Source: -1}, 0, 10)
	if err != nil || len(details) != 1 || details[0].Text != string(content) {
		t.Fatal(details, err)
	}
	var exported bytes.Buffer
	if err := logs.Export(report, logs.Filter{Source: -1, Keyword: "failed"}, &exported); err != nil || !strings.Contains(exported.String(), string(content)) {
		t.Fatal(exported.String(), err)
	}
	if _, err = os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("normal read created cache", err)
	}
	f, err := source.Open()
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || !bytes.Equal(data, content) {
		t.Fatal(string(data), err)
	}
	if _, err = os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("normal EOF created a cache", err)
	}
	// Inject a dropped read while retaining a live server for the cache reconnect.
	f, err = source.Open()
	if err != nil {
		t.Fatal(err)
	}
	resilient := f.(*resilientFile)
	resilient.file = brokenRead{resilient.file}
	data, err = io.ReadAll(f)
	f.Close()
	if err != nil || !bytes.Equal(data, content) {
		t.Fatal(string(data), err)
	}
	if source.cached == "" {
		t.Fatal("network failure did not create fallback")
	}
	server.Close()
	cachePath := source.cached
	details, _, err = logs.Details(report, logs.Filter{Source: -1, Keyword: "failed"}, 0, 10)
	if err != nil || len(details) != 1 || details[0].Text != string(content) {
		t.Fatalf("cached detail unavailable after disconnect: %v %v", details, err)
	}
	var timeExport bytes.Buffer
	if err = logs.ExportTime(context.Background(), report, 0, "2026-10-15 12:30:00", "2026-10-15 12:30:00", &timeExport); err != nil || !bytes.Equal(timeExport.Bytes(), content) {
		t.Fatalf("cached time export: %q %v", timeExport.String(), err)
	}
	if source.cached != cachePath {
		t.Fatal("cached source was downloaded again")
	}
	if _, err = logs.Analyze([]string{key}, t.TempDir(), logs.Options{Readers: map[string]logs.SourceOpener{key: source.Open}}); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), options.Password) {
		t.Fatal("credentials serialized")
	}
	var restored logs.Report
	if err = json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if _, _, err = logs.Details(&restored, logs.Filter{Source: -1}, 0, 10); err == nil || !strings.Contains(err.Error(), "reconnect") {
		t.Fatalf("restored remote report must require reconnection: %v", err)
	}
	after, _ := os.Stat(sourcePath)
	got, _ := os.ReadFile(sourcePath)
	if !bytes.Equal(got, content) || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("source changed")
	}
	for _, err := range []error{os.ErrPermission, os.ErrNotExist, context.Canceled, errors.New("authentication failed"), errors.New("file changed")} {
		if networkFailure(err) {
			t.Fatal("incorrect fallback", err)
		}
	}
}

func TestDirectSourceRejectsUnsafeFallback(t *testing.T) {
	for _, kind := range []string{"changed", "missing", "authentication", "fingerprint", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "app.log")
			if err := os.WriteFile(file, []byte("ERROR original\n"), 0600); err != nil {
				t.Fatal(err)
			}
			fixture, err := sftpfixture.Start(root)
			if err != nil {
				t.Fatal(err)
			}
			defer fixture.Close()
			host, port, _ := net.SplitHostPort(fixture.Listener.Addr().String())
			number, _ := strconv.Atoi(port)
			options := Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: fixture.Fingerprint, Path: "app.log"}
			files, _, err := Preview(context.Background(), options, nil)
			if err != nil || len(files) != 1 {
				t.Fatalf("preview: %v %v", files, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "changed":
				err = os.WriteFile(file, []byte("ERROR changed and longer\n"), 0600)
			case "missing":
				err = os.Remove(file)
			case "authentication":
				options.Password = "incorrect"
			case "fingerprint":
				options.Fingerprint = "SHA256:incorrect"
			case "canceled":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			cache := filepath.Join(t.TempDir(), "cache")
			source := &DirectSource{Context: ctx, Options: options, File: files[0], CacheDir: cache}
			opened, err := source.Open()
			if opened != nil {
				opened.Close()
			}
			if err == nil {
				t.Fatal("invalid source accepted")
			}
			if _, statErr := os.Stat(cache); !os.IsNotExist(statErr) {
				t.Fatalf("non-network failure created cache: %v", statErr)
			}
			if source.cached != "" {
				t.Fatal("failed source registered cache")
			}
		})
	}
}

func TestNetworkFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false}, {"permission", os.ErrPermission, false}, {"missing", os.ErrNotExist, false},
		{"canceled", context.Canceled, false}, {"timeout", &net.DNSError{IsTimeout: true}, true},
		{"wrapped network", fmt.Errorf("read: %w", net.ErrClosed), true}, {"truncated transport", io.ErrUnexpectedEOF, true},
		{"SFTP disconnected", &sftp.StatusError{Code: 6}, true}, {"SFTP connection lost", &sftp.StatusError{Code: 7}, true},
		{"SFTP permission", &sftp.StatusError{Code: 3}, false}, {"SFTP missing", &sftp.StatusError{Code: 2}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := networkFailure(tc.err); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

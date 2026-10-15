package server

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"log-reviewer/internal/remote"
	"log-reviewer/internal/testsupport/sftpfixture"
)

func TestRepeatedAttachKeepsRelocatableReportReader(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.log"), []byte("2026-10-15 12:30:00 ERROR app : repeat attach\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixture, err := sftpfixture.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	host, port, _ := net.SplitHostPort(fixture.Listener.Addr().String())
	number, _ := strconv.Atoi(port)
	options := remote.Options{Host: host, Port: number, User: "tester", Password: "fixture-password", Fingerprint: fixture.Fingerprint, Path: "app.log"}
	files, _, err := remote.Preview(context.Background(), options, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := New(t.TempDir())
	defer s.Close()
	s.remoteDir = filepath.Join(t.TempDir(), "cache")
	attach := func(id string) {
		t.Helper()
		s.remoteState = remoteState{}
		ctx, cancel := context.WithCancel(context.Background())
		s.jobs.Add(1)
		s.downloadRemote(ctx, cancel, id, options, "attach", files)
		if s.remoteState.Error != "" {
			t.Fatal(s.remoteState.Error)
		}
	}
	attach("first")
	id := s.remoteState.Downloads[0].ID
	key := s.uploads[id]
	original := s.directSources[key]
	if err := s.Start([]string{key}, false, nil); err != nil {
		t.Fatal(err)
	}
	s.jobs.Wait()
	if s.Report == nil {
		t.Fatal(s.err)
	}
	attach("second")
	if s.directSources[key] != original {
		t.Fatal("repeated attach detached the report reader from cache relocation")
	}
}

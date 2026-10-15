package logs

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathFormatsReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "日志 [01] #.log")
	content := []byte("2026-10-15 12:30:00 ERROR app : failed\n")
	if err := os.WriteFile(path, content, 0400); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	uri := (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}).String()
	quoted, _ := json.Marshal(path)
	for _, input := range []string{path, "\"" + path + "\"", "'" + path + "'", string(quoted), uri, strings.Replace(uri, "file:///", "file://localhost/", 1)} {
		paths, err := ExpandPaths([]string{input})
		if err != nil || len(paths) != 1 || paths[0] != path {
			t.Fatalf("%q: %v %v", input, paths, err)
		}
		report, err := Analyze(paths, t.TempDir(), Options{})
		if err != nil || report.Events != 1 {
			t.Fatalf("%q: %v", input, err)
		}
	}
	for _, input := range []string{"https://example.org/a.log", "file:///a.log?x=1", "file:///a.log#x", "file:///a%00.log"} {
		if _, err := ExpandPaths([]string{input}); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	after, _ := os.Stat(path)
	got, _ := os.ReadFile(path)
	if string(got) != string(content) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("source changed")
	}
}

func TestNormalizePathCompatibility(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"relative", " ./logs/app.log ", "./logs/app.log"},
		{"single quotes", "'./logs/app.log'", "./logs/app.log"},
		{"JSON escapes", `"./logs/a\u0020b.log"`, "./logs/a b.log"},
		{"encoded URI", "file:///logs/a%20b%23%25.log", filepath.FromSlash("/logs/a b#%.log")},
		{"opaque URI", "file:logs/a%20b.log", filepath.FromSlash("logs/a b.log")},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, []struct{ name, input, want string }{
			{"drive URI", "file:///C:/logs/app.log", `C:\logs\app.log`},
			{"drive authority", "file://C:/logs/app.log", `C:\logs\app.log`},
			{"localhost URI", "file://localhost/C:/logs/app.log", `C:\logs\app.log`},
			{"UNC URI", "file://server/share/app.log", `\\server\share\app.log`},
			{"UNC literal", `\\server\share\app.log`, `\\server\share\app.log`},
		}...)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizePath(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, input := range []string{"file:///a%ZZ.log", "file://user:password@localhost/a.log", "file:///a%00.log", "\x00", "sftp://host/a.log"} {
		t.Run("reject "+input, func(t *testing.T) {
			if _, err := normalizePath(input); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	if _, err := openRead("sftp:disconnected", nil); err == nil || !strings.Contains(err.Error(), "reconnect") {
		t.Fatalf("missing reconnection guidance: %v", err)
	}
}

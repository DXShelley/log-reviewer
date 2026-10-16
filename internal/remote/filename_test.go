package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeFilename(t *testing.T) {
	for _, tc := range []struct{ name, system, want string }{
		{"app.log", "windows", "app.log"},
		{"app:2026?*.text", "windows", "app_2026__.text"},
		{"CON.log", "windows", "_CON.log"},
		{"LPT1.txt", "windows", "_LPT1.txt"},
		{"COM¹.log", "windows", "_COM¹.log"},
		{"app.log. ", "windows", "app.log"},
		{"app:2026?.text", "linux", "app:2026?.text"},
		{".", "linux", "_log"},
	} {
		if got := safeFilename(tc.name, tc.system); got != tc.want {
			t.Errorf("%q on %s: %q, want %q", tc.name, tc.system, got, tc.want)
		}
	}
}

func TestDownloadFilenameCollisions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"app.log", "app (1).log", "manifest.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, want string }{{"app.log", "app (2).log"}, {"manifest.json", "manifest (1).json"}, {"trace.text", "trace.text"}} {
		got, err := downloadFilename(dir, tc.name)
		if err != nil || filepath.Base(got) != tc.want {
			t.Fatalf("%s: %s %v", tc.name, got, err)
		}
	}
}

func TestValidLongFilenamePreserved(t *testing.T) {
	name := strings.Repeat("a", 200) + ".log"
	if got := safeFilename(name, "linux"); got != name {
		t.Fatal("valid filename changed")
	}
	for _, system := range []string{"windows", "linux"} {
		got := safeFilename(strings.Repeat("a", 260)+".text", system)
		if filenameUnits(got, system) > 255 || !strings.HasSuffix(got, ".text") {
			t.Fatal(system, got)
		}
	}
	dir := t.TempDir()
	name = strings.Repeat("a", 251) + ".log"
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := downloadFilename(dir, name)
	if err != nil || filepath.Base(got) == name || len(filepath.Base(got)) > 255 || !strings.HasSuffix(got, " (1).log") {
		t.Fatal(got, err)
	}
}

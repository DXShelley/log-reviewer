package logs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPythonTracebackAttached(t *testing.T) {
	p := testFile(t, "python.log", "2026-10-14 12:30:00,123 ERROR worker - task failed\nTraceback (most recent call last):\n  File \"worker.py\", line 42\nTimeoutError: failed\n2026-10-14 12:31:00 INFO worker - recovered\n")
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 1 || r.Groups[0].UnknownTime != 0 || !strings.Contains(r.Groups[0].Title, "TimeoutError") {
		t.Fatalf("traceback split %+v", r)
	}
	d, _, err := Details(r, Filter{Source: -1}, 0, 10)
	if err != nil || !strings.Contains(d[0].Text, "Traceback") {
		t.Fatal("missing traceback", err)
	}
}
func TestEpochPrecision(t *testing.T) {
	p, _ := NewParser(nil)
	for _, s := range []string{"1791952200123", "1791952200.123", "1791952200123000", "1791952200123000000"} {
		ns, ok := p.parseTime(s, "")
		if !ok || time.Unix(0, ns).Nanosecond() != 123000000 {
			t.Fatalf("%s => %d", s, ns)
		}
	}
}
func TestUnitNormalization(t *testing.T) {
	a, _ := signature("Slow request order=1002 latency=1200ms")
	b, _ := signature("Slow request order=1004 latency=1600ms")
	if a != b {
		t.Fatalf("unit normalization %s != %s", a, b)
	}
}
func TestUnsupportedEncoding(t *testing.T) {
	p := testFile(t, "utf16.log", string([]byte{0xff, 0xfe, 'W', 0}))
	if _, err := Analyze([]string{p}, t.TempDir(), Options{}); err == nil {
		t.Fatal("UTF16 was silently accepted")
	}
}
func TestSafeOutput(t *testing.T) {
	p := testFile(t, "app.log", spring)
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveReport(r, p); err == nil {
		t.Fatal("report overwrote source")
	}
	b, _ := os.ReadFile(p)
	if string(b) != spring {
		t.Fatal("source modified")
	}
	alias := filepath.Join(filepath.Dir(p), "alias.log")
	if err := os.Link(p, alias); err == nil {
		if err := SafeOutput(r, alias); err == nil {
			t.Fatal("hardlink accepted")
		}
	}
}
func TestPhysicalLineRegexAndKeywordBoundary(t *testing.T) {
	text := "ERROR " + strings.Repeat("x", 65525) + "BOUNDARY-NEEDLE" + strings.Repeat("y", 70000) + "\nWARN final\n"
	p := testFile(t, "long.log", text)
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := Summarize(r, Filter{Source: -1, Keyword: "boundary-needle"})
	if err != nil || len(g) != 1 {
		t.Fatal("keyword missed chunk boundary", err)
	}
	g, err = Summarize(r, Filter{Source: -1, Regex: `^BOUNDARY`})
	if err != nil || len(g) != 0 {
		t.Fatal("regex matched a fragment rather than physical line", err)
	}
	g, err = Summarize(r, Filter{Source: -1, Regex: `^WARN final$`})
	if err != nil || len(g) != 1 {
		t.Fatal("line anchors failed", err)
	}
}
func TestCorruptIndex(t *testing.T) {
	p := testFile(t, "app.log", spring)
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, RecordSize)
	Record{Group: r.Groups[0].Number, Source: 99, Length: 1}.Encode(buf)
	os.WriteFile(r.IndexPath, buf, 0600)
	var out bytes.Buffer
	if err = Export(r, Filter{Source: -1}, &out); err == nil {
		t.Fatal("corrupt source span accepted")
	}
}
func TestGlogAndDotnetGrouping(t *testing.T) {
	text := "E1014 12:30:00.123456 1234 main.cc:42] failed request 123\nE1014 12:31:00.123456 5678 main.cc:44] failed request 456\nfail: Microsoft.Hosting[1]\n  Connection refused\nfail: Microsoft.Hosting[1]\n  Invalid configuration\n"
	p := testFile(t, "app.log", text)
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 4 || len(r.Groups) != 3 || r.Groups[0].Count != 2 || r.Groups[0].Loggers[0] != "main.cc" {
		t.Fatalf("glog or dotnet grouping failed %+v", r)
	}
	for _, g := range r.Groups {
		if g.Title == "(empty message)" {
			t.Fatal("dotnet body lost")
		}
	}
}

package logs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testFile(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

const spring = "2026-10-14 12:30:00.123 ERROR 123 --- [main] com.demo.OrderService : request 123 failed\njava.lang.RuntimeException: failed\n\tat com.demo.OrderService.run(OrderService.java:42)\nCaused by: java.sql.SQLException: Connection refused\n\tat com.demo.DB.open(DB.java:19)\n"

func TestGroupingStackAndContext(t *testing.T) {
	p := testFile(t, "app.log", "2026-10-14 12:29:00 INFO demo : before\n"+spring+strings.Replace(spring, "12:30:00", "12:31:00", 1)+"2026-10-14 12:32:00 WARN demo : slow request 456\n2026-10-14 12:33:00 INFO demo : after\n")
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 3 || len(r.Groups) != 2 || r.Lines != 13 {
		t.Fatalf("events=%d groups=%d lines=%d", r.Events, len(r.Groups), r.Lines)
	}
	g := r.Groups[0]
	if g.Count != 2 || !strings.Contains(g.Title, "SQLException") || g.First == g.Last || g.Loggers[0] != "com.demo.OrderService" {
		t.Fatalf("group: %+v", g)
	}
	d, more, err := Details(r, Filter{Group: g.ID, Source: -1}, 0, 1)
	if err != nil || !more || len(d) != 1 {
		t.Fatalf("details %+v %v %v", d, more, err)
	}
	if !strings.Contains(d[0].Text, "DB.java:19") || !strings.Contains(d[0].Before, "before") || d[0].Line != 2 {
		t.Fatalf("incomplete detail: %+v", d[0])
	}
	var out bytes.Buffer
	if err := Export(r, Filter{Group: g.ID, Source: -1}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "DB.java:19") != 2 {
		t.Fatal("export missing occurrences")
	}
}
func TestFilters(t *testing.T) {
	p := testFile(t, "app.log", spring+strings.Replace(spring, "12:30:00", "12:31:00", 1)+"2026-10-14 12:32:00 WARN demo : slow request 456\n")
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		f Filter
		n uint64
	}{{Filter{Source: -1, Keyword: "connection REFUSED"}, 2}, {Filter{Source: -1, Regex: `DB\.java:19`}, 2}, {Filter{Source: -1, From: "2026-10-14T12:31:00", To: "2026-10-14T12:31:59"}, 1}, {Filter{Source: -1, Level: "WARN"}, 1}, {Filter{Source: -1, Keyword: "no-such-message"}, 0}}
	for _, tt := range tests {
		g, err := Summarize(r, tt.f)
		if err != nil {
			t.Fatal(err)
		}
		var n uint64
		for _, v := range g {
			n += v.Count
		}
		if n != tt.n {
			t.Fatalf("filter %+v: %d expected %d", tt.f, n, tt.n)
		}
	}
	if _, err := Summarize(r, Filter{Source: -1, Regex: "["}); err == nil {
		t.Fatal("invalid regex accepted")
	}
	if _, err := Summarize(r, Filter{Source: -1, From: "2026-10-15", To: "2026-10-14"}); err == nil {
		t.Fatal("reversed range accepted")
	}
}
func TestMultipleFilesAndPersistence(t *testing.T) {
	p := testFile(t, "a.log", spring)
	q := testFile(t, "b.log", spring)
	r, err := Analyze([]string{p, q, p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sources) != 2 || len(r.Groups) != 1 || r.Groups[0].Count != 2 || len(r.Groups[0].Sources) != 2 {
		t.Fatalf("bad multifile %+v", r)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := SaveReport(r, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadReport(path)
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := Details(loaded, Filter{Source: 1}, 0, 10)
	if err != nil || len(d) != 1 {
		t.Fatalf("persisted details %v %v", d, err)
	}
	os.WriteFile(q, []byte("changed"), 0600)
	if _, _, err = Details(loaded, Filter{Source: 1}, 0, 10); err == nil {
		t.Fatal("changed file accepted")
	}
}
func TestFormats(t *testing.T) {
	tests := []struct {
		line, level, format string
		timed               bool
	}{
		{"2026-10-14 12:30:00,123 WARNING root - slow request", "WARN", "logging/log4j", true},
		{`{"level":50,"time":1791971400123,"msg":"failed","name":"node"}`, "ERROR", "json", true},
		{`{"level":"error","ts":1791971400.123,"msg":"failed"}`, "ERROR", "json", true},
		{`{"@t":"2026-10-14T12:30:00Z","@l":"Error","@mt":"failed","@x":"System.Exception: bad"}`, "ERROR", "json", true},
		{"2026-10-14T12:30:00Z ERROR server: request failed", "ERROR", "text", true},
		{"2026/10/14 12:30:00 [error] 123#0: connection refused", "ERROR", "text", true},
		{"fail: Microsoft.Hosting[1]", "ERROR", "dotnet", false},
		{"E1014 12:30:00.123456 1234 main.cc:42] failed request", "ERROR", "glog", false},
		{"Traceback (most recent call last):", "ERROR", "python/traceback", false},
		{"panic: connection failed", "FATAL", "go/panic", false},
		{"\xef\xbb\xbf2026-10-14 12:30:00 ERROR request failed", "ERROR", "text", true},
		{"\x1b[31m2026-10-14 12:30:00 ERROR\x1b[0m request failed", "ERROR", "text", true},
	}
	for _, tt := range tests {
		p, _ := NewParser(nil)
		h, ok := p.Parse([]byte(tt.line), false)
		if !ok || h.Level != tt.level || h.Format != tt.format || h.HasTime != tt.timed {
			t.Errorf("%s => %+v, %v", tt.line, h, ok)
		}
	}
}
func TestCustomRule(t *testing.T) {
	rule := Rule{Name: "pipe", Pattern: `^(?P<time>[^|]+)\|(?P<level>[^|]+)\|(?P<logger>[^|]+)\|(?P<message>.*)$`, TimeLayout: "2006/01/02 15:04:05"}
	p, err := NewParser([]Rule{rule})
	if err != nil {
		t.Fatal(err)
	}
	h, ok := p.Parse([]byte("2026/10/14 12:30:00|WARN|worker|failed 123"), false)
	if !ok || h.Format != "pipe" || !h.HasTime || h.Logger != "worker" {
		t.Fatalf("bad custom %+v", h)
	}
	if _, err := NewParser([]Rule{{Name: "bad", Pattern: "(.*)"}}); err == nil {
		t.Fatal("invalid rule accepted")
	}
}
func TestLargeEventAndLine(t *testing.T) {
	text := "2026-10-14 12:30:00 ERROR huge message " + strings.Repeat("x", 3*1024*1024) + "\nCaused by: java.io.IOException: deep cause\n2026-10-14 12:31:00 INFO finished\n"
	p := testFile(t, "big.log", text)
	r, err := Analyze([]string{p}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 1 || r.Lines != 3 || !strings.Contains(r.Groups[0].Title, "IOException") {
		t.Fatalf("large event failed %+v", r)
	}
	d, _, err := Details(r, Filter{Source: -1}, 0, 10)
	if err != nil || !d[0].Truncated {
		t.Fatal("large detail should be bounded", err)
	}
	var b bytes.Buffer
	if err := Export(r, Filter{Source: -1}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "deep cause") || b.Len() < 3*1024*1024 {
		t.Fatal("large export truncated")
	}
}
func TestEmptyAndNoTrailingNewline(t *testing.T) {
	for _, text := range []string{"", "INFO normal\n", "WARN final"} {
		p := testFile(t, "a.log", text)
		r, err := Analyze([]string{p}, t.TempDir(), Options{})
		if err != nil {
			t.Fatal(err)
		}
		expected := uint64(strings.Count(text, "\n"))
		if text != "" && !strings.HasSuffix(text, "\n") {
			expected++
		}
		if r.Lines != expected {
			t.Fatalf("%q lines %d != %d", text, r.Lines, expected)
		}
	}
}
func TestCancelAndAll(t *testing.T) {
	p := testFile(t, "a.log", "INFO normal\nDEBUG useful\nWARN danger\n")
	r, err := Analyze([]string{p}, t.TempDir(), Options{All: true})
	if err != nil || r.Events != 3 {
		t.Fatal("all levels failed", err)
	}
	progress := &Progress{}
	progress.Cancel.Store(true)
	if _, err := Analyze([]string{p}, t.TempDir(), Options{Progress: progress}); err == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestNoStackFrameFalsePositive(t *testing.T) {
	p, _ := NewParser(nil)
	for _, line := range []string{"  at com.error.Service.run(Service.java:42)", "Caused by: java.lang.Error: failed", "  File \"error.py\", line 42", "\tat INFO.handler(file.go:19)"} {
		if _, ok := p.Parse([]byte(line), false); ok {
			t.Fatalf("stack split by %q", line)
		}
	}
}

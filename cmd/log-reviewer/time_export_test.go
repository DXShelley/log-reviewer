package main

import (
	"log-reviewer/internal/logs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportTimeCLI(t *testing.T) {
	dir := t.TempDir()
	sources := []string{}
	for _, name := range []string{"a.log", "b.log"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("2026-10-14 12:30:00 INFO app : "+name), 0600); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, p)
	}
	r, err := logs.Analyze(sources, dir, logs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dir, "report.json")
	if err = logs.SaveReport(r, report); err != nil {
		t.Fatal(err)
	}
	args := []string{"--report", report, "--from", "2026-10-14T12:30:00", "--to", "2026-10-14T12:30:00"}
	out := filepath.Join(dir, "merged.txt")
	if err = exportTime(append(args, "--out", out)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), "a.log\n2026") {
		t.Fatalf("merged boundary: %s %v", b, err)
	}
	if exportTime(append(args, "--out", out)) == nil {
		t.Fatal("overwrote existing output")
	}
	split := filepath.Join(dir, "split")
	if err = exportTime(append(args, "--out-dir", split)); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(split)
	if err != nil || len(files) != 2 {
		t.Fatalf("split: %v %v", files, err)
	}
	for _, f := range files {
		b, err = os.ReadFile(filepath.Join(split, f.Name()))
		if err != nil || strings.HasSuffix(string(b), "\n") {
			t.Fatal("split did not preserve original bytes")
		}
	}
	if exportTime(append(args, "--out-dir", split)) == nil {
		t.Fatal("accepted existing directory")
	}
}

package logs

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestConfigurableKeywordContext(t *testing.T) {
	var text strings.Builder
	for i := 1; i <= 160; i++ {
		if i == 30 {
			fmt.Fprintln(&text, "ERROR failure")
		} else if i == 33 {
			fmt.Fprintln(&text, "needle here")
		} else if i == 40 {
			fmt.Fprintln(&text, "INFO end")
		} else {
			fmt.Fprintf(&text, "line-%03d %s\n", i, strings.Repeat("x", 200))
		}
	}
	path := testFile(t, "context.log", text.String())
	dir := t.TempDir()
	report, err := Analyze([]string{path}, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	details, _, err := Details(report, Filter{Keyword: "needle", Source: -1}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	d := details[0]
	if d.ContextLine != 33 || len(strings.Split(d.Before, "\n")) != 20 || len(strings.Split(d.After, "\n")) != 100 || !strings.Contains(d.Before, "line-013") || !strings.Contains(d.After, "line-133") {
		t.Fatalf("default context line=%d before=%d after=%d", d.ContextLine, len(strings.Split(d.Before, "\n")), len(strings.Split(d.After, "\n")))
	}
	details, _, err = Details(report, Filter{Regex: "needle", Source: -1}, 0, 1, ContextOptions{Before: 2, After: 3})
	if err != nil {
		t.Fatal(err)
	}
	if details[0].ContextLine != 33 || len(strings.Split(details[0].Before, "\n")) != 2 || len(strings.Split(details[0].After, "\n")) != 3 {
		t.Fatal(details[0])
	}
	details, _, err = Details(report, Filter{Source: -1}, 0, 1, ContextOptions{Before: 0, After: 0})
	if err != nil || details[0].Before != "" || details[0].After != "" {
		t.Fatal(details, err)
	}
	_, _, err = Details(report, Filter{Source: -1}, 0, 1, ContextOptions{Before: -1, After: 0})
	if err == nil {
		t.Fatal("negative context accepted")
	}
}

func TestContextBoundariesAndBlankLines(t *testing.T) {
	path := testFile(t, "blank.log", "a\n\nb\nlast")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	before, truncated, err := readContext(f, 5, 3, true)
	if err != nil || truncated || before != "a\n\nb" {
		t.Fatalf("before %q %v %v", before, truncated, err)
	}
	after, truncated, err := readContext(f, 5, 100, false)
	if err != nil || truncated || after != "last" {
		t.Fatalf("after %q %v %v", after, truncated, err)
	}
}

func TestContextAlignedReadAndHugeKeywordLine(t *testing.T) {
	text := strings.Repeat("x", 64*1024-1) + "\n" + strings.Repeat("y", 64*1024-1) + "\n"
	path := testFile(t, "aligned.log", text)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, truncated, err := readContext(f, 0, 1, false)
	if err != nil || truncated || got != strings.Repeat("x", 64*1024-1) {
		t.Fatal("aligned forward", len(got), truncated, err)
	}
	got, truncated, err = readContext(f, int64(len(text)), 1, true)
	if err != nil || truncated || got != strings.Repeat("y", 64*1024-1) {
		t.Fatal("aligned backward", len(got), truncated, err)
	}
	huge := strings.Repeat("z", 3*1024*1024) + "needle\nnext\n"
	path = testFile(t, "huge.log", huge)
	big, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer big.Close()
	start, end, line, preview, truncated, found, err := findContextAnchor(big, Record{Length: uint64(len(huge)), Line: 1}, "needle", nil)
	if err != nil || !found || !truncated || line != 1 || start != 0 || end != int64(len(huge)-5) || len(preview) != 2*1024*1024 {
		t.Fatal("huge keyword", start, end, line, len(preview), truncated, found, err)
	}
}

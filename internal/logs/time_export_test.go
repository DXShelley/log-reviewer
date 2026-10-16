package logs

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestExportTimeRawAndSummary(t *testing.T) {
	input := "untimed prefix\n2026-10-14 12:29:00 INFO demo : excluded\n" + spring + "2026-10-14 12:30:00.999 INFO demo : included\ncontinuation\n2026-10-14 12:31:00 INFO demo : excluded\n"
	path := testFile(t, "node.log", input)
	r, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	err = ExportTime(context.Background(), r, 0, "2026-10-14T12:30:00", "2026-10-14T12:30:00", &b)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != spring+"2026-10-14 12:30:00.999 INFO demo : included\ncontinuation\n" {
		t.Fatalf("unexpected raw export: %s", b.String())
	}
	d, _, err := Details(r, Filter{Source: -1}, 0, 10, ContextOptions{Summary: true})
	if err != nil || len(d) != 1 || d[0].Text != "" || d[0].Before != "" || d[0].Line != 3 {
		t.Fatalf("summary: %+v %v", d, err)
	}
	for _, pair := range [][2]string{{"", ""}, {"2026-10-15", "2026-10-14"}, {"invalid", "2026-10-14"}} {
		if ExportTime(context.Background(), r, 0, pair[0], pair[1], &b) == nil {
			t.Fatal("accepted invalid range")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ExportTime(ctx, r, 0, "2026-10-14", "2026-10-14", &b) == nil {
		t.Fatal("ignored cancellation")
	}
	if err = os.WriteFile(path, []byte(input+"changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if ExportTime(context.Background(), r, 0, "2026-10-14", "2026-10-14", &b) == nil {
		t.Fatal("accepted changed source")
	}
}

func TestExportTimeLongLineAndUnsorted(t *testing.T) {
	input := "2026-10-14 12:31:00 INFO demo : later\n2026-10-14 12:30:00 INFO demo : " + strings.Repeat("x", 200000) + "\n2026-10-14 12:29:00 INFO demo : earlier"
	path := testFile(t, "large.log", input)
	r, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	err = ExportTime(context.Background(), r, 0, "2026-10-14T12:30:00", "2026-10-14T12:30:00", &b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), strings.Repeat("x", 200000)) || strings.Contains(b.String(), "later") || strings.Contains(b.String(), "earlier") {
		t.Fatal("incorrect long or unsorted line")
	}
}

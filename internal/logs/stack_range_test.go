package logs

import (
	"strings"
	"testing"
)

func TestCallStacksSeparateSameRoot(t *testing.T) {
	other := strings.Replace(spring, "OrderService.run(OrderService.java:42)", "OtherService.run(OtherService.java:42)", 1)
	same := strings.ReplaceAll(spring, "\tat ", "    at   ")
	path := testFile(t, "stack.log", spring+other+same)
	report, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Groups) != 2 || report.Groups[0].Count != 2 || report.Groups[1].Count != 1 {
		t.Fatalf("different callers merged or whitespace split: %+v", report.Groups)
	}
	if report.Groups[0].Title != report.Groups[1].Title || report.Groups[0].ID == report.Groups[1].ID {
		t.Fatal("same root must have distinct stack category IDs")
	}
}

func TestStackBeyondPreviewSeparates(t *testing.T) {
	long := spring + strings.Repeat(" at demo.Service.run(Service.java:40)\n", 3000)
	a := long + " at demo.First.run(First.java:12)\n"
	b := long + " at demo.Second.run(Second.java:12)\n"
	path := testFile(t, "long-stack.log", a+b)
	report, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Groups) != 2 {
		t.Fatalf("stack tail beyond preview was ignored: %d categories", len(report.Groups))
	}
}

func TestEventRangeAndUniqueLocator(t *testing.T) {
	for _, ending := range []string{"", "\n", "\r\n"} {
		t.Run("ending="+ending, func(t *testing.T) {
			text := "2026-10-14 12:30:00 ERROR app : failed\r\n at demo.Run(Main.java:42)\r\n2026-10-14 12:30:03 still processing" + ending
			path := testFile(t, "range.log", text)
			report, err := Analyze([]string{path}, t.TempDir(), Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, summary := range []bool{true, false} {
				details, _, err := Details(report, Filter{Source: -1}, 0, 10, ContextOptions{Summary: summary})
				if err != nil || len(details) != 1 {
					t.Fatalf("details: %+v %v", details, err)
				}
				d := details[0]
				if d.Line != 1 || d.EndLine != 3 || d.EndOffset != uint64(len(text)) || d.EventID == "" || d.Time == d.EndTime || !strings.Contains(d.EndTime, "12:30:03") {
					t.Fatalf("incorrect event bounds: %+v", d)
				}
			}
		})
	}
	a := testFile(t, "a.log", spring+spring)
	b := testFile(t, "b.log", spring)
	report, err := Analyze([]string{a, b}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	details, _, err := Details(report, Filter{Source: -1}, 0, 10, ContextOptions{Summary: true})
	if err != nil || len(details) != 3 {
		t.Fatalf("details: %+v %v", details, err)
	}
	seen := map[string]bool{}
	for _, detail := range details {
		if seen[detail.EventID] {
			t.Fatal("event locator collides across files or offsets")
		}
		seen[detail.EventID] = true
	}
}

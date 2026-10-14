package logs

import (
	"strings"
	"testing"
)

func TestAutomaticContextBoundaries(t *testing.T) {
	event := "2026-10-14 12:30:00 ERROR app : requestId=abc connection timeout\njava.lang.RuntimeException: failed\n\tat app.run(Main.java:42)\n"
	before := "2026-10-14 12:28:00 INFO app : distant\n2026-10-14 12:29:57 ERROR app : requestId=other unrelated\n\tat other.unrelated(Other.java:1)\n2026-10-14 12:29:59 INFO app : requestId=abc connecting"
	after := "2026-10-14 12:30:01 INFO app : requestId=abc retry\n2026-10-14 12:30:02 INFO app : requestId=abc recovered\n2026-10-14 12:30:03 INFO app : requestId=other unrelated"
	stamp, _ := ParseBound("2026-10-14T12:30:00")
	b, a, reason := automaticContext(event, before, after, stamp)
	if strings.Contains(b, "unrelated") || strings.Contains(b, "distant") || !strings.Contains(b, "connecting") || strings.Contains(a, "unrelated") || !strings.Contains(a, "recovered") || !strings.Contains(reason, "请求 ID") {
		t.Fatalf("before=%s after=%s reason=%s", b, a, reason)
	}
}

func TestAutomaticDetailAndManualOverride(t *testing.T) {
	path := testFile(t, "auto.log", "2026-10-14 12:29:00 INFO app : old\n2026-10-14 12:29:59 INFO app : requestId=abc preparation\n2026-10-14 12:30:00 ERROR app : requestId=abc failed\njava.lang.RuntimeException: broken\n\tat app.run(Main.java:42)\n2026-10-14 12:30:01 INFO app : requestId=other unrelated\n")
	report, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	auto, _, err := Details(report, Filter{Source: -1}, 0, 1, ContextOptions{Before: 20, After: 100, Auto: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(auto) != 1 || auto[0].ContextMode != "auto" || auto[0].ContextBefore != 1 || auto[0].ContextAfter != 0 || !strings.Contains(auto[0].Text, "Main.java:42") {
		t.Fatalf("auto: %+v", auto)
	}
	manual, _, err := Details(report, Filter{Source: -1}, 0, 1, ContextOptions{Before: 20, After: 100})
	if err != nil {
		t.Fatal(err)
	}
	if manual[0].ContextMode != "manual" || !strings.Contains(manual[0].Before, "old") || !strings.Contains(manual[0].After, "unrelated") {
		t.Fatalf("manual: %+v", manual)
	}
	zero, _, err := Details(report, Filter{Source: -1}, 0, 1, ContextOptions{Auto: true})
	if err != nil || zero[0].ContextBefore != 0 || zero[0].ContextAfter != 0 {
		t.Fatalf("zero: %+v %v", zero, err)
	}
}

func TestAutomaticContextDoesNotCutMatchedStack(t *testing.T) {
	path := testFile(t, "stack.log", spring+"2026-10-14 12:30:01 INFO app : unrelated\n")
	report, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := Details(report, Filter{Source: -1, Keyword: "SQLException"}, 0, 1, ContextOptions{Before: 20, After: 100, Auto: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d[0].Text, "OrderService.java:42") || !strings.Contains(d[0].Text, "DB.java:19") || d[0].ContextLine == 0 {
		t.Fatalf("incomplete stack %+v", d)
	}
}

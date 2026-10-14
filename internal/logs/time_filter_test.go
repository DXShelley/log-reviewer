package logs

import (
	"testing"
	"time"
)

func TestInclusiveTimePrecision(t *testing.T) {
	cases := []struct{ upper, last string }{
		{"2026-10-14T12:30:00+08:00", "2026-10-14T12:30:00.999999999+08:00"},
		{"2026-10-14T12:30:00Z", "2026-10-14T12:30:00.999999999Z"},
		{"2026-10-14T12:30", "2026-10-14T12:30:59.999999999"},
		{"2026-10-14", "2026-10-14T23:59:59.999999999"},
		{"2026-10-14T12:30:00.500Z", "2026-10-14T12:30:00.500Z"},
	}
	for _, c := range cases {
		got, err := ParseUpperBound(c.upper)
		want, e := ParseBound(c.last)
		if err != nil || e != nil || got != want {
			t.Fatalf("%s got %d want %d errors %v %v", c.upper, got, want, err, e)
		}
	}
	path := testFile(t, "bounds.log", "2026-10-14T04:30:00.123Z ERROR early\n2026-10-14T04:30:00.999999999Z ERROR included\n2026-10-14T04:30:01Z ERROR outside\n")
	r, err := Analyze([]string{path}, t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := Summarize(r, Filter{Source: -1, From: "2026-10-14T12:30:00+08:00", To: "2026-10-14T12:30:00+08:00"})
	if err != nil || len(groups) != 2 {
		t.Fatal(len(groups), err)
	}
}

func TestDayUpperBoundDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	old := time.Local
	time.Local = location
	defer func() { time.Local = old }()
	got, err := ParseUpperBound("2026-03-08")
	want, _ := ParseBound("2026-03-08T23:59:59.999999999")
	if err != nil || got != want {
		t.Fatal(got, want, err)
	}
}

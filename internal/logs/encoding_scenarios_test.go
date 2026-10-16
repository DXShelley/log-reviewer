package logs

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEncodingLineEndingsAndMixedSources(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", ""} {
		t.Run(strings.NewReplacer("\r", "CR", "\n", "LF").Replace(ending), func(t *testing.T) {
			prefix := "2026-10-16 12:00:00 ERROR demo : "
			expected := prefix + "导出结束" + ending
			rawGBK := prefix + string([]byte{0xb5, 0xbc, 0xb3, 0xf6, 0xbd, 0xe1, 0xca, 0xf8}) + ending
			inputs := []string{expected, "\xef\xbb\xbf" + expected, rawGBK}
			var paths []string
			for i, raw := range inputs {
				paths = append(paths, testFile(t, []string{"utf8.log", "bom.log", "gbk.log"}[i], raw))
			}
			r, err := Analyze(paths, t.TempDir(), Options{})
			if err != nil {
				t.Fatal(err)
			}
			details, _, err := Details(r, Filter{Source: -1, Keyword: "导出结束"}, 0, 10)
			if err != nil || len(details) != 3 {
				t.Fatalf("details=%+v err=%v", details, err)
			}
			for _, d := range details {
				if d.Text != expected {
					t.Fatalf("text=%q want=%q", d.Text, expected)
				}
			}
			for i, source := range r.Sources {
				var out bytes.Buffer
				if err := ExportTime(context.Background(), r, i, "2026-10-16T12:00:00", "2026-10-16T12:00:00", &out); err != nil {
					t.Fatal(err)
				}
				if !utf8.Valid(out.Bytes()) || strings.TrimPrefix(out.String(), "\ufeff") != expected {
					t.Fatalf("%s: %q", source.Path, out.String())
				}
			}
			var out bytes.Buffer
			if err := Export(r, Filter{Source: -1, Keyword: "导出结束"}, &out); err != nil {
				t.Fatal(err)
			}
			if !utf8.Valid(out.Bytes()) || strings.Count(out.String(), "导出结束") != 3 {
				t.Fatalf("export=%q", out.String())
			}
			for i, path := range paths {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != inputs[i] {
					t.Fatalf("source modified: %s %v", path, err)
				}
			}
		})
	}
}

func TestUnsupportedEncodingSignatures(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"utf16-le", []byte{0xff, 0xfe, 'E', 0}},
		{"utf16-be", []byte{0xfe, 0xff, 0, 'E'}},
		{"gzip", []byte{0x1f, 0x8b, 0x08, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := testFile(t, tc.name+".log", string(tc.raw))
			_, err := Analyze([]string{path}, t.TempDir(), Options{})
			if err == nil || !strings.Contains(err.Error(), "convert to UTF-8 or decompress first") {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}

func TestReportRejectsLegacyJSONEncoding(t *testing.T) {
	path := testFile(t, "report.json", `{"schema":"log-reviewer/v1","notes":["`+string([]byte{0xb5, 0xbc, 0xb3, 0xf6})+`"]}`)
	if _, err := LoadReport(path); err == nil || !strings.Contains(err.Error(), "JSON must be UTF-8") {
		t.Fatalf("invalid JSON encoding accepted: %v", err)
	}
}

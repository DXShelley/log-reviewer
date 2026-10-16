package logs

import (
	"bytes"
	"context"
	"golang.org/x/text/encoding/simplifiedchinese"
	"os"
	"strings"
	"testing"
)

func TestLegacyChineseLog(t *testing.T) {
	// GBK bytes for Chinese text, independent of the decoder under test.
	message := string([]byte{0xb5, 0xbc, 0xb3, 0xf6, 0xbd, 0xe1, 0xca, 0xf8})
	raw := "2026-10-16 12:00:00 INFO demo : " + message + "\n" +
		"2026-10-16 12:00:01 ERROR demo : " + message + "\n" +
		"2026-10-16 12:00:02 INFO demo : " + message + "\n"
	for _, remote := range []bool{false, true} {
		name := testFile(t, "export.log", raw)
		options := Options{}
		if remote {
			options.Readers = map[string]SourceOpener{name: func() (ReadFile, error) { return os.Open(name) }}
		}
		r, err := Analyze([]string{name}, t.TempDir(), options)
		if err != nil {
			t.Fatal(err)
		}
		for _, filter := range []Filter{{Source: -1}, {Source: -1, Keyword: "导出结束"}, {Source: -1, Regex: "导出.*束"}} {
			details, _, err := Details(r, filter, 0, 10, ContextOptions{Before: 1, After: 1})
			if err != nil || len(details) != 1 {
				t.Fatalf("details=%+v err=%v", details, err)
			}
			d := details[0]
			for _, text := range []string{d.Text, d.Before, d.After, r.Groups[0].Title} {
				if !strings.Contains(text, "导出结束") {
					t.Fatalf("garbled: %q", text)
				}
			}
			if d.Offset != uint64(strings.Index(raw, "2026-10-16 12:00:01")) {
				t.Fatal("raw offset changed")
			}
			var out bytes.Buffer
			if err := Export(r, filter, &out); err != nil || !strings.Contains(out.String(), "导出结束") {
				t.Fatalf("export: %q %v", out.String(), err)
			}
		}
		var out bytes.Buffer
		if err := ExportTime(context.Background(), r, 0, "2026-10-16T12:00:01", "2026-10-16T12:00:01", &out); err != nil || !strings.Contains(out.String(), "导出结束") {
			t.Fatalf("time export: %q %v", out.String(), err)
		}
		got, _ := os.ReadFile(name)
		if string(got) != raw {
			t.Fatal("source modified")
		}
	}
}

func TestEncodingBoundariesAndReports(t *testing.T) {
	for _, encoding := range []string{"UTF-8", "GB18030"} {
		t.Run(encoding, func(t *testing.T) {
			message := "导出结束𠀀"
			payload := []byte(message)
			if encoding == "GB18030" {
				var err error
				payload, err = simplifiedchinese.GB18030.NewEncoder().Bytes(payload)
				if err != nil {
					t.Fatal(err)
				}
			}
			// The first Chinese character straddles a 64 KiB raw read boundary.
			raw := "2026-10-16 12:00:00 ERROR demo : "
			raw += strings.Repeat("a", 64*1024-1-len(raw)) + string(payload) + "\n"
			name := testFile(t, "large.log", raw)
			r, err := Analyze([]string{name}, t.TempDir(), Options{})
			if err != nil {
				t.Fatal(err)
			}
			reportPath := name + ".json"
			if err = SaveReport(r, reportPath); err != nil {
				t.Fatal(err)
			}
			r, err = LoadReport(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			if r.Sources[0].Encoding != encoding {
				t.Fatal(r.Sources[0].Encoding)
			}
			for _, f := range []Filter{{Source: -1, Keyword: message}, {Source: -1, Regex: "导出.*𠀀"}} {
				details, _, err := Details(r, f, 0, 10)
				if err != nil || len(details) != 1 {
					t.Fatalf("details=%+v err=%v", details, err)
				}
				if !strings.Contains(details[0].Text, message) || !strings.Contains(details[0].ContextText, message) {
					t.Fatalf("boundary decoding: %+v", details[0])
				}
			}
		})
	}
}

func TestUTF8BOMAndLateGBK(t *testing.T) {
	for _, raw := range []string{
		"\xef\xbb\xbf2026-10-16 12:00:00 ERROR demo : 导出结束\n",
		strings.Repeat("ASCII prefix\n", 6000) + "2026-10-16 12:00:00 ERROR demo : " + string([]byte{0xb5, 0xbc, 0xb3, 0xf6, 0xbd, 0xe1, 0xca, 0xf8}) + "\n",
	} {
		name := testFile(t, "app.log", raw)
		r, err := Analyze([]string{name}, t.TempDir(), Options{})
		if err != nil {
			t.Fatal(err)
		}
		d, _, err := Details(r, Filter{Source: -1, Keyword: "导出结束"}, 0, 10)
		if err != nil || len(d) != 1 || !strings.Contains(d[0].Text, "导出结束") {
			t.Fatalf("details=%+v err=%v", d, err)
		}
	}
}

func TestJSONFileEncoding(t *testing.T) {
	for _, prefix := range []string{"", "\xef\xbb\xbf"} {
		path := testFile(t, "rules.json", prefix+`{"label":"导出结束"}`)
		var data map[string]string
		if err := ReadJSON(path, &data); err != nil || data["label"] != "导出结束" {
			t.Fatalf("%v %v", data, err)
		}
	}
	path := testFile(t, "rules.json", `{"label":"`+string([]byte{0xb5, 0xbc, 0xb3, 0xf6})+`"}`)
	var data map[string]string
	if err := ReadJSON(path, &data); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid encoding accepted: %v %v", data, err)
	}
}

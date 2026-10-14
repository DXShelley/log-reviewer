package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageEvidence(t *testing.T) {
	p, err := NewParser(nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"java":       "java.lang.NullPointerException\n at app.Main.run(Main.java:12)",
		"kotlin":     "java.lang.IllegalStateException\n at app.Main.run(Main.kt:12)",
		"scala":      " at app.Main.run(Main.scala:12)",
		"python":     "Traceback (most recent call last):\n File \"app.py\", line 12\nValueError: bad",
		"javascript": "TypeError: bad\n at app (/app/main.js:12:3)",
		"typescript": "TypeError: bad\n at app (/app/main.ts:12:3)",
		"go":         "panic: bad\ngoroutine 1 [running]:\nmain.go:12",
		"csharp":     "System.InvalidOperationException: bad\n at App.Run() in App.cs:line 12",
		"rust":       "thread 'main' panicked at app.rs:12:3",
		"php":        "PHP Fatal error: bad in app.php on line 12",
		"ruby":       "app.rb:12:in `run': bad (NoMethodError)",
		"cpp":        "terminate called after throwing an instance of 'std::runtime_error'",
		"c":          "==12==ERROR: AddressSanitizer: heap-use-after-free",
		"swift":      "main.swift:12: Fatal error: bad",
		"dart":       "Unhandled Exception: bad\nmain.dart:12:3",
		"elixir":     "** (RuntimeError) bad\napp.ex:12",
		"lua":        "lua: app.lua:12: bad\nstack traceback:",
	}
	for want, text := range cases {
		t.Run(want, func(t *testing.T) {
			got := p.Languages(text)
			if len(got) != 1 || got[0] != want {
				t.Fatalf("got %v", got)
			}
		})
	}
	if got := p.Languages("ERROR request failed\nTypeError: bad"); len(got) != 0 {
		t.Fatalf("ambiguous: %v", got)
	}
}

func TestMixedLanguagesAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.log")
	text := "ERROR java.lang.IllegalStateException: bad\n at App.run(App.java:12)\nERROR python failed\nTraceback (most recent call last):\n File \"app.py\", line 12\nValueError: bad\npanic: bad\ngoroutine 1 [running]:\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Analyze([]string{path}, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 3 || len(r.Sources[0].Languages) != 3 {
		t.Fatalf("events %d languages %v", r.Events, r.Sources[0].Languages)
	}
	r, err = Analyze([]string{path}, dir, Options{Languages: []LanguageRule{{ID: "java", Evidence: "never_matches"}, {ID: "custom", Evidence: "App\\.java"}}})
	if err != nil {
		t.Fatal(err)
	}
	if containsLanguage(r.Sources[0].Languages, "java") || !containsLanguage(r.Sources[0].Languages, "custom") {
		t.Fatal(r.Sources[0].Languages)
	}
	_, err = Analyze([]string{path}, dir, Options{Languages: []LanguageRule{{ID: "custom", Evidence: "["}}})
	if err == nil {
		t.Fatal("invalid regex accepted")
	}
}

func TestBareStackWarningsAndChainedRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bare.log")
	text := "TypeError: failed\n at run (/app/main.js:12:3)\nPHP Warning: undefined variable in app.php on line 12\napp.py:12: RuntimeWarning: overflow\nTraceback (most recent call last):\n File \"app.py\", line 12\nValueError: first\nDuring handling of the above exception, another exception occurred:\nTraceback (most recent call last):\n File \"app.py\", line 14\nRuntimeError: final\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Analyze([]string{path}, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 4 {
		t.Fatalf("events %d", r.Events)
	}
	warnings := 0
	foundRoot := false
	for _, g := range r.Groups {
		if g.Level == "WARN" {
			warnings++
		}
		if g.RootCause == "RuntimeError: final" {
			foundRoot = true
		}
	}
	if warnings != 2 || !foundRoot {
		t.Fatalf("warnings %d final root %v", warnings, foundRoot)
	}
}

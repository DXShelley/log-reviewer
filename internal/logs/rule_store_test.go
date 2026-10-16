package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuleStoreIsolationPersistenceAndReset(t *testing.T) {
	dir := t.TempDir()
	store := &RuleStore{Dir: dir}
	entries, err := store.Catalogue()
	if err != nil || len(entries) != 17 {
		t.Fatal(len(entries), err)
	}
	rule := LanguageRule{ID: "java", Label: "Java", Evidence: "MY_JAVA_FAILURE", Start: "^MY_JAVA_FAILURE:", Level: "ERROR"}
	if err = store.Save(rule); err != nil {
		t.Fatal(err)
	}
	second := &RuleStore{Dir: dir}
	entries, err = second.Catalogue()
	if err != nil {
		t.Fatal(err)
	}
	modified := 0
	for _, entry := range entries {
		if entry.Modified {
			modified++
			if entry.Rule.ID != "java" || entry.Rule.Evidence != "MY_JAVA_FAILURE" {
				t.Fatal(entry)
			}
		}
	}
	if modified != 1 {
		t.Fatal(modified)
	}
	bad := rule
	bad.Evidence = "["
	if err = second.Save(bad); err == nil {
		t.Fatal("bad regex saved")
	}
	saved, err := second.Overrides()
	if err != nil || len(saved) != 1 || saved[0].Evidence != rule.Evidence {
		t.Fatal(saved, err)
	}
	path := testFile(t, "java.log", "MY_JAVA_FAILURE: failed\n")
	report, err := Analyze([]string{path}, t.TempDir(), Options{Languages: saved})
	if err != nil || report.Events != 1 || !containsLanguage(report.Groups[0].Languages, "java") {
		t.Fatal(report, err)
	}
	if err = second.Save(LanguageRule{ID: "custom", Evidence: "CustomFault", Start: "^CustomFault:", Level: "WARN"}); err != nil {
		t.Fatal(err)
	}
	if err = second.Reset("java"); err != nil {
		t.Fatal(err)
	}
	entries, err = second.Catalogue()
	if err != nil || len(entries) != 18 {
		t.Fatal(len(entries), err)
	}
	for _, entry := range entries {
		if entry.Rule.ID == "java" && entry.Modified {
			t.Fatal("Java was not restored")
		}
	}
	if err = second.Reset("custom"); err != nil {
		t.Fatal(err)
	}
	if err = second.Save(LanguageRule{ID: "../escape", Evidence: "x"}); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err = second.Reset("../escape"); err == nil {
		t.Fatal("path traversal reset accepted")
	}
	if _, err = os.Stat(filepath.Join(dir, "java.json")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

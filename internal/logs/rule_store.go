package logs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type RuleStore struct {
	Dir string
	mu  sync.Mutex
}
type LanguageEntry struct {
	Rule     LanguageRule `json:"rule"`
	Builtin  bool         `json:"builtin"`
	Modified bool         `json:"modified"`
}

var languageID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)

func DefaultRuleDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "log-reviewer", "rules"), nil
}
func ValidateLanguageRule(rule LanguageRule) error {
	if !languageID.MatchString(rule.ID) {
		return errors.New("language ID must use lowercase letters, digits or hyphens (1-48 characters)")
	}
	if len(rule.Label) > 256 {
		return errors.New("language label is too long")
	}
	for _, pattern := range []string{rule.Evidence, rule.Start, rule.Warning, rule.Root} {
		if len(pattern) > 16384 {
			return errors.New("rule pattern exceeds 16 KiB")
		}
	}
	_, err := compileLanguages([]LanguageRule{rule})
	return err
}
func (s *RuleStore) Overrides() ([]LanguageRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return []LanguageRule{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []LanguageRule
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if len(rules) >= 256 {
			return nil, errors.New("more than 256 language overrides")
		}
		var rule LanguageRule
		if err = ReadJSON(filepath.Join(s.Dir, entry.Name()), &rule); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if rule.ID+".json" != entry.Name() {
			return nil, fmt.Errorf("language filename does not match ID: %s", entry.Name())
		}
		if err = ValidateLanguageRule(rule); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
func (s *RuleStore) Catalogue() ([]LanguageEntry, error) {
	builtin, err := BuiltinLanguages()
	if err != nil {
		return nil, err
	}
	overrides, err := s.Overrides()
	if err != nil {
		return nil, err
	}
	entries := map[string]LanguageEntry{}
	for _, rule := range builtin {
		entries[rule.ID] = LanguageEntry{Rule: rule, Builtin: true}
	}
	for _, rule := range overrides {
		entry := entries[rule.ID]
		entry.Rule = rule
		entry.Modified = true
		entries[rule.ID] = entry
	}
	result := make([]LanguageEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Rule.ID < result[j].Rule.ID })
	return result, nil
}
func backupRule(path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "log-reviewer-rule-backup-")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filepath.Base(path)), b, 0600)
}
func (s *RuleStore) Save(rule LanguageRule) error {
	if err := ValidateLanguageRule(rule); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(s.Dir, rule.ID+".json")
	if err := backupRule(path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".rule-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (s *RuleStore) Reset(id string) error {
	if !languageID.MatchString(id) {
		return errors.New("invalid language ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.Dir, id+".json")
	if err := backupRule(path); err != nil {
		return err
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

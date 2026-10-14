package logs

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed rules/*.json
var languageFiles embed.FS

type LanguageRule struct {
	ID              string `json:"id"`
	Evidence        string `json:"evidence"`
	Start           string `json:"start,omitempty"`
	Level           string `json:"level,omitempty"`
	Warning         string `json:"warning,omitempty"`
	Root            string `json:"root,omitempty"`
	warning, root   *regexp.Regexp
	evidence, start *regexp.Regexp
}

func BuiltinLanguages() ([]LanguageRule, error) {
	entries, err := languageFiles.ReadDir("rules")
	if err != nil {
		return nil, err
	}
	var rules []LanguageRule
	for _, entry := range entries {
		b, err := languageFiles.ReadFile("rules/" + entry.Name())
		if err != nil {
			return nil, err
		}
		var rule LanguageRule
		if err := json.Unmarshal(b, &rule); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func compileLanguages(rules []LanguageRule) ([]LanguageRule, error) {
	seen := map[string]bool{}
	for i := range rules {
		r := &rules[i]
		if r.ID == "" || seen[r.ID] || r.Evidence == "" {
			return nil, fmt.Errorf("invalid or duplicate language %q", r.ID)
		}
		seen[r.ID] = true
		var err error
		r.evidence, err = regexp.Compile(r.Evidence)
		if err != nil {
			return nil, fmt.Errorf("language %s evidence: %w", r.ID, err)
		}
		if r.Start != "" {
			// Start expressions identify a new exception block.
			r.start, err = regexp.Compile(r.Start)
			if err != nil {
				return nil, fmt.Errorf("language %s start: %w", r.ID, err)
			}
			if canonical(r.Level) == "" {
				return nil, fmt.Errorf("language %s requires level", r.ID)
			}
		}
		if r.Warning != "" {
			r.warning, err = regexp.Compile(r.Warning)
			if err != nil {
				return nil, err
			}
		}
		if r.Root != "" {
			r.root, err = regexp.Compile(r.Root)
			if err != nil {
				return nil, err
			}
		}
	}
	return rules, nil
}

func (p *Parser) Languages(text string) []string {
	result := []string{}
	for _, r := range p.languages {
		if r.evidence.MatchString(text) {
			result = append(result, r.ID)
		}
	}
	// JVM subclasses provide stronger evidence than shared Java exception types.
	if containsLanguage(result, "kotlin") || containsLanguage(result, "scala") {
		result = removeLanguage(result, "java")
	}
	if containsLanguage(result, "typescript") {
		result = removeLanguage(result, "javascript")
	}
	sort.Strings(result)
	return result
}
func containsLanguage(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func removeLanguage(values []string, value string) []string {
	var out []string
	for _, v := range values {
		if v != value {
			out = append(out, v)
		}
	}
	return out
}

func (p *Parser) languageStart(b []byte) (Header, bool) {
	// Ordinary timestamped logs use the existing fast header parser.
	if len(b) == 0 || b[0] >= '0' && b[0] <= '9' || b[0] == '{' || b[0] == '[' {
		return Header{}, false
	}
	for _, r := range p.languages {
		if r.start != nil && r.start.Match(b) {
			format := r.ID + "/exception"
			message := strings.TrimSpace(string(b))
			if r.ID == "go" && strings.HasPrefix(message, "panic:") {
				format = "go/panic"
				message = strings.TrimSpace(strings.TrimPrefix(message, "panic:"))
			}
			if r.ID == "python" && strings.HasPrefix(string(b), "Traceback") {
				format = "python/traceback"
			}
			return Header{Level: canonical(r.Level), Message: message, Format: format}, true
		}
		if r.warning != nil && r.warning.Match(b) {
			return Header{Level: "WARN", Message: string(b), Format: r.ID + "/warning"}, true
		}
	}
	return Header{}, false
}

func (p *Parser) signature(text string, languages []string) (string, string) {
	sig, root := signature(text)
	for _, r := range p.languages {
		if r.root == nil || !containsLanguage(languages, r.ID) {
			continue
		}
		matches := r.root.FindAllString(text, -1)
		if len(matches) > 0 {
			root = strings.TrimSpace(matches[len(matches)-1])
			sig, _ = signature(root)
		}
	}
	return sig, root
}

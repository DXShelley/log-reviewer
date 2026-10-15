package logs

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const RecordSize = 40
const maxPreview = 64 * 1024

type Source struct {
	Remote    *RemoteSource `json:"remote,omitempty"`
	Languages []string      `json:"languages"`
	Path      string        `json:"path"`
	Size      int64         `json:"size"`
	Modified  int64         `json:"modified"`
	Lines     uint64        `json:"lines"`
	Events    uint64        `json:"events"`
}
type RemoteSource struct {
	Modified   string `json:"modified,omitempty"`
	Protocol   string `json:"protocol"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Path       string `json:"path"`
	CapturedAt string `json:"captured_at"`
	Changed    bool   `json:"changed"`
}
type Group struct {
	Languages   []string   `json:"languages"`
	ID          string     `json:"id"`
	Number      uint32     `json:"number"`
	Level       string     `json:"level"`
	Title       string     `json:"title"`
	RootCause   string     `json:"root_cause,omitempty"`
	Count       uint64     `json:"count"`
	First       string     `json:"first,omitempty"`
	Last        string     `json:"last,omitempty"`
	UnknownTime uint64     `json:"unknown_time"`
	Sources     []int      `json:"sources"`
	Loggers     []string   `json:"loggers"`
	Impact      string     `json:"impact"`
	ImpactBasis string     `json:"impact_basis"`
	Format      string     `json:"format"`
	Samples     []Evidence `json:"samples"`
	first, last int64
}
type Evidence struct {
	Source uint32 `json:"source"`
	Offset uint64 `json:"offset"`
	Length uint64 `json:"length"`
	Line   uint64 `json:"line"`
	Time   string `json:"time,omitempty"`
}
type Report struct {
	Readers   map[string]SourceOpener `json:"-"`
	Schema    string                  `json:"schema"`
	Version   string                  `json:"version"`
	Created   string                  `json:"created"`
	ElapsedMS int64                   `json:"elapsed_ms"`
	Bytes     int64                   `json:"bytes"`
	Lines     uint64                  `json:"lines"`
	Events    uint64                  `json:"events"`
	Sources   []Source                `json:"sources"`
	Groups    []*Group                `json:"groups"`
	Formats   map[string]uint64       `json:"formats"`
	Notes     []string                `json:"notes"`
	IndexPath string                  `json:"index_path"`
}
type Progress struct {
	Bytes  atomic.Int64
	Total  atomic.Int64
	Events atomic.Uint64
	Lines  atomic.Uint64
	Cancel atomic.Bool
}
type Options struct {
	Readers       map[string]SourceOpener
	RemoteSources map[string]RemoteSource
	Languages     []LanguageRule
	All           bool
	Rules         []Rule
	Progress      *Progress
}
type Record struct {
	Group, Source        uint32
	Offset, Length, Line uint64
	Time                 int64
}

func (r Record) Encode(b []byte) {
	binary.LittleEndian.PutUint32(b, r.Group)
	binary.LittleEndian.PutUint32(b[4:], r.Source)
	binary.LittleEndian.PutUint64(b[8:], r.Offset)
	binary.LittleEndian.PutUint64(b[16:], r.Length)
	binary.LittleEndian.PutUint64(b[24:], r.Line)
	binary.LittleEndian.PutUint64(b[32:], uint64(r.Time))
}
func Decode(b []byte) Record {
	return Record{binary.LittleEndian.Uint32(b), binary.LittleEndian.Uint32(b[4:]), binary.LittleEndian.Uint64(b[8:]), binary.LittleEndian.Uint64(b[16:]), binary.LittleEndian.Uint64(b[24:]), int64(binary.LittleEndian.Uint64(b[32:]))}
}
func stamp(n int64) string {
	if n == 0 {
		return ""
	}
	return time.Unix(0, n).Format(time.RFC3339Nano)
}
func ExpandPaths(args []string, patterns ...string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"*.log", "*.log.*", "*.txt", "*.text", "*.out", "*.err", "*.trace"}
	}
	matches := func(name string) bool {
		for _, pattern := range patterns {
			if ok, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(name)); ok {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	var paths []string
	add := func(p string) error {
		a, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		if !seen[a] {
			st, err := os.Stat(a)
			if err != nil {
				return err
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("not a regular file: %s", a)
			}
			seen[a] = true
			paths = append(paths, a)
		}
		return nil
	}
	for _, input := range args {
		p, normalizeErr := normalizePath(input)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		if p == "" {
			continue
		}
		st, err := os.Stat(p)
		if err == nil && st.Mode().IsRegular() {
			if err := add(p); err != nil {
				return nil, err
			}
			continue
		}
		if err == nil && st.IsDir() {
			entries, err := os.ReadDir(p)
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if !e.IsDir() && matches(e.Name()) {
					if err := add(filepath.Join(p, e.Name())); err != nil {
						return nil, err
					}
				}
			}
			continue
		}
		if strings.ContainsAny(p, "*?[") {
			matches, err := globPaths(p)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("no files match: %s", p)
			}
			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					return nil, err
				}
				if info.IsDir() {
					continue
				}
				if err := add(m); err != nil {
					return nil, err
				}
			}
		} else {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}
	if len(paths) == 0 {
		return nil, errors.New("no log files selected")
	}
	sort.Strings(paths)
	return paths, nil
}

// Preserve an existing directory prefix as literal text, even when it contains glob metacharacters.
func globPaths(pattern string) ([]string, error) {
	for parent := filepath.Dir(pattern); ; {
		if info, err := os.Stat(parent); err == nil && info.IsDir() {
			relative, err := filepath.Rel(parent, pattern)
			if err != nil {
				return nil, err
			}
			pairs := []string{"[", "[[]", "*", "[*]", "?", "[?]"}
			if filepath.Separator != '\\' {
				pairs = append(pairs, `\`, `\\`)
			}
			return filepath.Glob(filepath.Join(strings.NewReplacer(pairs...).Replace(parent), relative))
		}
		next := filepath.Dir(parent)
		if next == parent {
			return filepath.Glob(pattern)
		}
		parent = next
	}
}
func Analyze(paths []string, dir string, opt Options) (*Report, error) {
	start := time.Now()
	p, err := NewParser(opt.Rules)
	if err != nil {
		return nil, err
	}
	if len(opt.Languages) > 0 {
		byID := map[string]LanguageRule{}
		for _, r := range p.languages {
			byID[r.ID] = r
		}
		for _, r := range opt.Languages {
			byID[r.ID] = r
		}
		var merged []LanguageRule
		for _, r := range byID {
			merged = append(merged, r)
		}
		sort.Slice(merged, func(i, j int) bool { return merged[i].ID < merged[j].ID })
		p.languages, err = compileLanguages(merged)
		if err != nil {
			return nil, err
		}
	}
	paths, err = expandSources(paths, opt.Readers)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, "events-*.idx")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(f.Name())
		}
	}()
	w := bufio.NewWriterSize(f, 1024*1024)
	report := &Report{Schema: "log-reviewer/v1", Version: "0.7.1", Created: time.Now().Format(time.RFC3339), Formats: map[string]uint64{}, Groups: []*Group{}, Sources: []Source{}, IndexPath: f.Name(), Notes: []string{"Impact is a heuristic based on log severity; business impact is not inferred.", "Scope lists observed files and logger/service names, not affected users.", "Local timestamps without offsets use the scanning machine's local timezone.", "Grouping masks numeric values and IDs; inspect occurrences before assuming identical causes."}}
	for _, path := range paths {
		in, err := openRead(path, opt.Readers)
		if err != nil {
			return nil, err
		}
		st, err := in.Stat()
		in.Close()
		if err != nil {
			return nil, err
		}
		report.Sources = append(report.Sources, Source{Path: path, Size: st.Size(), Modified: st.ModTime().UnixNano()})
		if origin, ok := opt.RemoteSources[path]; ok {
			report.Sources[len(report.Sources)-1].Remote = &origin
		}
		report.Bytes += st.Size()
	}
	if opt.Progress != nil {
		opt.Progress.Total.Store(report.Bytes)
	}
	groups := map[string]*Group{}
	type cached struct{ sig, root string }
	sigCache := map[string]cached{}
	languageCache := map[string][]string{}
	var buf [RecordSize]byte
	for si := range report.Sources {
		src := &report.Sources[si]
		src.Languages = []string{}
		in, err := openRead(src.Path, opt.Readers)
		if err != nil {
			return nil, err
		}
		r := bufio.NewReaderSize(in, 1024*1024)
		var offset uint64
		if prefix, _ := r.Peek(2); len(prefix) == 2 && (prefix[0] == 0xff && prefix[1] == 0xfe || prefix[0] == 0xfe && prefix[1] == 0xff || prefix[0] == 0x1f && prefix[1] == 0x8b) {
			in.Close()
			return nil, fmt.Errorf("unsupported UTF-16 or gzip file: %s; convert to UTF-8 or decompress first", src.Path)
		}
		var line uint64
		var pending *Record
		var header Header
		stack := sha256.New()
		preview := make([]byte, 0, 4096)
		completedBytes := sourceBytes(report.Sources, si)
		flush := func() error {
			if pending == nil {
				return nil
			}
			text := string(preview)
			languages, languageCached := languageCache[text]
			if !languageCached {
				languages = p.Languages(text)
				if len(languageCache) >= 1024 {
					languageCache = map[string][]string{}
				}
				if len(text) <= 8192 {
					languageCache[text] = languages
				}
			}
			for _, language := range languages {
				if !containsLanguage(src.Languages, language) {
					src.Languages = append(src.Languages, language)
					sort.Strings(src.Languages)
				}
			}
			if header.Format == "go/panic" && !containsLanguage(languages, "go") {
				languages = append(append([]string{}, languages...), "go")
				sort.Strings(languages)
				if !containsLanguage(src.Languages, "go") {
					src.Languages = append(src.Languages, "go")
					sort.Strings(src.Languages)
				}
			}
			cache, found := sigCache[text]
			if !found {
				cache.sig, cache.root = p.signature(text, languages)
				if len(sigCache) >= 1024 {
					sigCache = map[string]cached{}
				}
				if len(text) <= 8192 {
					sigCache[text] = cache
				}
			}
			sig, root := cache.sig, cache.root
			key := header.Level + "\x00" + header.Logger + "\x00" + sig
			key += "\x00" + strings.Join(languages, ",")
			key += "\x00" + hex.EncodeToString(stack.Sum(nil))
			g := groups[key]
			if g == nil {
				if len(groups) >= 100000 {
					return errors.New("more than 100000 categories; narrow input or add a normalization rule")
				}
				hash := sha256.Sum256([]byte(key))
				impact, basis := "低", "DEBUG/INFO/TRACE"
				if header.Level == "WARN" {
					impact = "中"
					basis = "WARN: 潜在异常，需结合业务判断"
				}
				if header.Level == "ERROR" {
					impact = "高"
					basis = "ERROR: 操作失败，实际业务影响未知"
				}
				if header.Level == "FATAL" {
					impact = "严重"
					basis = "FATAL/CRITICAL/PANIC: 进程级故障信号"
				}
				g = &Group{ID: hex.EncodeToString(hash[:8]), Number: uint32(len(report.Groups) + 1), Level: header.Level, Title: sig, RootCause: root, Sources: []int{}, Loggers: []string{}, Impact: impact, ImpactBasis: basis, Format: header.Format}
				groups[key] = g
				g.Languages = languages
				report.Groups = append(report.Groups, g)
			}
			g.Count++
			if len(g.Samples) < 3 {
				g.Samples = append(g.Samples, Evidence{pending.Source, pending.Offset, pending.Length, pending.Line, stamp(pending.Time)})
			}
			if pending.Time == 0 {
				g.UnknownTime++
			} else {
				if g.first == 0 || pending.Time < g.first {
					g.first = pending.Time
				}
				if pending.Time > g.last {
					g.last = pending.Time
				}
			}
			if len(g.Sources) == 0 || g.Sources[len(g.Sources)-1] != si {
				g.Sources = append(g.Sources, si)
			}
			if header.Logger != "" && len(g.Loggers) == 0 {
				g.Loggers = append(g.Loggers, header.Logger)
			}
			pending.Group = g.Number
			pending.Encode(buf[:])
			if _, err := w.Write(buf[:]); err != nil {
				return err
			}
			src.Events++
			report.Events++
			report.Formats[header.Format]++
			if opt.Progress != nil {
				opt.Progress.Events.Store(report.Events)
			}
			pending = nil
			return nil
		}
		scanErr := func() error {
			for {
				if opt.Progress != nil && opt.Progress.Cancel.Load() {
					return errors.New("analysis canceled")
				}
				b, readErr := r.ReadSlice('\n')
				if len(b) == 0 && readErr == io.EOF {
					return flush()
				}
				lineStart := offset
				offset += uint64(len(b))
				line++
				src.Lines++
				report.Lines++
				// Oversized physical lines are streamed; only a bounded prefix is parsed.
				if readErr == bufio.ErrBufferFull {
					prefix := append([]byte(nil), b[:maxPreview]...)
					for readErr == bufio.ErrBufferFull {
						b, readErr = r.ReadSlice('\n')
						offset += uint64(len(b))
					}
					b = prefix
				}
				if len(b) > 0 {
					h, ok := p.Parse(b, opt.All)
					if h.Attach && pending != nil {
						ok = false
					}
					if ok && h.Format == "python/traceback" && pending != nil && header.Level == "ERROR" {
						ok = false
					}
					if ok {
						if err := flush(); err != nil {
							return err
						}
						if interested(h.Level, opt.All) {
							header = h
							stack.Reset()
							appendStack(stack, h.Message)
							preview = preview[:0]
							preview = append(preview, h.Message...)
							preview = append(preview, '\n')
							if len(preview) > maxPreview {
								preview = preview[:maxPreview]
							}
							pending = &Record{Source: uint32(si), Offset: lineStart, Length: offset - lineStart, Line: line, Time: h.Time}
						}
					} else if pending != nil {
						appendStack(stack, string(b))
						pending.Length = offset - pending.Offset
						if bytesRoot(b) && len(b) > maxPreview-len(preview) { // retain a complete cause when the preview is nearly full
							t := strings.TrimSpace(string(b))
							if len(t) > 2048 {
								t = t[:2048]
							}
							keep := maxPreview - len(t) - 2
							if keep > len(preview) {
								keep = len(preview)
							}
							preview = append(preview[:keep], '\n')
							preview = append(preview, t...)
							preview = append(preview, '\n')
						} else if len(preview) < maxPreview {
							n := len(b)
							if n > maxPreview-len(preview) {
								n = maxPreview - len(preview)
							}
							preview = append(preview, b[:n]...)
						} else if bytesRoot(b) { // retain deepest Java cause even after a very long stack
							t := strings.TrimSpace(string(b))
							if len(t) > 2048 {
								t = t[:2048]
							}
							preview = append(preview[:maxPreview-2049], ('\n'))
							preview = append(preview, t...)
						}
					}
				}
				if opt.Progress != nil {
					opt.Progress.Bytes.Store(int64(offset) + completedBytes)
					opt.Progress.Lines.Store(report.Lines)
				}
				if readErr != nil {
					if readErr != io.EOF {
						return readErr
					}
					return flush()
				}
			}
		}()
		after, statErr := in.Stat()
		if opt.Readers[src.Path] == nil {
			after, statErr = os.Stat(src.Path)
		}
		closeErr := in.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("%s: %w", src.Path, scanErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if statErr != nil {
			return nil, statErr
		}
		if after.Size() != src.Size || after.ModTime().UnixNano() != src.Modified {
			return nil, fmt.Errorf("file changed while scanning: %s; use a stable copy", src.Path)
		}
	}
	if err = w.Flush(); err != nil {
		return nil, err
	}
	for _, g := range report.Groups {
		g.First = stamp(g.first)
		g.Last = stamp(g.last)
	}
	sort.SliceStable(report.Groups, func(i, j int) bool { return report.Groups[i].Count > report.Groups[j].Count })
	report.ElapsedMS = time.Since(start).Milliseconds()
	success = true
	report.Readers = opt.Readers
	return report, nil
}
func bytesRoot(b []byte) bool { return strings.HasPrefix(strings.TrimSpace(string(b)), "Caused by:") }
func sourceBytes(s []Source, n int) int64 {
	var total int64
	for i := 0; i < n; i++ {
		total += s[i].Size
	}
	return total
}
func SaveReport(r *Report, path string) error {
	if err := SafeOutput(r, path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".report-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
func SafeOutput(r *Report, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dest, _ := os.Stat(abs)
	protected := []string{r.IndexPath}
	for _, s := range r.Sources {
		protected = append(protected, s.Path)
	}
	for _, p := range protected {
		if abs == p {
			return fmt.Errorf("output cannot overwrite source or index: %s", p)
		}
		if dest != nil {
			st, err := os.Stat(p)
			if err == nil && os.SameFile(dest, st) {
				return fmt.Errorf("output aliases a source or index: %s", p)
			}
		}
	}
	return nil
}
func LoadReport(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err = json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.Schema != "log-reviewer/v1" {
		return nil, errors.New("unsupported report schema")
	}
	return &r, nil
}

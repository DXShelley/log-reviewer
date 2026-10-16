package logs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

func sortGroups(g []*Group) {
	sort.SliceStable(g, func(i, j int) bool { return g[i].Count > g[j].Count })
}

type Filter struct {
	Group   string `json:"group"`
	Level   string `json:"level"`
	Keyword string `json:"keyword"`
	Regex   string `json:"regex"`
	From    string `json:"from"`
	To      string `json:"to"`
	Source  int    `json:"source"`
}
type query struct {
	Filter
	from, to int64
	re       *regexp.Regexp
	groups   map[uint32]*Group
	files    map[uint32]ReadFile
	scratch  []byte
}

func ParseBound(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	for _, l := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t.UnixNano(), nil
		}
	}
	return 0, fmt.Errorf("invalid time: %s", s)
}

// A date/minute/second upper bound includes its entire displayed precision.
func ParseUpperBound(s string) (int64, error) {
	n, err := ParseBound(s)
	if err != nil || s == "" {
		return n, err
	}
	precision := s
	if i := strings.IndexAny(precision, "T "); i >= 0 {
		precision = precision[i+1:]
		if j := strings.IndexAny(precision, "Z+-"); j >= 0 {
			precision = precision[:j]
		}
	} else if len(precision) == 10 {
		return time.Unix(0, n).In(time.Local).AddDate(0, 0, 1).UnixNano() - 1, nil
	}
	if !strings.ContainsAny(precision, ".,") {
		switch strings.Count(precision, ":") {
		case 1:
			n += int64(time.Minute) - 1
		case 2:
			n += int64(time.Second) - 1
		}
	}
	return n, nil
}
func newQuery(report *Report, f Filter) (*query, error) {
	q := &query{Filter: f, groups: map[uint32]*Group{}, files: map[uint32]ReadFile{}, scratch: make([]byte, 64*1024)}
	var err error
	q.from, err = ParseBound(f.From)
	if err != nil {
		return nil, err
	}
	q.to, err = ParseUpperBound(f.To)
	if err != nil {
		return nil, err
	}
	if q.from != 0 && q.to != 0 && q.from > q.to {
		return nil, errors.New("start time is after end time")
	}
	if len(f.Regex) > 4096 {
		return nil, errors.New("regular expression is too long")
	}
	if f.Regex != "" {
		q.re, err = regexp.Compile(f.Regex)
		if err != nil {
			return nil, err
		}
	}
	for _, g := range report.Groups {
		q.groups[g.Number] = g
	}
	return q, nil
}
func (q *query) close() {
	for _, f := range q.files {
		f.Close()
	}
}
func validatedFile(r *Report, si uint32) (ReadFile, error) {
	if int(si) >= len(r.Sources) {
		return nil, errors.New("invalid source index")
	}
	s := r.Sources[si]
	f, err := openRead(s.Path, r.Readers)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.Size() != s.Size || st.ModTime().UnixNano() != s.Modified {
		f.Close()
		return nil, fmt.Errorf("source changed since analysis: %s; analyze again", s.Path)
	}
	encoding := s.Encoding
	if encoding == "" {
		b := make([]byte, 64*1024)
		n, readErr := f.ReadAt(b, 0)
		if readErr != nil && readErr != io.EOF {
			f.Close()
			return nil, readErr
		}
		encoding = sourceEncoding(b[:n])
	}
	return &encodedFile{ReadFile: f, encoding: encoding}, nil
}
func (q *query) match(report *Report, r Record) (bool, error) {
	g := q.groups[r.Group]
	if g == nil {
		return false, errors.New("corrupt index: unknown group")
	}
	if q.Group != "" && q.Group != g.ID {
		return false, nil
	}
	if q.Level != "" && q.Level != g.Level {
		return false, nil
	}
	if q.Source >= 0 && int(r.Source) != q.Source {
		return false, nil
	}
	if q.from != 0 && (r.Time == 0 || r.Time < q.from) || q.to != 0 && (r.Time == 0 || r.Time > q.to) {
		return false, nil
	}
	if q.Keyword == "" && q.re == nil {
		return true, nil
	}
	f := q.files[r.Source]
	if f == nil {
		var err error
		f, err = validatedFile(report, r.Source)
		if err != nil {
			return false, err
		}
		q.files[r.Source] = f
	}
	if r.Length <= uint64(len(q.scratch)) {
		b := q.scratch[:int(r.Length)]
		_, err := f.ReadAt(b, int64(r.Offset))
		if err != nil && err != io.EOF {
			return false, err
		}
		b = fileText(f, b)
		if q.Keyword != "" && !strings.Contains(strings.ToLower(string(b)), strings.ToLower(q.Keyword)) {
			return false, nil
		}
		if q.re != nil {
			for len(b) > 0 {
				i := bytes.IndexByte(b, '\n')
				if i < 0 {
					i = len(b)
				}
				if q.re.Match(bytes.TrimRight(b[:i], "\r")) {
					return true, nil
				}
				if i == len(b) {
					break
				}
				b = b[i+1:]
			}
			return false, nil
		}
		return true, nil
	}
	reader := bufio.NewReaderSize(textReader(f, io.NewSectionReader(f, int64(r.Offset), int64(r.Length))), 64*1024)
	keywordOK := q.Keyword == ""
	regexOK := q.re == nil
	needle := strings.ToLower(q.Keyword)
	var prefix string
	var regexLine []byte
	decodeChunk := chunkText(nil)
	for {
		b, err := reader.ReadSlice('\n')
		s := strings.ToLower(string(decodeChunk(b, err != bufio.ErrBufferFull)))
		if !keywordOK && strings.Contains(prefix+s, needle) {
			keywordOK = true
		}
		if !regexOK {
			if len(regexLine)+len(b) > 1024*1024 {
				return false, errors.New("regex search supports physical lines up to 1 MiB; use keyword search or export for larger lines")
			}
			regexLine = append(regexLine, b...)
			if err != bufio.ErrBufferFull {
				if q.re.Match(bytes.TrimRight(regexLine, "\r\n")) {
					regexOK = true
				}
				regexLine = regexLine[:0]
			}
		}
		if keywordOK && regexOK {
			return true, nil
		}
		if err == bufio.ErrBufferFull {
			keep := len(needle)
			if keep > len(s) {
				keep = len(s)
			}
			prefix = s[len(s)-keep:]
		} else {
			prefix = ""
		}
		if err != nil && err != bufio.ErrBufferFull {
			if err != io.EOF {
				return false, err
			}
			return false, nil
		}
	}
}
func Walk(r *Report, f Filter, visit func(Record, *Group) error) error {
	q, err := newQuery(r, f)
	if err != nil {
		return err
	}
	defer q.close()
	idx, err := os.Open(r.IndexPath)
	if err != nil {
		return err
	}
	defer idx.Close()
	reader := bufio.NewReaderSize(idx, 1024*1024)
	var b [RecordSize]byte
	for {
		_, err := io.ReadFull(reader, b[:])
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("corrupt index: %w", err)
		}
		rec := Decode(b[:])
		if int(rec.Source) >= len(r.Sources) || rec.Offset > uint64(r.Sources[rec.Source].Size) || rec.Length > uint64(r.Sources[rec.Source].Size)-rec.Offset {
			return errors.New("corrupt index: invalid source span")
		}
		ok, err := q.match(r, rec)
		if err != nil {
			return err
		}
		if ok {
			if err := visit(rec, q.groups[rec.Group]); err != nil {
				return err
			}
		}
	}
}
func Summarize(r *Report, f Filter) ([]*Group, error) {
	// Validate even an empty query; invalid regular expressions never silently pass.
	q, err := newQuery(r, f)
	if err != nil {
		return nil, err
	}
	q.close()
	if f.Group == "" && f.Level == "" && f.Keyword == "" && f.Regex == "" && f.From == "" && f.To == "" && f.Source < 0 {
		return r.Groups, nil
	}
	out := map[uint32]*Group{}
	var ordered []*Group
	err = Walk(r, f, func(rec Record, g *Group) error {
		v := out[g.Number]
		if v == nil {
			copy := *g
			copy.Count = 0
			copy.UnknownTime = 0
			copy.First = ""
			copy.Last = ""
			copy.first = 0
			copy.last = 0
			copy.Sources = []int{}
			copy.Samples = []Evidence{}
			v = &copy
			out[g.Number] = v
			ordered = append(ordered, v)
		}
		v.Count++
		if len(v.Samples) < 3 {
			v.Samples = append(v.Samples, Evidence{rec.Source, rec.Offset, rec.Length, rec.Line, stamp(rec.Time)})
		}
		if rec.Time == 0 {
			v.UnknownTime++
		} else {
			if v.first == 0 || rec.Time < v.first {
				v.first = rec.Time
			}
			if rec.Time > v.last {
				v.last = rec.Time
			}
		}
		si := int(rec.Source)
		if len(v.Sources) == 0 || v.Sources[len(v.Sources)-1] != si {
			v.Sources = append(v.Sources, si)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, g := range ordered {
		g.First = stamp(g.first)
		g.Last = stamp(g.last)
	}
	sortGroups(ordered)
	return ordered, nil
}

type Detail struct {
	EventID          string        `json:"event_id"`
	EndLine          uint64        `json:"end_line"`
	EndTime          string        `json:"end_time"`
	EndOffset        uint64        `json:"end_offset"`
	ContextMode      string        `json:"context_mode,omitempty"`
	ContextReason    string        `json:"context_reason,omitempty"`
	ContextBefore    int           `json:"context_before"`
	ContextAfter     int           `json:"context_after"`
	Key              string        `json:"key,omitempty"`
	Remote           *RemoteSource `json:"remote,omitempty"`
	ContextLine      uint64        `json:"context_line,omitempty"`
	ContextText      string        `json:"context_text,omitempty"`
	ContextTruncated bool          `json:"context_truncated,omitempty"`
	GroupID          string        `json:"group_id"`
	Source           string        `json:"source"`
	Offset           uint64        `json:"offset"`
	Length           uint64        `json:"length"`
	Line             uint64        `json:"line"`
	Time             string        `json:"time"`
	Text             string        `json:"text"`
	Before           string        `json:"before"`
	After            string        `json:"after"`
	Truncated        bool          `json:"truncated"`
}

type ContextOptions struct {
	Before, After  int
	Keyword, Regex string
	Summary        bool
	Auto           bool
}

var eventKey = regexp.MustCompile(`(?i)(?:trace[_-]?id|request[_-]?id|correlation[_-]?id)["'\s:=]+[\w-]+`)

func ReadDetail(r *Report, rec Record, g *Group, options ...ContextOptions) (Detail, error) {
	context := ContextOptions{Before: 20, After: 100}
	if len(options) > 0 {
		context = options[0]
	}
	if context.Before < 0 || context.After < 0 || context.Before > 10000 || context.After > 10000 {
		return Detail{}, errors.New("context lines must be between 0 and 10000")
	}
	d := Detail{GroupID: g.ID, Source: r.Sources[rec.Source].Path, Remote: r.Sources[rec.Source].Remote, Offset: rec.Offset, Length: rec.Length, Line: rec.Line, Time: stamp(rec.Time)}
	d.EventID = fmt.Sprintf("S%d@%d+%d", rec.Source+1, rec.Offset, rec.Length)
	d.EndOffset = rec.Offset + rec.Length
	f, err := validatedFile(r, rec.Source)
	if err != nil {
		return d, err
	}
	defer f.Close()
	d.EndLine, d.EndTime, err = eventRange(f, rec)
	if err != nil {
		return d, err
	}
	if context.Summary {
		n := rec.Length
		if n > 4096 {
			n = 4096
		}
		b := make([]byte, n)
		if _, err = f.ReadAt(b, int64(rec.Offset)); err != nil && err != io.EOF {
			return d, err
		}
		d.Key = eventKey.FindString(string(b))
		return d, nil
	}
	n := rec.Length
	if n > 2*1024*1024 {
		n = 2 * 1024 * 1024
		d.Truncated = true
	}
	b := make([]byte, n)
	_, err = f.ReadAt(b, int64(rec.Offset))
	if err != nil && err != io.EOF {
		return d, err
	}
	d.Text = string(fileText(f, b))
	anchorStart, anchorEnd := int64(rec.Offset), int64(rec.Offset+rec.Length)
	if context.Keyword != "" || context.Regex != "" {
		var re *regexp.Regexp
		if context.Regex != "" {
			re, err = regexp.Compile(context.Regex)
			if err != nil {
				return d, err
			}
		}
		var found bool
		anchorStart, anchorEnd, d.ContextLine, d.ContextText, d.ContextTruncated, found, err = findContextAnchor(f, rec, context.Keyword, re)
		if err != nil {
			return d, err
		}
		if !found {
			anchorStart = int64(rec.Offset)
			anchorEnd = int64(rec.Offset + rec.Length)
		}
	}
	var truncated bool
	d.Before, truncated, err = readContext(f, anchorStart, context.Before, true)
	d.ContextTruncated = d.ContextTruncated || truncated
	if err != nil {
		return d, err
	}
	d.After, truncated, err = readContext(f, anchorEnd, context.After, false)
	d.ContextTruncated = d.ContextTruncated || truncated
	if err != nil {
		return d, err
	}
	d.ContextMode = "manual"
	if context.Auto {
		d.Before, d.After, d.ContextReason = automaticContext(d.Text, d.Before, d.After, rec.Time)
		d.ContextMode = "auto"
	}
	d.ContextBefore, d.ContextAfter = contextLines(d.Before), contextLines(d.After)
	return d, nil
}

// Derive exact bounds from the indexed byte span, including old report indexes.
func eventRange(f ReadFile, rec Record) (uint64, string, error) {
	reader := bufio.NewReaderSize(io.NewSectionReader(f, int64(rec.Offset), int64(rec.Length)), 64*1024)
	line, last := rec.Line, rec.Time
	parser := &Parser{}
	var prefix []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(prefix) < 4096 {
			n := min(len(part), 4096-len(prefix))
			prefix = append(prefix, part[:n]...)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return 0, "", err
		}
		if len(prefix) > 0 {
			if h, ok := parser.Parse(prefix, true); ok && h.HasTime {
				last = h.Time
			} else if n, ok := parser.parseTime(strings.TrimSpace(string(prefix)), ""); ok {
				last = n
			}
			line++
			prefix = prefix[:0]
		}
		if err == io.EOF {
			if line > rec.Line {
				line--
			}
			return line, stamp(last), nil
		}
	}
}

var stopWalk = errors.New("stop walk")

func Details(r *Report, f Filter, offset, limit int, options ...ContextOptions) ([]Detail, bool, error) {
	if len(options) == 0 {
		options = []ContextOptions{{Before: 20, After: 100}}
	}
	options[0].Keyword = f.Keyword
	options[0].Regex = f.Regex
	if len(options) > 0 && (options[0].Before < 0 || options[0].After < 0 || options[0].Before > 10000 || options[0].After > 10000) {
		return nil, false, errors.New("context lines must be between 0 and 10000")
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, false, errors.New("invalid pagination")
	}
	out := []Detail{}
	seen := 0
	more := false
	err := Walk(r, f, func(rec Record, g *Group) error {
		if seen < offset {
			seen++
			return nil
		}
		if len(out) >= limit {
			more = true
			return stopWalk
		}
		d, err := ReadDetail(r, rec, g, options...)
		if err != nil {
			return err
		}
		out = append(out, d)
		return nil
	})
	if err == stopWalk {
		err = nil
	}
	return out, more, err
}
func Export(r *Report, f Filter, w io.Writer) error {
	files := map[uint32]ReadFile{}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()
	return Walk(r, f, func(rec Record, g *Group) error {
		src := files[rec.Source]
		if src == nil {
			var err error
			src, err = validatedFile(r, rec.Source)
			if err != nil {
				return err
			}
			files[rec.Source] = src
		}
		if _, err := fmt.Fprintf(w, "\n--- %s | %s | %s:%d | %s ---\n", g.ID, g.Level, r.Sources[rec.Source].Path, rec.Line, stamp(rec.Time)); err != nil {
			return err
		}
		_, err := io.Copy(w, textReader(src, io.NewSectionReader(src, int64(rec.Offset), int64(rec.Length))))
		return err
	})
}

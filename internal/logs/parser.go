package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Rule extends recognition without recompiling the executable.
type Rule struct {
	Name       string `json:"name"`
	Pattern    string `json:"pattern"`
	TimeLayout string `json:"time_layout,omitempty"`
	re         *regexp.Regexp
}
type Header struct {
	Level, Message, Logger, Format string
	Time                           int64
	HasTime                        bool
}
type Parser struct {
	Rules     []Rule
	lastStamp string
	lastTime  int64
}

func NewParser(rules []Rule) (*Parser, error) {
	p := &Parser{Rules: rules}
	for i := range p.Rules {
		r := &p.Rules[i]
		var err error
		r.re, err = regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", r.Name, err)
		}
		if r.re.SubexpIndex("message") < 0 || r.re.SubexpIndex("level") < 0 {
			return nil, fmt.Errorf("rule %s requires named level and message groups", r.Name)
		}
	}
	return p, nil
}
func canonical(s string) string {
	switch strings.ToUpper(s) {
	case "WARNING", "WARN", "WRN", "W":
		return "WARN"
	case "ERROR", "ERR", "EROR", "E", "SEVERE", "FAIL":
		return "ERROR"
	case "FATAL", "FTL", "CRITICAL", "CRIT", "PANIC", "EMERG", "ALERT":
		return "FATAL"
	case "INFO", "INF", "INFORMATION", "I", "NOTICE":
		return "INFO"
	case "DEBUG", "DBG", "D", "DBUG":
		return "DEBUG"
	case "TRACE", "TRC", "T", "TRCE":
		return "TRACE"
	}
	return ""
}
func interested(s string, all bool) bool { return all || s == "ERROR" || s == "WARN" || s == "FATAL" }
func tokenByte(c byte) bool              { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func (p *Parser) Parse(b []byte, all bool) (Header, bool) {
	b = bytes.TrimSpace(b)
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if bytes.IndexByte(b, 0x1b) >= 0 {
		b = ansi.ReplaceAll(b, nil)
	}
	if len(b) == 0 {
		return Header{}, false
	}
	if bytes.HasPrefix(b, []byte("Traceback (most recent call last):")) {
		return Header{Level: "ERROR", Message: string(b), Format: "python/traceback"}, true
	}
	if bytes.HasPrefix(b, []byte("panic:")) {
		return Header{Level: "FATAL", Message: string(b[6:]), Format: "go/panic"}, true
	}
	if b[0] == '{' {
		if h, ok := p.parseJSON(b); ok {
			return h, true
		}
	}
	for _, r := range p.Rules {
		m := r.re.FindSubmatch(b)
		if m == nil {
			continue
		}
		get := func(name string) string {
			i := r.re.SubexpIndex(name)
			if i > 0 {
				return string(m[i])
			}
			return ""
		}
		l := canonical(get("level"))
		if l == "" {
			continue
		}
		h := Header{Level: l, Format: r.Name}
		if interested(l, all) {
			h.Message = get("message")
			h.Logger = get("logger")
			h.Time, h.HasTime = p.parseTime(get("time"), r.TimeLayout)
		}
		return h, true
	}
	// Inspect only the header region; stack frames and message text are not levels.
	end := len(b)
	if end > 320 {
		end = 320
	}
	levelStart, levelEnd := -1, -1
	l := ""
	for i := 0; i < end; {
		if !tokenByte(b[i]) {
			i++
			continue
		}
		start := i
		for i < end && tokenByte(b[i]) {
			i++
		}
		if i-start > 12 {
			continue
		}
		v := canonical(string(b[start:i]))
		if v == "" {
			continue
		}
		if i-start == 1 {
			continue
		}
		l = v
		levelStart = start
		levelEnd = i
		break
	}
	// glog's compact prefix (E1014 12:30:00.123456 ...).
	glog := false
	if l == "" && len(b) > 6 && b[1] >= '0' && b[1] <= '9' && b[5] == ' ' {
		l = canonical(string(b[:1]))
		if l != "" {
			levelStart = 0
			levelEnd = 1
			glog = true
		}
	}
	if l == "" {
		return Header{}, false
	}
	// An un-timestamped header must start with a level; do not split stack text.
	hasPrefix := len(b) > 4 && b[0] >= '0' && b[0] <= '9' || b[0] == '[' || glog || bytes.HasPrefix(b, []byte("["))
	if !hasPrefix && levelStart > 1 {
		return Header{}, false
	}
	h := Header{Level: l, Format: "text"}
	if !interested(l, all) {
		return h, true
	}
	msg := bytes.TrimLeft(b[levelEnd:], " \t:-]|,")
	if glog {
		if i := bytes.IndexByte(b, ']'); i >= 0 {
			msg = bytes.TrimSpace(b[i+1:])
			parts := bytes.Fields(b[:i])
			if len(parts) > 0 {
				component := parts[len(parts)-1]
				if j := bytes.LastIndexByte(component, ':'); j >= 0 {
					component = component[:j]
				}
				h.Logger = string(component)
			}
		}
	}
	if d := bytes.Index(msg, []byte(" : ")); d >= 0 {
		prefix := msg[:d]
		msg = msg[d+3:]
		if j := bytes.LastIndexByte(prefix, ' '); j >= 0 {
			h.Logger = string(bytes.TrimSpace(prefix[j+1:]))
		} else {
			h.Logger = string(prefix)
		}
		h.Format = "spring/logback"
	} else if d := bytes.Index(msg, []byte(" - ")); d >= 0 && d < 160 {
		prefix := bytes.TrimSpace(msg[:d])
		msg = msg[d+3:]
		if j := bytes.LastIndexByte(prefix, ' '); j >= 0 {
			prefix = prefix[j+1:]
		}
		h.Logger = string(prefix)
		h.Format = "logging/log4j"
	} else if len(msg) > 0 && msg[0] == '[' {
		if d := bytes.IndexByte(msg, ']'); d >= 0 {
			h.Logger = string(msg[1:d])
			msg = bytes.TrimLeft(msg[d+1:], " :-")
		}
	}
	h.Message = string(msg)
	if h.Logger == "" {
		if i := bytes.Index(msg, []byte(": ")); i > 0 && !bytes.ContainsAny(msg[:i], " \t") {
			h.Logger = string(bytes.TrimRight(msg[:i], ":"))
			h.Message = string(msg[i+2:])
		}
	}
	if levelStart == 0 && levelEnd < len(b) && b[levelEnd] == ':' && bytes.HasSuffix(msg, []byte("]")) {
		if i := bytes.IndexByte(msg, '['); i > 0 {
			h.Logger = string(msg[:i])
			h.Message = ""
			h.Format = "dotnet"
		}
	}
	stamp := b[:levelStart]
	h.Time, h.HasTime = p.parseTime(string(stamp), "")
	if glog {
		h.Format = "glog"
	}
	return h, true
}
func (p *Parser) parseJSON(b []byte) (Header, bool) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(b, &obj) != nil {
		return Header{}, false
	}
	val := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := obj[k]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					return s
				}
				return string(v)
			}
		}
		return ""
	}
	level := val("level", "severity", "log.level", "Level", "lvl", "@l")
	if level == "" && len(obj["@mt"]) > 0 {
		level = "INFO"
	}
	l := canonical(level)
	if l == "" {
		n, _ := strconv.Atoi(level)
		switch {
		case n >= 60:
			l = "FATAL"
		case n >= 50:
			l = "ERROR"
		case n >= 40:
			l = "WARN"
		case n >= 30:
			l = "INFO"
		case n >= 20:
			l = "DEBUG"
		case n >= 10:
			l = "TRACE"
		}
	}
	if l == "" {
		return Header{}, false
	}
	h := Header{Level: l, Format: "json", Message: val("message", "msg", "Message", "RenderedMessage", "event", "@m", "@mt"), Logger: val("logger", "logger_name", "name", "SourceContext", "service", "service.name", "component")}
	if h.Message == "" {
		h.Message = string(b)
	}
	for _, k := range []string{"err", "error", "exception", "stack", "stacktrace", "StackTrace", "@x"} {
		if v := val(k); v != "" && v != "null" {
			if strings.HasPrefix(v, "{") {
				var nested map[string]json.RawMessage
				if json.Unmarshal([]byte(v), &nested) == nil {
					var stack, msg string
					json.Unmarshal(nested["stack"], &stack)
					json.Unmarshal(nested["message"], &msg)
					if stack != "" {
						v = stack
					} else if msg != "" {
						v = msg
					}
				}
			}
			h.Message += "\n" + v
		}
	}
	t := val("timestamp", "time", "@timestamp", "ts", "Timestamp", "@t")
	h.Time, h.HasTime = p.parseTime(t, "")
	return h, true
}
func (p *Parser) parseTime(s, layout string) (int64, bool) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")
	if s == "" {
		return 0, false
	}
	// Select just the leading ISO/local timestamp, excluding thread and process fields.
	if layout == "" && len(s) >= 19 && (s[4] == '-' && s[7] == '-' || s[4] == '/' && s[7] == '/') {
		e := 19
		if len(s) > e && (s[e] == '.' || s[e] == ',') {
			e++
			for e < len(s) && s[e] >= '0' && s[e] <= '9' {
				e++
			}
		}
		if e < len(s) && s[e] == 'Z' {
			e++
		} else if e+6 <= len(s) && (s[e] == '+' || s[e] == '-') && s[e+3] == ':' {
			e += 6
		}
		s = s[:e]
	}
	if s == p.lastStamp {
		return p.lastTime, true
	}
	if layout != "" {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			p.lastStamp = s
			p.lastTime = t.UnixNano()
			return p.lastTime, true
		}
		return 0, false
	}
	for _, l := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05,999999999", "2006-01-02T15:04:05.999999999", "2006/01/02 15:04:05.999999999", "02/Jan/2006:15:04:05 -0700"} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			p.lastStamp = s
			p.lastTime = t.UnixNano()
			return p.lastTime, true
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 1e8 {
		switch {
		case n >= 1e17:
			return n, true
		case n >= 1e14:
			return n * 1000, true
		case n >= 1e11:
			return n * 1000000, true
		default:
			return n * 1000000000, true
		}
	}
	if parts := strings.Split(s, "."); len(parts) == 2 {
		sec, err := strconv.ParseInt(parts[0], 10, 64)
		frac := parts[1]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		frac += strings.Repeat("0", 9-len(frac))
		ns, err2 := strconv.ParseInt(frac, 10, 64)
		if err == nil && err2 == nil && sec > 1e8 && sec < 1e11 {
			return sec*1000000000 + ns, true
		}
	}
	return 0, false
}

var volatile = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b|\b0x[0-9a-f]+\b|\b[0-9a-f]{16,}\b|\b\d+(?:\.\d+)*(?:ms|us|ns|s|mb|gb)?\b`)
var exception = regexp.MustCompile(`(?:[A-Za-z_$][\w$]*\.)*(?:[A-Za-z_$][\w$]*)?(?:Exception|Error|Panic)\b`)
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

func signature(message string) (string, string) {
	root := ""
	for _, line := range strings.Split(message, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "Caused by:") {
			root = strings.TrimSpace(strings.TrimPrefix(t, "Caused by:"))
		} else if root == "" && !strings.HasPrefix(t, "at ") {
			if e := exception.FindString(t); e != "" {
				root = t
			}
		}
	}
	base := strings.TrimSpace(strings.Split(strings.TrimSpace(message), "\n")[0])
	if root != "" {
		base = root
	}
	if len(base) > 1024 {
		base = base[:1024]
	}
	base = volatile.ReplaceAllString(base, "<*>")
	base = strings.Join(strings.Fields(base), " ")
	if base == "" {
		base = "(empty message)"
	}
	return base, root
}

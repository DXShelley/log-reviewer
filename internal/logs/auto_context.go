package logs

import (
	"regexp"
	"strings"
	"time"
)

var contextID = regexp.MustCompile(`(?i)(?:trace[_-]?id|request[_-]?id|correlation[_-]?id)["'\s:=]+([\w-]+)`)
var stackHint = regexp.MustCompile(`(?m)(?:^\s+at\s|Caused by:|Traceback|^\s*File ".*", line |goroutine \d+|stack backtrace:|^\s*#\d+\s)`)
var causalHint = regexp.MustCompile(`(?i)timeout|timed out|connection|refused|retry|deadlock|pool|transaction|outofmemory|permission|denied|unavailable`)
var completionHint = regexp.MustCompile(`(?i)\b(completed|finished|recovered|succeeded|closed|resolved)\b`)
var threadHint = regexp.MustCompile(`\[([^\]\r\n]{1,100})\]`)

func contextLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
}
func contextIdentifier(text string) string {
	m := contextID.FindStringSubmatch(text)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
func contextThread(text string) string {
	for _, m := range threadHint.FindAllStringSubmatch(text, -1) {
		if canonical(m[1]) == "" && !strings.Contains(m[1], ":") {
			return m[1]
		}
	}
	return ""
}

// Keep a contiguous, bounded window. Strong request evidence extends a small
// baseline; another request/thread or a distant timestamp ends the window.
func automaticContext(event, before, after string, stamp int64) (string, string, string) {
	p, err := NewParser(nil)
	if err != nil {
		return before, after, "保留总体范围"
	}
	first := strings.SplitN(event, "\n", 2)[0]
	if len(first) > 64*1024 {
		first = first[:64*1024]
	}
	header, _ := p.Parse([]byte(first), true)
	id := contextIdentifier(event)
	thread := contextThread(first)
	pre, post := 3, 3
	reason := "相邻日志"
	if stackHint.MatchString(event) {
		pre, post = 6, 4
		reason = "堆栈与相邻日志"
	}
	if causalHint.MatchString(event) {
		pre, post = 12, 12
		reason = "异常提示与前后过程"
	}
	if id != "" {
		reason += " · 请求 ID"
	}
	choose := func(text string, reverse bool, baseline int) string {
		if text == "" {
			return ""
		}
		lines := strings.Split(text, "\n")
		type evidence struct {
			header        Header
			id, thread    string
			start, causal bool
		}
		entries := make([]evidence, len(lines))
		current := evidence{}
		for i, line := range lines {
			probe := line
			if len(probe) > 64*1024 {
				probe = probe[:64*1024]
			}
			h, ok := p.Parse([]byte(probe), true)
			if ok && !h.Attach {
				current = evidence{header: h, id: contextIdentifier(probe), thread: contextThread(probe), causal: causalHint.MatchString(probe)}
			}
			entries[i] = current
			entries[i].start = ok && !h.Attach
		}
		selected := -1
		for step := 0; step < len(lines); step++ {
			i := step
			if reverse {
				i = len(lines) - 1 - step
			}
			entry := entries[i]
			h, isHeader := entry.header, entry.start
			other, otherThread := entry.id, entry.thread
			if id != "" && other != "" && other != id {
				break
			}
			if h.HasTime && stamp != 0 && (h.Time > stamp+int64(30*time.Second) || h.Time < stamp-int64(30*time.Second)) {
				break
			}
			sameID := id != "" && other == id
			if !sameID && thread != "" && otherThread != "" && thread != otherThread {
				break
			}
			sameLogger := header.Logger != "" && h.Logger == header.Logger
			related := sameID || sameLogger && entry.causal
			if isHeader && !related && step >= baseline {
				break
			}
			if step < baseline || related {
				selected = i
			}
			if !reverse && isHeader && completionHint.MatchString(h.Message) {
				if sameID || sameLogger || step < baseline {
					selected = i
				}
				break
			}
		}
		if selected < 0 {
			return ""
		}
		if reverse {
			return strings.Join(lines[selected:], "\n")
		}
		return strings.Join(lines[:selected+1], "\n")
	}
	if stamp != 0 {
		reason += " · 时间边界 30 秒"
	} else {
		reason += " · 时间未知"
	}
	return choose(before, true, pre), choose(after, false, post), reason
}

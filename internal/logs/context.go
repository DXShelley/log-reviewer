package logs

import (
	"bufio"
	"bytes"
	"io"

	"regexp"
	"strings"
)

func findContextAnchor(f ReadFile, rec Record, keyword string, re *regexp.Regexp) (int64, int64, uint64, string, bool, bool, error) {
	reader := bufio.NewReaderSize(io.NewSectionReader(f, int64(rec.Offset), int64(rec.Length)), 64*1024)
	position, start, line := int64(rec.Offset), int64(rec.Offset), rec.Line
	needle := strings.ToLower(keyword)
	prefix := ""
	matched := false
	truncated := false
	preview := make([]byte, 0, 4096)
	for {
		b, err := reader.ReadSlice('\n')
		position += int64(len(b))
		if keyword != "" {
			lower := strings.ToLower(string(b))
			if strings.Contains(prefix+lower, needle) {
				matched = true
			}
			prefix += lower
			if len(prefix) > len(needle) {
				prefix = prefix[len(prefix)-len(needle):]
			}
		}
		n := len(b)
		if n > 2*1024*1024-len(preview) {
			n = 2*1024*1024 - len(preview)
			truncated = true
		}
		preview = append(preview, b[:n]...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return 0, 0, 0, "", false, false, err
		}
		if keyword == "" && re != nil && re.Match(bytes.TrimRight(preview, "\r\n")) {
			matched = true
		}
		if matched {
			return start, position, line, string(preview), truncated, true, nil
		}
		if err == io.EOF {
			return 0, 0, 0, "", false, false, nil
		}
		start = position
		line++
		preview = preview[:0]
		prefix = ""
		truncated = false
	}
}

// Grow reads by blocks; both directions have a bounded preview budget.
func readContext(f ReadFile, offset int64, count int, before bool) (string, bool, error) {
	if count == 0 {
		return "", false, nil
	}
	const budget = 2 * 1024 * 1024
	for size := 64 * 1024; ; size *= 2 {
		if size > budget {
			size = budget
		}
		start := offset
		if before {
			start = offset - int64(size)
			if start < 0 {
				start = 0
			}
			size = int(offset - start)
		}
		b := make([]byte, size)
		n, err := f.ReadAt(b, start)
		if err != nil && err != io.EOF {
			return "", false, err
		}
		b = b[:n]
		enough := bytes.Count(b, []byte{'\n'}) >= count+1
		if !before {
			enough = bytes.Count(b, []byte{'\n'}) >= count
		}
		boundary := err == io.EOF || before && start == 0
		if enough || boundary || size == budget {
			text := strings.TrimSuffix(string(b), "\n")
			lines := strings.Split(text, "\n")
			if len(b) == 0 {
				lines = nil
			}
			if before {
				if start > 0 && len(lines) > 0 {
					var previous [1]byte
					if _, err := f.ReadAt(previous[:], start-1); err != nil {
						return "", false, err
					}
					if previous[0] != '\n' {
						lines = lines[1:]
					}
				}
				if len(lines) > count {
					lines = lines[len(lines)-count:]
				}
			} else {
				if !boundary && len(lines) > 0 && !bytes.HasSuffix(b, []byte{'\n'}) {
					lines = lines[:len(lines)-1]
				}
				if len(lines) > count {
					lines = lines[:count]
				}
			}
			return strings.Join(lines, "\n"), !boundary && len(lines) < count, nil
		}
	}
}

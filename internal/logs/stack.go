package logs

import (
	"hash"
	"regexp"
	"strings"
)

var stackFrame = regexp.MustCompile(`^(?:at\s|File\s+["']|#\d+\s|\d+:\s)|[\\/\w.$-]+\.(?:java|kt|scala|py|js|ts|go|rs|cs|cpp|cc|c|h|rb|php|swift|ex|exs|dart|lua)(?::\d+|\(\d+)|\(.*\)(?:\s+\+0x[0-9a-f]+)?$`)
var stackAddress = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)

// Stream frame evidence separately from the bounded display preview.
func appendStack(h hash.Hash, text string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(ansi.ReplaceAllString(line, ""))
		if !stackFrame.MatchString(line) {
			continue
		}
		line = stackAddress.ReplaceAllString(line, "<address>")
		h.Write([]byte(strings.Join(strings.Fields(line), " ")))
		h.Write([]byte{'\n'})
	}
}

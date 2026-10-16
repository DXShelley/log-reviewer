package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// JSON configuration must be UTF-8; encoding/json otherwise replaces invalid bytes silently.
func ReadJSON(path string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(b) {
		return fmt.Errorf("%s: JSON must be UTF-8; convert the file to UTF-8 before importing", path)
	}
	return json.Unmarshal(b, value)
}

// Offsets always address the original bytes; decoding happens only after reads.
type encodedFile struct {
	ReadFile
	encoding string
}

func decodeText(encoding string, b []byte) []byte {
	if encoding == "GB18030" {
		decoded, _ := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
		return decoded
	}
	return bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
}

func fileText(f ReadFile, b []byte) []byte {
	if f, ok := f.(*encodedFile); ok {
		return decodeText(f.encoding, b)
	}
	return b
}

func textReader(f ReadFile, reader io.Reader) io.Reader {
	if f, ok := f.(*encodedFile); ok && f.encoding == "GB18030" {
		return transform.NewReader(reader, simplifiedchinese.GB18030.NewDecoder())
	}
	return reader
}

// Keep incomplete multibyte characters between bounded raw reads.
func chunkText(f ReadFile) func([]byte, bool) []byte {
	var pending []byte
	return func(b []byte, end bool) []byte {
		input := append(pending, b...)
		pending = nil
		if encoded, ok := f.(*encodedFile); ok && encoded.encoding == "GB18030" {
			out := make([]byte, len(input)*3+4)
			n, consumed, _ := simplifiedchinese.GB18030.NewDecoder().Transform(out, input, end)
			pending = append(pending, input[consumed:]...)
			return out[:n]
		}
		n := len(input)
		if !end {
			for start := max(0, n-3); start < n; start++ {
				if utf8.RuneStart(input[start]) && !utf8.FullRune(input[start:]) {
					n = start
					break
				}
			}
		}
		pending = append(pending, input[n:]...)
		return input[:n]
	}
}

func sourceEncoding(b []byte) string {
	// A sample may end inside a UTF-8 character.
	end := len(b)
	for start := max(0, end-3); start < end; start++ {
		if utf8.RuneStart(b[start]) && !utf8.FullRune(b[start:]) {
			b = b[:start]
			break
		}
	}
	if utf8.Valid(b) {
		return "UTF-8"
	}
	return "GB18030"
}

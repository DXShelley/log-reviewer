package logs

import (
	"bufio"
	"context"
	"errors"
	"io"
)

// ExportTime streams physical lines, retaining untimed continuation lines with
// the preceding timestamp. Untimed file prefixes are excluded.
func ExportTime(ctx context.Context, r *Report, source int, from, to string, w io.Writer) error {
	start, err := ParseBound(from)
	if err != nil {
		return err
	}
	end, err := ParseUpperBound(to)
	if err != nil {
		return err
	}
	if from == "" || to == "" || start > end {
		return errors.New("a valid start and end time are required")
	}
	if source < 0 || source >= len(r.Sources) {
		return errors.New("invalid source")
	}
	f, err := validatedFile(r, uint32(source))
	if err != nil {
		return err
	}
	defer f.Close()
	p, err := NewParser(nil)
	if err != nil {
		return err
	}
	reader := bufio.NewReaderSize(textReader(f, f), 64*1024)
	selected, continuation := false, false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, readErr := reader.ReadSlice('\n')
		if !continuation && len(b) > 0 {
			h, _ := p.Parse(b, true)
			if h.HasTime {
				selected = h.Time >= start && h.Time <= end
			}
		}
		if selected && len(b) > 0 {
			if _, err := w.Write(b); err != nil {
				return err
			}
		}
		continuation = readErr == bufio.ErrBufferFull
		if readErr == io.EOF {
			break
		}
		if readErr != nil && !continuation {
			return readErr
		}
	}
	// Reject snapshots changed while exporting, including appends.
	check, err := validatedFile(r, uint32(source))
	if err != nil {
		return err
	}
	return check.Close()
}

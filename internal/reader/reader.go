// Package reader provides a bufio.SplitFunc that safely handles unusually
// long or malformed (e.g. binary, newline-free) input: lines are capped at
// a maximum length, with the remainder of an over-long line discarded
// rather than buffered without bound, keeping memory use predictable
// regardless of what a misbehaving upstream process sends.
package reader

import (
	"bufio"
	"bytes"
)

// MaxLineBytes is the default cap on a single logical line before it is
// truncated. This is generous for real log lines while still bounding
// worst-case memory for one line.
const MaxLineBytes = 256 * 1024

const truncationMarker = " …[logquiet: line truncated]"

// BoundedLines returns a bufio.SplitFunc that emits lines up to maxLen
// bytes, appending truncationMarker and invoking onTruncate (if non-nil)
// for any line that exceeded it. The remaining bytes of an over-long line,
// up to the next newline, are discarded rather than returned as a separate
// spurious line.
func BoundedLines(maxLen int, onTruncate func()) bufio.SplitFunc {
	overflow := false

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
			line := bytes.TrimSuffix(data[:idx], []byte("\r"))
			adv := idx + 1
			if overflow {
				overflow = false
				return adv, nil, nil
			}
			return adv, copyTrim(line, maxLen, onTruncate), nil
		}

		if len(data) >= maxLen {
			if overflow {
				return len(data), nil, nil
			}
			overflow = true
			if onTruncate != nil {
				onTruncate()
			}
			out := make([]byte, 0, maxLen+len(truncationMarker))
			out = append(out, data[:maxLen]...)
			out = append(out, truncationMarker...)
			return maxLen, out, nil
		}

		if atEOF {
			if len(data) == 0 {
				return 0, nil, nil
			}
			return len(data), copyTrim(bytes.TrimSuffix(data, []byte("\r")), maxLen, onTruncate), nil
		}
		return 0, nil, nil
	}
}

func copyTrim(line []byte, maxLen int, onTruncate func()) []byte {
	if len(line) <= maxLen {
		out := make([]byte, len(line))
		copy(out, line)
		return out
	}
	if onTruncate != nil {
		onTruncate()
	}
	out := make([]byte, 0, maxLen+len(truncationMarker))
	out = append(out, line[:maxLen]...)
	out = append(out, truncationMarker...)
	return out
}

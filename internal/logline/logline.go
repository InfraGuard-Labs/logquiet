// Package logline extracts a severity level and a "content" string (the
// message body with any leading timestamp/level prefix removed, real
// values still intact) from one raw log line. Content is used both for
// display of first-seen events and as the input to structural
// normalization; Raw is preserved separately for anything that needs the
// exact original bytes.
package logline

import (
	"regexp"

	"github.com/azimsiddiqui/logquiet/internal/severity"
)

// leading timestamp shapes tried when no severity token was found at all
// (e.g. plain access-log style lines with no level marker). Each consumes
// at most one trailing separator character, not a greedy whitespace run:
// container runtimes (Docker/Kubernetes) prefix every line of raw output -
// including each line of a traceback - with an identical timestamp, so any
// indentation immediately following it is the traceback's own and must
// survive for multiline continuation detection.
var leadingTimestamp = []*regexp.Regexp{
	regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d{1,9})?(Z|[+-]\d{2}:?\d{2})?[ \t]?`),
	regexp.MustCompile(`^(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) {1,2}\d{1,2} \d{2}:\d{2}:\d{2}[ \t]?`),
	regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[^\]]*\][ \t]?`),
}

// Extract returns the detected severity (Unknown if none found) and the
// message content with the leading timestamp/level prefix stripped.
func Extract(raw string) (severity.Level, string) {
	if lvl, found, consumed := severity.Detect(raw); found {
		return lvl, raw[consumed:]
	}
	for _, re := range leadingTimestamp {
		if loc := re.FindStringIndex(raw); loc != nil && loc[0] == 0 {
			return severity.Unknown, raw[loc[1]:]
		}
	}
	return severity.Unknown, raw
}

// Package normalize turns a raw log message into a stable structural
// template by replacing high-cardinality variable substrings (timestamps,
// UUIDs, IP addresses, numeric IDs, durations, byte sizes, hashes, memory
// addresses, and similar generated identifiers) with class placeholders.
//
// The goal is that two lines which differ only in their variable payload
// normalize to the identical template:
//
//	10:00:01 User 10481 connected from 10.0.1.2
//	10:00:02 User 49210 connected from 10.0.1.8
//
//	-> [TIME] User [NUM] connected from [IP]
//
// Normalization intentionally favors precision over recall: a class is only
// substituted when the surrounding shape makes false collisions unlikely.
// See docs/TECHNICAL_METHOD.md for the rationale behind ordering and each
// pattern's design.
//
// Performance: Go's regexp (RE2) engine has real per-pattern cost even for
// an unanchored, non-matching scan - proportional to the number of
// alternatives simulated, not just whether anything matches
// (BenchmarkBisect makes this measurable per class). The five classes that
// benchmarking showed to be the most expensive relative to their simplicity
// - syslog-style month timestamps, durations, byte sizes, generic
// alphanumeric IDs, and bare numbers, all of which appear on a large
// fraction of real log lines - are hand-written byte scanners instead of
// regexes. The remaining, structurally intricate classes (ISO timestamps,
// dates, clock times, UUIDs, MAC/IPv6/IPv4 addresses, memory addresses,
// percentages, hex hashes) stay as one combined regex. A single left-to-
// right pass then interleaves the two: regex spans are matched first over
// the whole line, and the hand-written scanner fills in the gaps between
// them, so both tiers still respect the original documented class
// priority. See BenchmarkTemplate and docs/TECHNICAL_METHOD.md.
package normalize

import (
	"regexp"
	"strings"
)

// regexClasses lists the classes still matched via regex, in priority
// order (earlier wins when alternatives could match at the same start).
var regexClasses = []struct {
	name    string
	pattern string
	repl    string
}{
	{"iso8601", `\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:?\d{2})?\b`, "[TIMESTAMP]"},
	// Apache/nginx combined log format, e.g. [30/Aug/2026:03:01:00 +0000].
	// Listed before ipv6 so its "HH:MM:SS" portion is never misread as an
	// IPv6 address (both are colon-separated hex-looking groups).
	{"apachetime", `\[\d{1,2}/(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\]`, "[TIMESTAMP]"},
	{"date", `\b\d{4}-\d{2}-\d{2}\b`, "[DATE]"},
	{"dateslash", `\b\d{1,4}/\d{1,2}/\d{1,4}\b`, "[DATE]"},
	{"clock", `\b\d{1,2}:\d{2}:\d{2}(?:\.\d{1,9})?\b`, "[TIME]"},
	{"uuid", `\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`, "[UUID]"},
	{"mac", `\b(?:[0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}\b`, "[MAC]"},
	{"ipv6", `\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{0,4}(?::\d{1,5})?\b`, "[IPV6]"},
	{"ipv4port", `\b(?:\d{1,3}\.){3}\d{1,3}:\d{1,5}\b`, "[IP]:[PORT]"},
	{"ipv4", `\b(?:\d{1,3}\.){3}\d{1,3}\b`, "[IP]"},
	{"memaddr", `\b0x[0-9a-fA-F]{4,}\b`, "[ADDR]"},
	{"percent", `\d+(?:\.\d+)?%`, "[PCT]%"},
	{"hexhash", `\b[0-9a-fA-F]{12,64}\b`, "[HASH]"},
}

// alnumIDMinLen guards the hand-rolled alphanumeric-ID scan so short tokens
// like "db2", "ipv6", "utf8" (common non-variable words) are not mistaken
// for generated identifiers.
const alnumIDMinLen = 6

var durationUnits = []string{"seconds", "minutes", "hours", "secs", "mins", "hrs", "sec", "min", "hr", "ms", "us", "ns", "s", "m", "h"}
var byteUnits = []string{"KiB", "MiB", "GiB", "TiB", "KB", "MB", "GB", "TB", "B"}
var months = map[string]bool{
	"Jan": true, "Feb": true, "Mar": true, "Apr": true, "May": true, "Jun": true,
	"Jul": true, "Aug": true, "Sep": true, "Oct": true, "Nov": true, "Dec": true,
}

type classifier struct {
	name string
	full *regexp.Regexp
	repl string
}

var (
	spanFinder  *regexp.Regexp
	classifiers []classifier
)

func init() {
	classifiers = make([]classifier, len(regexClasses))
	var b strings.Builder
	for i, c := range regexClasses {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(`(?:`)
		b.WriteString(c.pattern)
		b.WriteString(`)`)
		classifiers[i] = classifier{name: c.name, full: regexp.MustCompile(`^(?:` + c.pattern + `)$`), repl: c.repl}
	}
	spanFinder = regexp.MustCompile(b.String())
}

// Template normalizes a single-line message into its structural form.
func Template(msg string) string {
	spans := spanFinder.FindAllStringIndex(msg, -1)

	var sb strings.Builder
	sb.Grow(len(msg))
	n := len(msg)
	i, si := 0, 0
	for i < n {
		if si < len(spans) && spans[si][0] == i {
			start, end := spans[si][0], spans[si][1]
			sb.WriteString(classify(msg[start:end]))
			i = end
			si++
			continue
		}
		gapEnd := n
		if si < len(spans) {
			gapEnd = spans[si][0]
		}
		i = scanOne(msg, i, gapEnd, &sb)
	}
	return sb.String()
}

// classify returns the replacement for a span already known to match one of
// regexClasses, tried in the same priority order.
func classify(tok string) string {
	for _, c := range classifiers {
		if c.full.MatchString(tok) {
			return c.repl
		}
	}
	return tok
}

// scanOne advances the hand-rolled scanner by exactly one token (or one
// byte, if nothing matches) within [i, limit) and writes the result to sb,
// returning the new position. limit is the start of the next regex-claimed
// span (or end of string), so the hand-rolled scan never reads into
// territory already assigned to a higher-priority regex class - safe
// because none of the five hand-rolled classes can textually overlap a
// timestamp/UUID/IP/hash/percent match (see package doc).
func scanOne(msg string, i, limit int, sb *strings.Builder) int {
	c := msg[i]

	if isAlpha(c) {
		if end, ok := trySyslogMonth(msg, i, limit); ok {
			sb.WriteString("[TIMESTAMP]")
			return end
		}
		end := i + 1
		for end < limit && isIdentByte(msg[end]) {
			end++
		}
		if hasDigit(msg[i:end]) && end-i >= alnumIDMinLen && boundaryOK(msg, i, end) {
			sb.WriteString("[ID]")
			return end
		}
		sb.WriteString(msg[i:end])
		return end
	}

	if isDigit(c) {
		return scanNumeric(msg, i, limit, sb)
	}

	sb.WriteByte(c)
	return i + 1
}

func scanNumeric(msg string, i, limit int, sb *strings.Builder) int {
	start := i
	end := i
	for end < limit && isDigit(msg[end]) {
		end++
	}
	if end < limit && msg[end] == '.' && end+1 < limit && isDigit(msg[end+1]) {
		end++
		for end < limit && isDigit(msg[end]) {
			end++
		}
	}
	numEnd := end

	if !boundaryBefore(msg, start) {
		// Not at a word boundary (e.g. part of an identifier already
		// handled elsewhere) - degrade to copying the digits verbatim.
		sb.WriteString(msg[start:numEnd])
		return numEnd
	}

	if uEnd, ok := matchUnit(msg, numEnd, limit, durationUnits); ok {
		sb.WriteString("[DURATION]")
		return uEnd
	}
	sizeStart := numEnd
	if sizeStart < limit && msg[sizeStart] == ' ' {
		sizeStart++
	}
	if uEnd, ok := matchUnit(msg, sizeStart, limit, byteUnits); ok {
		sb.WriteString("[SIZE]")
		return uEnd
	}

	// Digit-led alphanumeric identifier, e.g. "7c9f6d8b4d".
	if numEnd < limit && isIdentByte(msg[numEnd]) {
		wordEnd := numEnd
		for wordEnd < limit && isIdentByte(msg[wordEnd]) {
			wordEnd++
		}
		if hasAlpha(msg[start:wordEnd]) && wordEnd-start >= alnumIDMinLen && boundaryOK(msg, start, wordEnd) {
			sb.WriteString("[ID]")
			return wordEnd
		}
		sb.WriteString(msg[start:wordEnd])
		return wordEnd
	}

	if !boundaryAfter(msg, numEnd) {
		sb.WriteString(msg[start:numEnd])
		return numEnd
	}
	sb.WriteString("[NUM]")
	return numEnd
}

// matchUnit checks whether one of units appears at position p (case
// sensitive, longest-listed-first so e.g. "ms" is tried before "m"), and
// that a word boundary follows it.
func matchUnit(msg string, p, limit int, units []string) (int, bool) {
	for _, u := range units {
		e := p + len(u)
		if e <= limit && msg[p:e] == u && boundaryAfter(msg, e) {
			return e, true
		}
	}
	return p, false
}

func trySyslogMonth(msg string, i, limit int) (int, bool) {
	if i+3 > limit || !months[msg[i:i+3]] {
		return 0, false
	}
	p := i + 3
	if p >= limit || msg[p] != ' ' {
		return 0, false
	}
	p++
	if p < limit && msg[p] == ' ' {
		p++
	}
	dayStart := p
	for p < limit && isDigit(msg[p]) {
		p++
	}
	if p == dayStart || p-dayStart > 2 {
		return 0, false
	}
	if p >= limit || msg[p] != ' ' {
		return 0, false
	}
	p++
	// HH:MM:SS
	if p+8 > limit {
		return 0, false
	}
	clock := msg[p : p+8]
	if !isDigit(clock[0]) || !isDigit(clock[1]) || clock[2] != ':' ||
		!isDigit(clock[3]) || !isDigit(clock[4]) || clock[5] != ':' ||
		!isDigit(clock[6]) || !isDigit(clock[7]) {
		return 0, false
	}
	p += 8
	if !boundaryBefore(msg, i) || !boundaryAfter(msg, p) {
		return 0, false
	}
	return p, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isWordByte(c byte) bool {
	return isAlpha(c) || isDigit(c) || c == '_'
}
func isIdentByte(c byte) bool { return isWordByte(c) || c == '-' }

func hasDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if isDigit(s[i]) {
			return true
		}
	}
	return false
}

func hasAlpha(s string) bool {
	for i := 0; i < len(s); i++ {
		if isAlpha(s[i]) {
			return true
		}
	}
	return false
}

func boundaryBefore(msg string, i int) bool {
	return i == 0 || !isWordByte(msg[i-1])
}

func boundaryAfter(msg string, j int) bool {
	return j >= len(msg) || !isWordByte(msg[j])
}

func boundaryOK(msg string, start, end int) bool {
	return boundaryBefore(msg, start) && boundaryAfter(msg, end)
}

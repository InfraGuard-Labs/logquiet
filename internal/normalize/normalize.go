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
//	-> [TIME] User [ID] connected from [IP]
//
// Normalization intentionally favors precision over recall: a class is only
// substituted when the surrounding shape makes false collisions unlikely.
// See docs/TECHNICAL_METHOD.md for the rationale behind ordering and each
// pattern's design.
package normalize

import "regexp"

// rule is applied in sequence; order matters because later rules only see
// what earlier rules left behind (e.g. bare-number substitution must run
// last, after every pattern that itself contains digits).
type rule struct {
	name string
	re   *regexp.Regexp
	repl string
}

var rules = []rule{
	// --- timestamps & dates (most specific first) ---
	{"iso8601", regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d{1,9})?(Z|[+-]\d{2}:?\d{2})?\b`), "[TIMESTAMP]"},
	{"date", regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`), "[DATE]"},
	{"date-slash", regexp.MustCompile(`\b\d{1,4}/\d{1,2}/\d{1,4}\b`), "[DATE]"},
	{"clock", regexp.MustCompile(`\b\d{1,2}:\d{2}:\d{2}(\.\d{1,9})?\b`), "[TIME]"},
	{"syslog-month", regexp.MustCompile(`\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) {1,2}\d{1,2} \d{2}:\d{2}:\d{2}\b`), "[TIMESTAMP]"},

	// --- identifiers with fixed, unmistakable shapes ---
	{"uuid", regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), "[UUID]"},
	{"mac", regexp.MustCompile(`\b(?:[0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}\b`), "[MAC]"},
	{"ipv6", regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{0,4}(?::\d{1,5})?\b`), "[IPV6]"},
	{"ipv4-port", regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}:\d{1,5}\b`), "[IP]:[PORT]"},
	{"ipv4", regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), "[IP]"},
	{"memaddr", regexp.MustCompile(`\b0x[0-9a-fA-F]{4,}\b`), "[ADDR]"},

	// --- durations & sizes (unit-suffixed numbers, before bare numbers) ---
	{"duration", regexp.MustCompile(`\b\d+(\.\d+)?(ms|us|µs|ns|s|sec|secs|seconds|m|min|mins|minutes|h|hr|hrs|hours)\b`), "[DURATION]"},
	{"bytesize", regexp.MustCompile(`\b\d+(\.\d+)?\s?(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)\b`), "[SIZE]"},
	{"percent", regexp.MustCompile(`\b\d+(\.\d+)?%`), "[PCT]%"},

	// --- long hex hashes (git SHAs, checksums, request/trace ids) ---
	{"hexhash", regexp.MustCompile(`\b[0-9a-fA-F]{12,64}\b`), "[HASH]"},

	// --- generic alphanumeric identifiers: at least one letter AND one
	// digit, length >= 6, e.g. req-8f3ac2, trace_4e5f9, pod-7c9f6d8b4d ---
	{"alnum-id", regexp.MustCompile(`\b(?:[A-Za-z][A-Za-z0-9_-]*\d[A-Za-z0-9_-]*|[0-9][A-Za-z0-9_-]*[A-Za-z][A-Za-z0-9_-]*)\b`), "[ID]"},

	// --- bare numbers (applied last: everything with digits and extra
	// shape has already been consumed by a more specific rule above) ---
	{"number", regexp.MustCompile(`-?\b\d+\b`), "[NUM]"},
}

// alnumIDMinLen guards the alnum-id rule so short tokens like "db2", "ipv6",
// "utf8", "sha256" (common non-variable words) are not mistaken for
// generated identifiers.
const alnumIDMinLen = 6

// Template normalizes a single-line message into its structural form.
func Template(msg string) string {
	out := msg
	for _, r := range rules {
		if r.name == "alnum-id" {
			out = r.re.ReplaceAllStringFunc(out, func(tok string) string {
				if len(tok) < alnumIDMinLen {
					return tok
				}
				return r.repl
			})
			continue
		}
		out = r.re.ReplaceAllString(out, r.repl)
	}
	return out
}

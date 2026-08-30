// Package severity classifies log lines into a canonical severity level
// without assuming any particular bracket, delimiter, or logging framework.
package severity

import "strings"

// Level is a canonical, ordered log severity.
type Level int

const (
	Unknown Level = iota
	Trace
	Debug
	Info
	Notice
	Warn
	Error
	Critical
	Fatal
)

func (l Level) String() string {
	switch l {
	case Trace:
		return "TRACE"
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Notice:
		return "NOTICE"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	case Critical:
		return "CRITICAL"
	case Fatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// aliases maps every recognized textual token (already upper-cased) to its
// canonical Level. Order of insertion does not matter; lookups are exact.
var aliases = map[string]Level{
	"TRACE":       Trace,
	"TRC":         Trace,
	"DEBUG":       Debug,
	"DBG":         Debug,
	"INFO":        Info,
	"INFORMATION": Info,
	"NOTICE":      Notice,
	"WARN":        Warn,
	"WARNING":     Warn,
	"ERROR":       Error,
	"ERR":         Error,
	"CRITICAL":    Critical,
	"CRIT":        Critical,
	"ALERT":       Critical,
	"FATAL":       Fatal,
	"PANIC":       Fatal,
	"EMERG":       Fatal,
	"EMERGENCY":   Fatal,
}

// maxScanLen bounds how far into a line we look for a severity token, so a
// pathologically long line never makes classification expensive.
const maxScanLen = 200

// maxTokens bounds how many candidate tokens (timestamp fields, hostnames,
// pids, ...) we walk past before giving up. Real formats put the level
// within the first handful of fields.
const maxTokens = 8

// Detect scans the given text for a recognizable severity token, tolerating
// an arbitrary prefix (timestamp, hostname, pid, service name - as in
// syslog/journalctl output). It matches bracketed forms like "[ERROR]"
// case-insensitively, and bare forms like "ERROR:" or "ERROR " but only when
// the token is not immediately preceded by '/' or '.' (which would indicate
// a path segment or qualified name such as "/info" or "com.foo.Info" rather
// than a genuine level marker).
//
// It returns the detected level, whether anything was found, and the byte
// index immediately after the consumed token (including any trailing
// separator such as ':' or a closing bracket and following spaces) so the
// caller can treat the remainder as the message body.
func Detect(text string) (level Level, found bool, consumed int) {
	scan := text
	if len(scan) > maxScanLen {
		scan = scan[:maxScanLen]
	}

	i := 0
	tokens := 0
	for i < len(scan) && tokens < maxTokens {
		for i < len(scan) && isSkippable(scan[i]) {
			i++
		}
		if i >= len(scan) {
			break
		}

		bracketed := scan[i] == '['
		prevRune := byte(0)
		if i > 0 {
			prevRune = scan[i-1]
		}
		start := i
		if bracketed {
			start = i + 1
		}

		j := start
		for j < len(scan) && isWordChar(scan[j]) {
			j++
		}
		if j == start {
			i++
			continue
		}
		tokens++

		word := strings.ToUpper(scan[start:j])
		if lvl, ok := aliases[word]; ok && (bracketed || (prevRune != '/' && prevRune != '.')) {
			end := j
			if bracketed && end < len(scan) && scan[end] == ']' {
				end++
			}
			if bracketed || end >= len(scan) || scan[end] == ':' || scan[end] == ' ' || scan[end] == '\t' {
				// Consume at most one separator (an optional ':' plus one
				// space/tab) after the token. Further whitespace is left
				// intact in the returned content, since it may be
				// meaningful indentation (a multiline continuation signal)
				// rather than a mere field separator.
				if end < len(scan) && scan[end] == ':' {
					end++
				}
				if end < len(scan) && (scan[end] == ' ' || scan[end] == '\t') {
					end++
				}
				return lvl, true, end
			}
		}
		i = j
	}
	return Unknown, false, 0
}

func isSkippable(b byte) bool {
	return b == ' ' || b == '\t' || b == ':' || b == ',' || b == '-' || b == '_' ||
		(b >= '0' && b <= '9')
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// Rank returns an integer ordering suitable for threshold comparisons.
// Unknown is treated as equivalent to Info for suppression purposes: it is
// neither specially protected nor specially suppressed.
func (l Level) Rank() int {
	if l == Unknown {
		return int(Info)
	}
	return int(l)
}

// Icon returns a short glyph used by the terminal renderer.
func (l Level) Icon() string {
	switch l {
	case Trace, Debug:
		return "·"
	case Info, Notice:
		return " "
	case Warn:
		return "⚠"
	case Error:
		return "✖"
	case Critical, Fatal:
		return "🚨"
	default:
		return " "
	}
}

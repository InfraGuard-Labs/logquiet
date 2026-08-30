// Package multiline groups raw input lines that belong to a single logical
// event - most importantly stack traces and tracebacks - so they are
// normalized, fingerprinted, counted, and rendered as one unit rather than
// as dozens of unrelated one-line "events".
//
// The detection is heuristic and line-shape based (no per-language parser):
// it recognizes the common shapes produced by Python, Java/JVM, Node.js and
// Go, plus a generic "indentation continues the previous event" rule that
// covers most other multiline formats. It intentionally does not attempt to
// be a complete parser for every possible stack-trace dialect; see
// docs/ARCHITECTURE.md for known limitations.
package multiline

import "regexp"

// MaxBlockLines bounds how many raw lines a single logical event may absorb,
// guaranteeing bounded memory even against an adversarial or malformed
// stream that never stops "looking like" a continuation.
const MaxBlockLines = 500

var (
	rePyTraceback   = regexp.MustCompile(`Traceback \(most recent call last\):\s*$`)
	rePyExcSummary  = regexp.MustCompile(`^[A-Za-z_][\w.]*(Error|Exception|Warning|Interrupt|SystemExit)\b`)
	rePyFileLine    = regexp.MustCompile(`^\s*File "[^"]*", line \d+`)
	reJavaAt        = regexp.MustCompile(`^\s*at\s+\S+\(`)
	reCausedBy      = regexp.MustCompile(`^\s*Caused by:`)
	reMoreFrames    = regexp.MustCompile(`^\s*\.\.\.\s*\d+\s*more\s*$`)
	reGoPanicHeader = regexp.MustCompile(`^panic:`)
	reGoroutineHead = regexp.MustCompile(`^goroutine \d+ \[`)
	reGoCreatedBy   = regexp.MustCompile(`^created by\b`)
	reGoFrameCall   = regexp.MustCompile(`^[\w./*()\[\]{}, ]+\(.*\)\s*$`)
	reGoFrameFile   = regexp.MustCompile(`^\s+\S+\.go:\d+`)
	reExitStatus    = regexp.MustCompile(`^exit status \d+\s*$`)
)

type mode int

const (
	modeGeneric mode = iota
	modePyTraceback
	modeGoPanic
)

// Block is one assembled logical event: one or more raw input lines plus
// their parallel "content" form (severity/timestamp prefix stripped, real
// values intact) used for normalization and first-seen display.
type Block struct {
	Lines    []string
	Contents []string
}

// Assembler consumes lines one at a time and emits completed Blocks. Feed
// takes both the raw line (preserved verbatim in the output Block) and its
// content (used to decide continuation, since a prefix such as
// "2026-08-30 03:01:07 [ERROR] " repeated on every line of a traceback
// would otherwise defeat indentation-based continuation detection).
type Assembler struct {
	open     bool
	mode     mode
	lines    []string
	contents []string
}

// Feed processes one raw input line and its content. If it starts a new
// logical event, any previously open block is completed and returned along
// with ok=true, and the new line becomes the start of the next (now open)
// block. If the line extends the currently open block, Feed returns
// ok=false and nothing is emitted yet.
func (a *Assembler) Feed(line, content string) (completed Block, ok bool) {
	if !a.open {
		a.start(line, content)
		return Block{}, false
	}

	if a.continuesBlock(content) {
		a.lines = append(a.lines, line)
		a.contents = append(a.contents, content)
		if len(a.lines) >= MaxBlockLines {
			return a.flush(), true
		}
		return Block{}, false
	}

	completed = a.flush()
	a.start(line, content)
	return completed, true
}

// Flush completes and returns any currently open block (used at EOF).
func (a *Assembler) Flush() (Block, bool) {
	if !a.open {
		return Block{}, false
	}
	return a.flush(), true
}

func (a *Assembler) start(line, content string) {
	a.open = true
	a.lines = []string{line}
	a.contents = []string{content}
	switch {
	// "panic:" is checked against the raw line, not content: severity
	// detection treats the bare word "panic" as a FATAL-level token like
	// any other and strips it, but here it is also a structural marker
	// that must survive for Go-panic mode detection.
	case reGoPanicHeader.MatchString(line):
		a.mode = modeGoPanic
	case rePyTraceback.MatchString(content):
		a.mode = modePyTraceback
	default:
		a.mode = modeGeneric
	}
}

func (a *Assembler) flush() Block {
	b := Block{Lines: a.lines, Contents: a.contents}
	a.open = false
	a.lines = nil
	a.contents = nil
	a.mode = modeGeneric
	return b
}

// looksIndented reports whether line begins with indentation substantial
// enough to be a genuine multiline continuation, as opposed to a single
// leftover space from severity-tag column alignment (e.g. many logging
// frameworks pad "INFO" with a trailing space so it lines up with "ERROR");
// see logline.Extract, which strips only one separator character per level
// tag and deliberately leaves further padding in place for this check to
// reject. A single leading tab is also accepted unconditionally, since a
// tab is essentially never used merely as alignment padding.
func looksIndented(line string) bool {
	if len(line) == 0 {
		return false
	}
	if line[0] == '\t' {
		return len(line) > 1
	}
	if line[0] == ' ' && len(line) > 1 && line[1] == ' ' {
		i := 2
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return i < len(line)
	}
	return false
}

func (a *Assembler) continuesBlock(line string) bool {
	if line == "" {
		// A blank line separates events, except immediately inside a Go
		// panic header where "panic: ...\n\ngoroutine 1 [running]:" is
		// normal shape.
		return a.mode == modeGoPanic && len(a.lines) < 3
	}

	if looksIndented(line) {
		return true
	}
	if reJavaAt.MatchString(line) || reCausedBy.MatchString(line) || reMoreFrames.MatchString(line) {
		return true
	}

	switch a.mode {
	case modePyTraceback:
		if rePyExcSummary.MatchString(line) || rePyFileLine.MatchString(line) {
			return true
		}
	case modeGoPanic:
		if reGoroutineHead.MatchString(line) || reGoCreatedBy.MatchString(line) ||
			reGoFrameFile.MatchString(line) || reExitStatus.MatchString(line) {
			return true
		}
		if reGoFrameCall.MatchString(line) {
			return true
		}
	}
	return false
}

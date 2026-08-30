// Package render turns pipeline decisions into terminal output. It supports
// four output modes:
//
//   - TTY (default, stdout is a terminal): colorized text.
//   - Plain (--plain, or automatically when stdout is not a terminal):
//     no color.
//   - No-color (--no-color): like TTY but without SGR color codes.
//   - JSON (--json): one NDJSON object per decision.
//
// Repeat handling: a brand-new structural pattern is always shown in full,
// immediately, with its real values. Every later occurrence of that same
// pattern - whether or not other, different patterns are interleaved in
// between, which is the normal case for real logs (a restart loop cycling
// through several distinct messages, or several services' output merged
// together) - is accumulated into a per-pattern counter and flushed as a
// compact summary line on a short, bounded cadence, rather than either
// reprinting the full line every time or waiting for that exact pattern to
// repeat back-to-back. This is a deliberate simplification versus
// cursor-repositioning "redraw the same terminal line in place" tricks:
// periodic summarization is simple to reason about, is correct across
// terminal multiplexers/resizing, and generalizes to many concurrently
// recurring patterns, at the cost of a small bounded delay before a count
// visibly updates. See docs/ARCHITECTURE.md.
package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"syscall"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/severity"
)

// Options configures a Renderer.
type Options struct {
	IsTTY   bool
	Plain   bool
	NoColor bool
	JSON    bool

	// FlushInterval is how often an accumulating pattern's counter is
	// flushed as a summary line.
	FlushInterval time.Duration
	// ProtectedFlushInterval is used instead of FlushInterval for
	// severities at/above ProtectRank, so important classes of event
	// surface their running counts sooner.
	ProtectedFlushInterval time.Duration
	ProtectRank            int
}

// DefaultOptions returns sensible defaults; callers override IsTTY/Plain/
// NoColor/JSON based on flags and terminal detection.
func DefaultOptions() Options {
	return Options{
		FlushInterval:          2 * time.Second,
		ProtectedFlushInterval: 500 * time.Millisecond,
		ProtectRank:            int(severity.Error),
	}
}

// Event carries everything a renderer needs to display a brand-new pattern.
type Event struct {
	Fingerprint fingerprint.ID
	Severity    severity.Level
	Template    string
	RawLines    []string
	IsNew       bool
}

// Anomaly carries the data needed to render a frequency-spike banner.
type Anomaly struct {
	Fingerprint    fingerprint.ID
	Severity       severity.Level
	Template       string
	RawLines       []string
	BaselinePerMin float64
	CurrentPerMin  float64
	// Bootstrap is true when BaselinePerMin is an assumed value (no real
	// history existed for this pattern yet), not one learned from its own
	// observed behavior. Rendered with different wording so an assumed
	// number is never presented as if it were measured.
	Bootstrap bool
}

type pending struct {
	seq      uint64
	severity severity.Level
	template string
	count    uint64
	since    time.Time
}

// Renderer accumulates repeat counts per structural pattern and flushes
// them on a bounded cadence. It is not safe for concurrent use.
type Renderer struct {
	w    io.Writer
	opts Options

	pending   map[fingerprint.ID]*pending
	seq       uint64
	lastCheck time.Time
}

// New creates a Renderer writing to w.
func New(w io.Writer, opts Options) *Renderer {
	return &Renderer{w: &safeWriter{w: w}, opts: opts, pending: make(map[fingerprint.ID]*pending)}
}

// Broken reports whether the destination writer has been detected as a
// closed pipe. Once true, all further calls are cheap no-ops and the
// caller should stop reading input for output purposes.
func (r *Renderer) Broken() bool {
	sw, ok := r.w.(*safeWriter)
	return ok && sw.broken
}

// safeWriter wraps the destination writer so a closed downstream pipe
// (e.g. `logquiet | head`) becomes a recorded, checkable condition instead
// of a panic or a torrent of repeated write errors.
type safeWriter struct {
	w      io.Writer
	broken bool
}

func (s *safeWriter) Write(p []byte) (int, error) {
	if s.broken {
		return len(p), nil
	}
	n, err := s.w.Write(p)
	if err != nil && isBrokenPipe(err) {
		s.broken = true
	}
	return n, nil
}

func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)
}

// ANSI helpers.
const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiBold   = "\x1b[1m"
)

func (r *Renderer) color(lvl severity.Level) (prefix, reset string) {
	if r.opts.NoColor || r.opts.Plain || !r.opts.IsTTY || r.opts.JSON {
		return "", ""
	}
	switch lvl {
	case severity.Trace, severity.Debug:
		return ansiDim, ansiReset
	case severity.Warn:
		return ansiYellow, ansiReset
	case severity.Error:
		return ansiRed, ansiReset
	case severity.Critical, severity.Fatal:
		return ansiBold + ansiRed, ansiReset
	default:
		return "", ""
	}
}

// jsonLine is the stable NDJSON schema for --json mode.
type jsonLine struct {
	Type           string   `json:"type"`
	Fingerprint    string   `json:"fingerprint,omitempty"`
	Severity       string   `json:"severity,omitempty"`
	Template       string   `json:"template,omitempty"`
	Lines          []string `json:"lines,omitempty"`
	Count          uint64   `json:"count,omitempty"`
	New            bool     `json:"new,omitempty"`
	BaselinePerMin float64  `json:"baseline_per_min,omitempty"`
	CurrentPerMin  float64  `json:"current_per_min,omitempty"`
	Bootstrap      bool     `json:"bootstrap,omitempty"`
}

func (r *Renderer) emitJSON(v jsonLine) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(r.w, string(b))
}

// Emit displays a brand-new pattern's first occurrence in full, with its
// real (non-normalized) values.
func (r *Renderer) Emit(evt Event) {
	if r.opts.JSON {
		r.emitJSON(jsonLine{
			Type:        "event",
			Fingerprint: fpStr(evt.Fingerprint),
			Severity:    evt.Severity.String(),
			Template:    evt.Template,
			Lines:       evt.RawLines,
			New:         evt.IsNew,
			Count:       1,
		})
		return
	}
	prefix, reset := r.color(evt.Severity)
	icon := evt.Severity.Icon()
	for i, line := range evt.RawLines {
		if i == 0 {
			if evt.Severity == severity.Unknown {
				_, _ = fmt.Fprintf(r.w, "%s%s%s\n", prefix, line, reset)
			} else {
				_, _ = fmt.Fprintf(r.w, "%s%s %s %s%s\n", prefix, icon, evt.Severity.String(), line, reset)
			}
		} else {
			_, _ = fmt.Fprintf(r.w, "%s  %s%s\n", prefix, line, reset)
		}
	}
	_, _ = fmt.Fprintln(r.w)
}

// Accumulate registers another occurrence of an already-seen pattern. It
// does not print anything itself; Tick decides when accumulated counts are
// flushed.
func (r *Renderer) Accumulate(fp fingerprint.ID, lvl severity.Level, template string, now time.Time) {
	p, ok := r.pending[fp]
	if !ok {
		r.seq++
		p = &pending{seq: r.seq, severity: lvl, template: template, since: now}
		r.pending[fp] = p
	}
	p.count++
}

// interval returns the flush cadence for a given severity.
func (r *Renderer) interval(lvl severity.Level) time.Duration {
	if lvl.Rank() >= r.opts.ProtectRank {
		return r.opts.ProtectedFlushInterval
	}
	return r.opts.FlushInterval
}

// Tick flushes any accumulating patterns whose interval has elapsed. It is
// cheap to call frequently: the common case is a single time comparison.
func (r *Renderer) Tick(now time.Time) {
	if len(r.pending) == 0 {
		return
	}
	minInterval := r.opts.ProtectedFlushInterval
	if r.opts.FlushInterval < minInterval {
		minInterval = r.opts.FlushInterval
	}
	if !r.lastCheck.IsZero() && now.Sub(r.lastCheck) < minInterval/4 {
		return
	}
	r.lastCheck = now

	var due []fingerprint.ID
	for fp, p := range r.pending {
		if now.Sub(p.since) >= r.interval(p.severity) {
			due = append(due, fp)
		}
	}
	sort.Slice(due, func(i, j int) bool { return r.pending[due[i]].seq < r.pending[due[j]].seq })
	for _, fp := range due {
		r.flushOne(fp, now)
	}
}

// FlushFingerprint immediately flushes one pattern's pending counter, if
// any, regardless of whether its interval has elapsed. Used before an
// anomaly banner so the running count leading up to it is visible first.
func (r *Renderer) FlushFingerprint(fp fingerprint.ID, now time.Time) {
	if _, ok := r.pending[fp]; ok {
		r.flushOne(fp, now)
	}
}

// FlushAll immediately flushes every accumulating pattern. Call at EOF/shutdown.
func (r *Renderer) FlushAll(now time.Time) {
	var all []fingerprint.ID
	for fp := range r.pending {
		all = append(all, fp)
	}
	sort.Slice(all, func(i, j int) bool { return r.pending[all[i]].seq < r.pending[all[j]].seq })
	for _, fp := range all {
		r.flushOne(fp, now)
	}
}

func (r *Renderer) flushOne(fp fingerprint.ID, now time.Time) {
	p := r.pending[fp]
	delete(r.pending, fp)
	if p == nil || p.count == 0 {
		return
	}
	if r.opts.JSON {
		r.emitJSON(jsonLine{Type: "repeat_summary", Fingerprint: fpStr(fp), Severity: p.severity.String(), Template: p.template, Count: p.count})
		return
	}
	prefix, reset := r.color(p.severity)
	icon := p.severity.Icon()
	if p.severity == severity.Unknown {
		_, _ = fmt.Fprintf(r.w, "%s%s%s\n", prefix, p.template, reset)
	} else {
		_, _ = fmt.Fprintf(r.w, "%s%s %s %s%s\n", prefix, icon, p.severity.String(), p.template, reset)
	}
	_, _ = fmt.Fprintf(r.w, "%s  × %d%s\n\n", prefix, p.count, reset)
}

// Finalize flushes any remaining accumulated counters. Call at EOF/shutdown.
func (r *Renderer) Finalize(now time.Time) { r.FlushAll(now) }

// EmitAnomaly prints a frequency-spike banner. This is always shown in
// full immediately; anomalies are never throttled or deferred. Any pending
// counter for the same pattern is flushed first so context is not lost.
func (r *Renderer) EmitAnomaly(a Anomaly, now time.Time) {
	r.FlushFingerprint(a.Fingerprint, now)

	if r.opts.JSON {
		r.emitJSON(jsonLine{
			Type:           "anomaly",
			Fingerprint:    fpStr(a.Fingerprint),
			Severity:       a.Severity.String(),
			Template:       a.Template,
			Lines:          a.RawLines,
			BaselinePerMin: a.BaselinePerMin,
			CurrentPerMin:  a.CurrentPerMin,
			Bootstrap:      a.Bootstrap,
		})
		return
	}

	prefix, reset := r.color(severity.Critical)
	summary := a.Template
	if len(a.RawLines) > 0 {
		summary = a.RawLines[0]
	}
	if a.Bootstrap {
		// No real history exists for this pattern yet; BaselinePerMin is
		// an assumed value, not something observed. Say so plainly rather
		// than presenting it as a measured baseline.
		_, _ = fmt.Fprintf(r.w, "%s\U0001F6A8 FREQUENCY SPIKE (new pattern)%s\n", prefix, reset)
		_, _ = fmt.Fprintf(r.w, "%s   %s%s\n", prefix, summary, reset)
		_, _ = fmt.Fprintf(r.w, "%s   no prior history - already occurring at %s/min%s\n", prefix, rate(a.CurrentPerMin), reset)
	} else {
		_, _ = fmt.Fprintf(r.w, "%s\U0001F6A8 FREQUENCY SPIKE%s\n", prefix, reset)
		_, _ = fmt.Fprintf(r.w, "%s   %s%s\n", prefix, summary, reset)
		_, _ = fmt.Fprintf(r.w, "%s   baseline: %s/min%s\n", prefix, rate(a.BaselinePerMin), reset)
		_, _ = fmt.Fprintf(r.w, "%s   current:  %s/min%s\n", prefix, rate(a.CurrentPerMin), reset)
	}
	_, _ = fmt.Fprintln(r.w)
}

func rate(v float64) string {
	if v < 10 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.0f", v)
}

func fpStr(fp fingerprint.ID) string {
	return fmt.Sprintf("%016x", uint64(fp))
}

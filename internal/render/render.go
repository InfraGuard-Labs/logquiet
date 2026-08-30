// Package render turns pipeline decisions into terminal output. It supports
// four output modes:
//
//   - TTY (default, stdout is a terminal): colorized, with the repeat
//     counter for an in-progress streak updated in place using ANSI cursor
//     movement instead of reprinting a line per occurrence.
//   - Plain (--plain, or automatically when stdout is not a terminal):
//     no cursor movement or color; the repeat counter is flushed
//     periodically and when a streak ends, never mid-line.
//   - No-color (--no-color): like TTY but without SGR color codes.
//   - JSON (--json): one NDJSON object per decision, sharing the same
//     event/repeat/anomaly model so downstream tools see a bounded,
//     structured stream rather than one line per raw input line.
package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/severity"
)

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

// Options configures a Renderer.
type Options struct {
	IsTTY   bool
	Plain   bool
	NoColor bool
	JSON    bool
	// FlushInterval is how often a plain-mode or JSON-mode active streak's
	// counter is flushed while it is still accumulating.
	FlushInterval time.Duration
	// RepaintInterval throttles TTY in-place counter repaints.
	RepaintInterval time.Duration
}

// DefaultOptions returns sensible defaults; callers override IsTTY/Plain/
// NoColor/JSON based on flags and terminal detection.
func DefaultOptions() Options {
	return Options{
		FlushInterval:   2 * time.Second,
		RepaintInterval: 150 * time.Millisecond,
	}
}

// Event carries everything a renderer needs to display a brand-new or
// recurring-but-not-currently-active pattern.
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
}

// Renderer is stateful: it tracks the currently "active" streak (the most
// recently displayed pattern) so repeats can be collapsed into a counter.
type Renderer struct {
	w    io.Writer
	opts Options

	activeFP      fingerprint.ID
	activeOpen    bool
	activeCount   uint64
	counterShown  bool
	lastPainted   uint64
	lastFlush     time.Time
	lastRepaint   time.Time
}

// New creates a Renderer writing to w.
func New(w io.Writer, opts Options) *Renderer {
	return &Renderer{w: &safeWriter{w: w}, opts: opts}
}

// Broken reports whether the destination writer has been detected as a
// closed pipe. Once true, all further Emit/Repeat/Finalize calls are cheap
// no-ops and the caller should stop reading input for output purposes.
func (r *Renderer) Broken() bool {
	sw, ok := r.w.(*safeWriter)
	return ok && sw.broken
}

// IsActive reports whether fp is the pattern currently being displayed as
// an in-progress repeat streak.
func (r *Renderer) IsActive(fp fingerprint.ID) bool {
	return r.activeOpen && r.activeFP == fp
}

// ANSI helpers.
const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiBold   = "\x1b[1m"
	cursorUp   = "\x1b[1A"
	clearLine  = "\x1b[2K"
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
	Type           string  `json:"type"`
	Fingerprint    string  `json:"fingerprint,omitempty"`
	Severity       string  `json:"severity,omitempty"`
	Template       string  `json:"template,omitempty"`
	Lines          []string `json:"lines,omitempty"`
	Count          uint64  `json:"count,omitempty"`
	New            bool    `json:"new,omitempty"`
	BaselinePerMin float64 `json:"baseline_per_min,omitempty"`
	CurrentPerMin  float64 `json:"current_per_min,omitempty"`
}

func (r *Renderer) emitJSON(v jsonLine) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(r.w, string(b))
}

// Emit displays a new event, or a recurrence of a pattern that was not the
// immediately-preceding one. It finalizes any currently active streak first.
func (r *Renderer) Emit(evt Event) {
	r.finalize()

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
	} else {
		prefix, reset := r.color(evt.Severity)
		icon := evt.Severity.Icon()
		lines := evt.RawLines
		if !evt.IsNew {
			lines = []string{evt.Template}
		}
		for i, line := range lines {
			if i == 0 {
				label := evt.Severity.String()
				if evt.Severity == severity.Unknown {
					_, _ = fmt.Fprintf(r.w, "%s%s%s\n", prefix, line, reset)
				} else {
					_, _ = fmt.Fprintf(r.w, "%s%s %s %s%s\n", prefix, icon, label, line, reset)
				}
			} else {
				_, _ = fmt.Fprintf(r.w, "%s  %s%s\n", prefix, line, reset)
			}
		}
	}

	r.activeFP = evt.Fingerprint
	r.activeOpen = true
	r.activeCount = 1
	r.counterShown = false
	r.lastPainted = 1
	r.lastFlush = time.Now()
}

// Repeat registers another occurrence of the currently active pattern and
// updates its visible counter according to the render mode.
func (r *Renderer) Repeat(fp fingerprint.ID, now time.Time) {
	if !r.activeOpen || fp != r.activeFP {
		return
	}
	r.activeCount++

	if r.opts.JSON {
		if now.Sub(r.lastFlush) >= r.opts.FlushInterval {
			r.flushCounter(now)
		}
		return
	}
	if r.opts.Plain || !r.opts.IsTTY {
		if now.Sub(r.lastFlush) >= r.opts.FlushInterval {
			r.flushCounter(now)
		}
		return
	}

	// TTY: repaint in place, throttled.
	if now.Sub(r.lastRepaint) < r.opts.RepaintInterval {
		return
	}
	r.paintCounterTTY()
	r.lastRepaint = now
}

func (r *Renderer) paintCounterTTY() {
	if r.counterShown {
		_, _ = fmt.Fprint(r.w, cursorUp, clearLine)
	}
	_, _ = fmt.Fprintf(r.w, "  × %d\n", r.activeCount)
	r.counterShown = true
	r.lastPainted = r.activeCount
}

func (r *Renderer) flushCounter(now time.Time) {
	if r.activeCount == r.lastPainted {
		return
	}
	if r.opts.JSON {
		r.emitJSON(jsonLine{Type: "repeat_update", Fingerprint: fpStr(r.activeFP), Count: r.activeCount})
	} else {
		_, _ = fmt.Fprintf(r.w, "  × %d\n", r.activeCount)
	}
	r.lastPainted = r.activeCount
	r.counterShown = true
	r.lastFlush = now
}

// finalize ends the currently active streak, if any, ensuring its final
// count is visible before something else is printed.
func (r *Renderer) finalize() {
	if !r.activeOpen {
		return
	}
	if r.activeCount > r.lastPainted {
		if r.opts.JSON {
			r.emitJSON(jsonLine{Type: "repeat_final", Fingerprint: fpStr(r.activeFP), Count: r.activeCount})
		} else if r.opts.Plain || !r.opts.IsTTY {
			_, _ = fmt.Fprintf(r.w, "  × %d\n", r.activeCount)
		} else {
			r.paintCounterTTY()
		}
	}
	if !r.opts.JSON {
		_, _ = fmt.Fprintln(r.w)
	}
	r.activeOpen = false
	r.counterShown = false
}

// Finalize is the exported form, called at EOF/shutdown.
func (r *Renderer) Finalize() { r.finalize() }

// EmitAnomaly prints a frequency-spike banner, finalizing any active streak
// first. This is always shown in full immediately; anomalies are never
// throttled or collapsed.
func (r *Renderer) EmitAnomaly(a Anomaly) {
	r.finalize()

	if r.opts.JSON {
		r.emitJSON(jsonLine{
			Type:           "anomaly",
			Fingerprint:    fpStr(a.Fingerprint),
			Severity:       a.Severity.String(),
			Template:       a.Template,
			Lines:          a.RawLines,
			BaselinePerMin: a.BaselinePerMin,
			CurrentPerMin:  a.CurrentPerMin,
		})
		return
	}

	prefix, reset := r.color(severity.Critical)
	_, _ = fmt.Fprintf(r.w, "%s\U0001F6A8 FREQUENCY SPIKE%s\n", prefix, reset)
	summary := a.Template
	if len(a.RawLines) > 0 {
		summary = a.RawLines[0]
	}
	_, _ = fmt.Fprintf(r.w, "%s   %s%s\n", prefix, summary, reset)
	_, _ = fmt.Fprintf(r.w, "%s   baseline: %s/min%s\n", prefix, rate(a.BaselinePerMin), reset)
	_, _ = fmt.Fprintf(r.w, "%s   current:  %s/min%s\n", prefix, rate(a.CurrentPerMin), reset)
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

package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/severity"
)

// TestInteractiveTTYModeUsesColor exercises the actual interactive-
// terminal rendering path (IsTTY=true, not Plain/NoColor/JSON), which
// otherwise has no automated coverage - the CLI's own TTY detection can't
// be exercised from a non-interactive test runner, but the rendering
// logic it feeds into can and should be.
func TestInteractiveTTYModeUsesColor(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.IsTTY = true // as if os.Stdout were a real terminal
	r := New(&buf, opts)

	r.Emit(Event{Fingerprint: 1, Severity: severity.Error, Template: "boom", RawLines: []string{"boom"}, IsNew: true})
	r.Finalize(time.Unix(0, 0))

	out := buf.String()
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected interactive TTY mode to include ANSI color codes, got %q", out)
	}
	if !strings.Contains(out, "\x1b[0m") {
		t.Fatalf("expected a reset code after the colored segment, got %q", out)
	}
}

// TestNoColorModeSuppressesColorEvenOnTTY ensures --no-color overrides TTY
// detection: color is opt-out even when attached to a real terminal.
func TestNoColorModeSuppressesColorEvenOnTTY(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.IsTTY = true
	opts.NoColor = true
	r := New(&buf, opts)

	r.Emit(Event{Fingerprint: 1, Severity: severity.Critical, Template: "boom", RawLines: []string{"boom"}, IsNew: true})
	r.Finalize(time.Unix(0, 0))

	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("--no-color must suppress ANSI codes even when IsTTY is true, got %q", buf.String())
	}
}

func TestPlainModeNoANSI(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Info, Template: "hello", RawLines: []string{"hello"}, IsNew: true})
	r.Accumulate(1, severity.Info, "hello", now)
	r.Finalize(now.Add(time.Hour))

	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("plain mode output contains ANSI escape codes: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected message text in output, got %q", out)
	}
}

func TestAccumulateFlushesOnInterval(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	opts.FlushInterval = time.Second
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Info, Template: "tmpl", RawLines: []string{"raw"}, IsNew: true})
	r.Accumulate(1, severity.Info, "tmpl", now)
	r.Tick(now)
	if strings.Contains(buf.String(), "×") {
		t.Fatalf("counter should not flush before the interval elapses")
	}

	later := now.Add(2 * time.Second)
	r.Tick(later)
	if !strings.Contains(buf.String(), "× 1") {
		t.Fatalf("expected counter to flush after the interval elapsed, got %q", buf.String())
	}
}

func TestInterleavedPatternsBothAccumulate(t *testing.T) {
	// Reproduces the "restart loop cycling through several distinct
	// messages" case: two DIFFERENT patterns recur, never back-to-back,
	// and both must still end up collapsed rather than shown every time.
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Info, Template: "A", RawLines: []string{"A"}, IsNew: true})
	r.Emit(Event{Fingerprint: 2, Severity: severity.Info, Template: "B", RawLines: []string{"B"}, IsNew: true})
	for i := 0; i < 10; i++ {
		r.Accumulate(1, severity.Info, "A", now)
		r.Accumulate(2, severity.Info, "B", now)
	}
	r.FlushAll(now)

	out := buf.String()
	if !strings.Contains(out, "× 10") {
		t.Fatalf("expected both interleaved patterns to accumulate 10 each, got %q", out)
	}
	if strings.Count(out, "× 10") != 2 {
		t.Fatalf("expected two separate ×10 summaries (one per pattern), got %q", out)
	}
}

func TestJSONModeEmitsValidNDJSON(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.JSON = true
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: fingerprint.ID(42), Severity: severity.Warn, Template: "retry [NUM]", RawLines: []string{"retry 1"}, IsNew: true})
	r.EmitAnomaly(Anomaly{Fingerprint: fingerprint.ID(42), Severity: severity.Error, Template: "db timeout", BaselinePerMin: 0.1, CurrentPerMin: 280}, now)
	r.Finalize(now)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one JSON line")
	}
	for _, l := range lines {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(l), &v); err != nil {
			t.Fatalf("invalid JSON line %q: %v", l, err)
		}
	}
}

// TestJSONEventTypesMatchDocumentedSchema is the regression test for the
// stable JSON "type" values documented in the README and
// docs/ARCHITECTURE.md: exactly "event", "repeat_summary", and "anomaly".
// There is no separate "repeat_final" type - a repeat_summary is used both
// for periodic flushes and for the final flush of a pattern's count at
// EOF/shutdown, and this test exercises both to prove it.
func TestJSONEventTypesMatchDocumentedSchema(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.JSON = true
	opts.FlushInterval = time.Second
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Info, Template: "tmpl", RawLines: []string{"raw"}, IsNew: true})
	r.Accumulate(1, severity.Info, "tmpl", now)
	r.Tick(now.Add(2 * time.Second)) // periodic flush -> repeat_summary
	r.Accumulate(1, severity.Info, "tmpl", now.Add(2*time.Second))
	r.EmitAnomaly(Anomaly{Fingerprint: 2, Severity: severity.Error, Template: "db timeout", CurrentPerMin: 40, Bootstrap: true}, now)
	r.Finalize(now.Add(3 * time.Second)) // final flush -> also repeat_summary

	const (
		typeEvent   = "event"
		typeRepeat  = "repeat_summary"
		typeAnomaly = "anomaly"
	)
	allowed := map[string]bool{typeEvent: true, typeRepeat: true, typeAnomaly: true}

	seen := map[string]int{}
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(l), &v); err != nil {
			t.Fatalf("invalid JSON line %q: %v", l, err)
		}
		typ, _ := v["type"].(string)
		if !allowed[typ] {
			t.Fatalf("unexpected JSON type %q (only %v are documented), line: %q", typ, allowed, l)
		}
		if typ == "repeat_final" {
			t.Fatalf("found a repeat_final event type, but no such type should ever be emitted")
		}
		seen[typ]++
	}

	if seen[typeRepeat] < 2 {
		t.Fatalf("expected at least 2 repeat_summary events (one periodic, one final flush), got %d", seen[typeRepeat])
	}
	if seen[typeEvent] < 1 {
		t.Fatalf("expected at least 1 event")
	}
	if seen[typeAnomaly] < 1 {
		t.Fatalf("expected at least 1 anomaly")
	}
}

func TestSingletonEventShowsNoCounter(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Info, Template: "once", RawLines: []string{"once"}, IsNew: true})
	r.Finalize(now)

	out := buf.String()
	if strings.Contains(out, "×") {
		t.Fatalf("a single non-repeating event should not show a repeat counter, got %q", out)
	}
}

func TestAnomalyAlwaysShownEvenIfAccumulating(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Error, Template: "db timeout", RawLines: []string{"db timeout"}, IsNew: true})
	r.Accumulate(1, severity.Error, "db timeout", now)
	r.EmitAnomaly(Anomaly{Fingerprint: 1, Severity: severity.Error, Template: "db timeout", BaselinePerMin: 0.1, CurrentPerMin: 280}, now)

	out := buf.String()
	if !strings.Contains(out, "FREQUENCY SPIKE") {
		t.Fatalf("expected frequency spike banner, got %q", out)
	}
	if !strings.Contains(out, "0.1") || !strings.Contains(out, "280") {
		t.Fatalf("expected baseline and current rate in banner, got %q", out)
	}
}

// TestBootstrapAnomalyLabeledDistinctly ensures an assumed (not learned)
// baseline is never presented as if it were measured from real history.
func TestBootstrapAnomalyLabeledDistinctly(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Error, Template: "db timeout", RawLines: []string{"db timeout"}, IsNew: true})
	r.EmitAnomaly(Anomaly{Fingerprint: 1, Severity: severity.Error, Template: "db timeout", CurrentPerMin: 280, Bootstrap: true}, now)

	out := buf.String()
	if !strings.Contains(out, "FREQUENCY SPIKE") {
		t.Fatalf("expected a frequency spike banner, got %q", out)
	}
	if strings.Contains(out, "baseline:") {
		t.Fatalf("bootstrap anomaly must not present an assumed baseline as a measured one, got %q", out)
	}
	if !strings.Contains(out, "no prior history") {
		t.Fatalf("expected the banner to say plainly that there is no prior history, got %q", out)
	}
	if !strings.Contains(out, "280") {
		t.Fatalf("expected the current rate to still be shown, got %q", out)
	}
}

func TestProtectedSeverityFlushesSooner(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Plain = true
	opts.FlushInterval = 10 * time.Second
	opts.ProtectedFlushInterval = 100 * time.Millisecond
	opts.ProtectRank = int(severity.Error)
	r := New(&buf, opts)

	now := time.Unix(0, 0)
	r.Emit(Event{Fingerprint: 1, Severity: severity.Error, Template: "err", RawLines: []string{"err"}, IsNew: true})
	r.Accumulate(1, severity.Error, "err", now)
	r.Tick(now.Add(200 * time.Millisecond))

	if !strings.Contains(buf.String(), "× 1") {
		t.Fatalf("protected severity should flush well before the ordinary interval, got %q", buf.String())
	}
}

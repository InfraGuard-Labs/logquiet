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

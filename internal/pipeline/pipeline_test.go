package pipeline

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/config"
	"github.com/azimsiddiqui/logquiet/internal/render"
)

// letterize converts a non-negative int into a unique pure-alphabetic
// string (base-26, A-Z), used by tests that need many structurally
// distinct messages without accidentally producing digits that structural
// normalization would collapse away.
func letterize(i int) string {
	if i == 0 {
		return "A"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('A' + i%26)}, b...)
		i /= 26
	}
	return string(b)
}

func newTestPipeline(jsonMode bool) (*Pipeline, *bytes.Buffer) {
	var buf bytes.Buffer
	ropts := render.DefaultOptions()
	ropts.Plain = true
	ropts.JSON = jsonMode
	r := render.New(&buf, ropts)
	cfg := config.Default()
	return New(cfg, r), &buf
}

// TestRoutineRepetitionIsCollapsed is the headline scenario from the spec:
// a flood of identical structural lines should not each appear individually.
func TestRoutineRepetitionIsCollapsed(t *testing.T) {
	p, buf := newTestPipeline(false)
	now := time.Unix(0, 0)
	for i := 0; i < 1000; i++ {
		now = now.Add(10 * time.Millisecond)
		p.ProcessLine("2026-08-30 03:01:00 [INFO] Connection pool active. 42 connections open.", now)
	}
	p.Finish(now)

	out := buf.String()
	// 1000 occurrences over ~10 simulated seconds will produce a handful of
	// periodic counter flushes (by design - see render.go), never anywhere
	// close to one line per occurrence.
	occurrences := strings.Count(out, "Connection pool active")
	if occurrences == 0 || occurrences > 10 {
		t.Fatalf("expected a small number of periodic summaries, not %d, for 1000 repeats:\n%s", occurrences, out)
	}
	snap := p.Counters.Snapshot(now, p.PatternCount(), p.PatternsEvicted())
	if snap.SuppressedEvents != 999 {
		t.Fatalf("suppressed events = %d, want 999", snap.SuppressedEvents)
	}
	if snap.DisplayedEvents != 1 {
		t.Fatalf("displayed events = %d, want 1", snap.DisplayedEvents)
	}
}

// TestNovelEventAlwaysSurfaces ensures a rare, structurally distinct line
// breaks through even while a routine pattern floods the stream.
func TestNovelEventAlwaysSurfaces(t *testing.T) {
	p, buf := newTestPipeline(false)
	now := time.Unix(0, 0)
	for i := 0; i < 50; i++ {
		now = now.Add(10 * time.Millisecond)
		p.ProcessLine("2026-08-30 03:01:00 [INFO] heartbeat ok", now)
	}
	now = now.Add(10 * time.Millisecond)
	p.ProcessLine("2026-08-30 03:01:00 [INFO] User 10829 requested page /dashboard", now)
	for i := 0; i < 50; i++ {
		now = now.Add(10 * time.Millisecond)
		p.ProcessLine("2026-08-30 03:01:00 [INFO] heartbeat ok", now)
	}
	p.Finish(now)

	out := buf.String()
	if !strings.Contains(out, "requested page") {
		t.Fatalf("novel event did not surface:\n%s", out)
	}
}

// TestErrorSeverityAlwaysSurfaces checks that a single ERROR/CRITICAL line
// amid routine INFO noise is never dropped.
func TestErrorSeverityAlwaysSurfaces(t *testing.T) {
	p, buf := newTestPipeline(false)
	now := time.Unix(0, 0)
	for i := 0; i < 20; i++ {
		now = now.Add(10 * time.Millisecond)
		p.ProcessLine("2026-08-30 03:01:00 [INFO] heartbeat ok", now)
	}
	now = now.Add(10 * time.Millisecond)
	p.ProcessLine("2026-08-30 03:01:00 [CRITICAL] DATABASE TIMEOUT: Host 10.0.1.45 failed to respond in 5000ms.", now)
	p.Finish(now)

	out := buf.String()
	if !strings.Contains(out, "DATABASE TIMEOUT") {
		t.Fatalf("critical event did not surface:\n%s", out)
	}
}

// TestMultilineTracebackStaysGrouped verifies end-to-end that a Python
// traceback fed line-by-line through the full pipeline renders as a single
// event, not four unrelated ones.
func TestMultilineTracebackStaysGrouped(t *testing.T) {
	p, buf := newTestPipeline(true)
	now := time.Unix(0, 0)
	lines := []string{
		`2026-08-30 03:01:07 [ERROR] Traceback (most recent call last):`,
		`2026-08-30 03:01:07 [ERROR]   File "/app/db.py", line 42, in execute_query`,
		`2026-08-30 03:01:07 [ERROR]     raise TimeoutError("DB connection lost")`,
		`2026-08-30 03:01:07 [ERROR] TimeoutError: DB connection lost`,
	}
	for _, l := range lines {
		now = now.Add(1 * time.Millisecond)
		p.ProcessLine(l, now)
	}
	p.Finish(now)

	var events []map[string]interface{}
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(l), &v); err != nil {
			t.Fatalf("invalid JSON line %q: %v", l, err)
		}
		if v["type"] == "event" {
			events = append(events, v)
		}
	}
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event for the whole traceback, got %d: %+v", len(events), events)
	}
	rawLines, _ := events[0]["lines"].([]interface{})
	if len(rawLines) != 4 {
		t.Fatalf("expected 4 lines grouped into the traceback event, got %d", len(rawLines))
	}
}

// TestRawLineCountsAlwaysSumToInputLines is the correctness invariant for
// the raw-line suppression metric: every physical input line ends up in
// exactly one completed multiline block, and that block contributes its
// full line count to exactly one of DisplayedRawLines/SuppressedRawLines -
// never both, never neither - so the two must always sum to InputLines.
func TestRawLineCountsAlwaysSumToInputLines(t *testing.T) {
	p, _ := newTestPipeline(false)
	now := time.Unix(0, 0)
	lines := []string{
		`2026-08-30 03:01:00 [INFO] processed order 100000 in 23ms`,
		`2026-08-30 03:01:01 [INFO] processed order 100001 in 19ms`,
		`2026-08-30 03:01:02 [INFO] processed order 100002 in 21ms`,
		`2026-08-30 03:01:07 [ERROR] Traceback (most recent call last):`,
		`2026-08-30 03:01:07 [ERROR]   File "/app/db.py", line 42, in execute_query`,
		`2026-08-30 03:01:07 [ERROR]     raise TimeoutError("DB connection lost")`,
		`2026-08-30 03:01:07 [ERROR] TimeoutError: DB connection lost`,
		`2026-08-30 03:01:08 [INFO] processed order 100003 in 25ms`,
	}
	for _, l := range lines {
		now = now.Add(1 * time.Millisecond)
		p.ProcessLine(l, now)
	}
	p.Finish(now)

	c := p.Counters
	if c.InputLines != uint64(len(lines)) {
		t.Fatalf("InputLines = %d, want %d", c.InputLines, len(lines))
	}
	if got := c.DisplayedRawLines + c.SuppressedRawLines; got != c.InputLines {
		t.Fatalf("DisplayedRawLines(%d) + SuppressedRawLines(%d) = %d, want InputLines = %d",
			c.DisplayedRawLines, c.SuppressedRawLines, got, c.InputLines)
	}
}

// TestRawLineSuppressionDiffersFromLogicalEventSuppression is the
// regression test proving the two metrics are genuinely different numbers
// when a multiline block is involved, not just differently-named aliases
// for the same computation - which is the entire reason both are exposed.
func TestRawLineSuppressionDiffersFromLogicalEventSuppression(t *testing.T) {
	p, _ := newTestPipeline(false)
	now := time.Unix(0, 0)
	// 1 novel single-line event, 1 novel 4-line traceback event, then 3
	// suppressed repeats of the single-line pattern.
	lines := []string{
		`2026-08-30 03:01:00 [INFO] processed order 100000 in 23ms`,
		`2026-08-30 03:01:07 [ERROR] Traceback (most recent call last):`,
		`2026-08-30 03:01:07 [ERROR]   File "/app/db.py", line 42, in execute_query`,
		`2026-08-30 03:01:07 [ERROR]     raise TimeoutError("DB connection lost")`,
		`2026-08-30 03:01:07 [ERROR] TimeoutError: DB connection lost`,
		`2026-08-30 03:01:01 [INFO] processed order 100001 in 19ms`,
		`2026-08-30 03:01:02 [INFO] processed order 100002 in 21ms`,
		`2026-08-30 03:01:03 [INFO] processed order 100003 in 25ms`,
	}
	for _, l := range lines {
		now = now.Add(1 * time.Millisecond)
		p.ProcessLine(l, now)
	}
	p.Finish(now)

	snap := p.Counters.Snapshot(now, p.PatternCount(), p.PatternsEvicted())

	// Logical events: 2 displayed (the INFO line, the traceback) + 3
	// suppressed (repeats) = 5 total -> 60% suppressed.
	if snap.DisplayedEvents != 2 || snap.SuppressedEvents != 3 {
		t.Fatalf("expected 2 displayed / 3 suppressed logical events, got %d/%d", snap.DisplayedEvents, snap.SuppressedEvents)
	}
	// Raw lines: displayed = 1 (INFO) + 4 (traceback) = 5; suppressed = 3;
	// total = 8 -> 37.5% suppressed. Deliberately different from the
	// logical-event figure above.
	if snap.DisplayedRawLines != 5 || snap.SuppressedRawLines != 3 {
		t.Fatalf("expected 5 displayed / 3 suppressed raw lines, got %d/%d", snap.DisplayedRawLines, snap.SuppressedRawLines)
	}
	if snap.LogicalEventSuppressionPercent == snap.RawLineSuppressionPercent {
		t.Fatalf("expected the two suppression metrics to differ given a multiline block, both were %.2f%%", snap.LogicalEventSuppressionPercent)
	}
	if snap.RawLineSuppressionPercent != 37.5 {
		t.Fatalf("RawLineSuppressionPercent = %.2f, want 37.5", snap.RawLineSuppressionPercent)
	}
	if snap.LogicalEventSuppressionPercent != 60 {
		t.Fatalf("LogicalEventSuppressionPercent = %.2f, want 60", snap.LogicalEventSuppressionPercent)
	}
}

// TestFrequencySpikeIsSurfacedNotSuppressed reproduces the spec's critical
// safety scenario: a rare error suddenly firing hundreds of times per
// minute must trigger a visible anomaly rather than being silently
// absorbed as routine repetition.
func TestFrequencySpikeIsSurfacedNotSuppressed(t *testing.T) {
	p, buf := newTestPipeline(true)
	now := time.Unix(0, 0)

	for i := 0; i < 10; i++ {
		now = now.Add(45 * time.Second)
		p.ProcessLine("2026-08-30 03:01:00 [ERROR] database timeout on host 10.0.1.45", now)
	}
	for i := 0; i < 400; i++ {
		now = now.Add(150 * time.Millisecond)
		p.ProcessLine("2026-08-30 03:01:00 [ERROR] database timeout on host 10.0.1.45", now)
	}
	p.Finish(now)

	found := false
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(l), &v); err == nil && v["type"] == "anomaly" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a frequency-spike anomaly to be emitted during the burst")
	}
	snap := p.Counters.Snapshot(now, p.PatternCount(), p.PatternsEvicted())
	if snap.AnomalyEvents == 0 {
		t.Fatalf("expected AnomalyEvents > 0 in stats")
	}
}

// TestBoundedMemoryUnderHighCardinality feeds many thousands of distinct
// patterns and asserts the tracked pattern count never exceeds the
// configured bound.
func TestBoundedMemoryUnderHighCardinality(t *testing.T) {
	var buf bytes.Buffer
	ropts := render.DefaultOptions()
	ropts.Plain = true
	r := render.New(&buf, ropts)
	cfg := config.Default()
	cfg.MaxPatterns = 500
	p := New(cfg, r)

	now := time.Unix(0, 0)
	for i := 0; i < 5000; i++ {
		now = now.Add(time.Millisecond)
		// A pure-letter suffix (not digits, not mixed alnum) so structural
		// normalization does not collapse these back into one pattern -
		// each iteration must remain a genuinely distinct template to
		// actually exercise high-cardinality eviction.
		p.ProcessLine("2026-08-30 03:01:00 [INFO] unique event marker "+letterize(i), now)
	}
	p.Finish(now)

	if p.PatternCount() > cfg.MaxPatterns {
		t.Fatalf("pattern count %d exceeds configured bound %d", p.PatternCount(), cfg.MaxPatterns)
	}
	if p.PatternsEvicted() == 0 {
		t.Fatalf("expected evictions to have occurred under high cardinality")
	}
}

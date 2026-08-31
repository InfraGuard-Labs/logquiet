// Package stats accumulates session-level counters used by --stats and
// --impact-report. The impact report intentionally contains only aggregate
// numbers - never raw log content - so it is safe to share; see
// docs/IMPACT_REPORT.md.
package stats

import (
	"encoding/json"
	"math"
	"runtime"
	"time"
)

// Counters accumulates counts over the lifetime of one run. It is not
// safe for concurrent use; the pipeline is single-threaded by design.
type Counters struct {
	Start time.Time

	InputLines       uint64
	DisplayedEvents  uint64
	SuppressedEvents uint64

	// DisplayedRawLines and SuppressedRawLines count actual raw physical
	// input lines (not logical events): every raw line fed to the pipeline
	// ends up in exactly one completed multiline block, and that block's
	// line count is added to whichever of these two counters matches how
	// the block was rendered - so DisplayedRawLines + SuppressedRawLines
	// == InputLines always holds (see
	// pipeline.TestRawLineCountsAlwaysSumToInputLines). This is what makes
	// a true raw-line reduction percentage possible, distinct from the
	// logical-event one - see RawLineSuppressionPercent below.
	DisplayedRawLines  uint64
	SuppressedRawLines uint64

	// WarningEvents and ErrorEvents count every WARN and ERROR-or-above
	// occurrence *observed* in the input, incremented once per occurrence
	// regardless of whether that particular occurrence was shown in full
	// or collapsed into a suppressed repeat counter. They are NOT a count
	// of individually-displayed lines - see DisplayedEvents/
	// SuppressedEvents for that distinction. Named "observed" rather than
	// "surfaced" or "displayed" for exactly this reason.
	WarningEvents uint64
	ErrorEvents   uint64

	AnomalyEvents  uint64
	BytesRead      uint64
	TruncatedLines uint64
}

// New returns a Counters with Start set to now.
func New() *Counters {
	return &Counters{Start: time.Now()}
}

// Snapshot is a point-in-time, human- and machine-readable view of Counters
// plus derived metrics.
type Snapshot struct {
	// InputLines is a count of raw physical input lines (one per line read
	// from the stream), independent of multiline assembly.
	InputLines uint64 `json:"input_lines"`

	// DisplayedEvents and SuppressedEvents count logical events - after
	// multiline assembly, so one multi-line stack trace is one event, not
	// one per raw line it spans - not raw physical lines. See
	// LogicalEventSuppressionPercent below and docs/IMPACT_REPORT.md
	// "Physical lines vs. logical events" for why this distinction matters.
	// RawLineSuppressionPercent below is the separate, true raw-line
	// figure.
	DisplayedEvents  uint64 `json:"displayed_events"`
	SuppressedEvents uint64 `json:"suppressed_events"`

	// LogicalEventSuppressionPercent is
	// 100 * SuppressedEvents / (SuppressedEvents + DisplayedEvents) - the
	// fraction of *logical events*, not raw input lines, collapsed into a
	// repeat counter rather than shown individually. It is named
	// explicitly "logical event" (not just "suppression") so it cannot be
	// mistaken for a raw-line reduction figure.
	LogicalEventSuppressionPercent float64 `json:"logical_event_suppression_percentage"`

	// RawLineSuppressionPercent is 100 * SuppressedRawLines / InputLines -
	// the fraction of actual raw physical input lines that were not
	// individually displayed (folded into a repeat counter instead),
	// counting every line of a multiline block that was collapsed, not
	// just the block itself. This is the metric to use for a genuine
	// "N% of the raw log was hidden" claim; LogicalEventSuppressionPercent
	// is not that and the two will legitimately differ whenever multiline
	// blocks are involved - see docs/IMPACT_REPORT.md.
	RawLineSuppressionPercent float64 `json:"raw_line_suppression_percentage"`
	// DisplayedRawLines and SuppressedRawLines are the raw counts behind
	// RawLineSuppressionPercent, exposed directly so the percentage can be
	// independently verified rather than taken on faith.
	DisplayedRawLines  uint64 `json:"displayed_raw_lines"`
	SuppressedRawLines uint64 `json:"suppressed_raw_lines"`

	StructuralPatterns int    `json:"structural_patterns"`
	PatternsEvicted    uint64 `json:"patterns_evicted"`
	// WarningEvents and ErrorEvents: occurrences observed at that
	// severity, not a count of individually-displayed lines - see the
	// Counters doc comment and docs/IMPACT_REPORT.md.
	WarningEvents  uint64  `json:"warning_events"`
	ErrorEvents    uint64  `json:"error_events"`
	AnomalyEvents  uint64  `json:"anomaly_events"`
	TruncatedLines uint64  `json:"truncated_lines"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	LinesPerSecond float64 `json:"lines_per_second"`
	MBPerSecond    float64 `json:"mb_per_second"`
	ApproxMemoryMB float64 `json:"approx_memory_mb"`
}

// Snapshot computes derived metrics as of now, given the current pattern
// count and eviction count from the pattern store.
func (c *Counters) Snapshot(now time.Time, patternCount int, patternsEvicted uint64) Snapshot {
	elapsed := now.Sub(c.Start).Seconds()
	if elapsed <= 0 {
		elapsed = 1e-9
	}
	suppressionPct := 0.0
	total := c.DisplayedEvents + c.SuppressedEvents
	if total > 0 {
		suppressionPct = 100 * float64(c.SuppressedEvents) / float64(total)
	}
	rawLineSuppressionPct := 0.0
	if c.InputLines > 0 {
		rawLineSuppressionPct = 100 * float64(c.SuppressedRawLines) / float64(c.InputLines)
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return Snapshot{
		InputLines:                     c.InputLines,
		DisplayedEvents:                c.DisplayedEvents,
		SuppressedEvents:               c.SuppressedEvents,
		LogicalEventSuppressionPercent: round2(suppressionPct),
		RawLineSuppressionPercent:      round2(rawLineSuppressionPct),
		DisplayedRawLines:              c.DisplayedRawLines,
		SuppressedRawLines:             c.SuppressedRawLines,
		StructuralPatterns:             patternCount,
		PatternsEvicted:                patternsEvicted,
		WarningEvents:                  c.WarningEvents,
		ErrorEvents:                    c.ErrorEvents,
		AnomalyEvents:                  c.AnomalyEvents,
		TruncatedLines:                 c.TruncatedLines,
		ElapsedSeconds:                 round2(elapsed),
		LinesPerSecond:                 round2(float64(c.InputLines) / elapsed),
		MBPerSecond:                    round2((float64(c.BytesRead) / (1024 * 1024)) / elapsed),
		ApproxMemoryMB:                 round2(float64(mem.Alloc) / (1024 * 1024)),
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// ImpactReportSchemaVersion identifies the shape of ImpactReport. It was
// introduced (jumping straight to 2, since schema version 1 was never
// tagged with an explicit field) when suppression_percentage was replaced
// by the more precise logical_event_suppression_percentage and
// raw_line_suppression_percentage - see docs/IMPACT_REPORT.md
// "Versioning".
const ImpactReportSchemaVersion = 2

// ImpactReport is the stable, documented schema written by --impact-report.
// It contains aggregate technical statistics only - no raw log content,
// hostnames, usernames, IPs, tokens, or file contents. See
// docs/IMPACT_REPORT.md for the full field-by-field contract.
type ImpactReport struct {
	SchemaVersion       int     `json:"schema_version"`
	LogquietVersion     string  `json:"logquiet_version"`
	GeneratedAt         string  `json:"generated_at"`
	SessionDurationSecs float64 `json:"session_duration_seconds"`
	// RawLines is raw physical input lines. DisplayedEvents and
	// SuppressedEvents are logical events (post multiline-assembly) - see
	// docs/IMPACT_REPORT.md "Physical lines vs. logical events".
	RawLines         uint64 `json:"raw_lines"`
	DisplayedEvents  uint64 `json:"displayed_events"`
	SuppressedEvents uint64 `json:"suppressed_events"`
	// LogicalEventSuppressionPercentage: the fraction of logical events
	// collapsed into a repeat counter, NOT a raw-line reduction
	// percentage - see docs/IMPACT_REPORT.md.
	LogicalEventSuppressionPercentage float64 `json:"logical_event_suppression_percentage"`
	// RawLineSuppressionPercentage: the true fraction of raw physical
	// input lines not individually displayed - see docs/IMPACT_REPORT.md.
	RawLineSuppressionPercentage float64 `json:"raw_line_suppression_percentage"`
	DisplayedRawLines            uint64  `json:"displayed_raw_lines"`
	SuppressedRawLines           uint64  `json:"suppressed_raw_lines"`
	StructuralPatterns           int     `json:"structural_patterns"`
	// WarningEvents and ErrorEvents: occurrences observed at that
	// severity in the input, whether or not each one was individually
	// displayed - see docs/IMPACT_REPORT.md.
	WarningEvents     uint64  `json:"warning_events"`
	ErrorEvents       uint64  `json:"error_events"`
	AnomalyEvents     uint64  `json:"anomaly_events"`
	ProcessingRateLPS float64 `json:"processing_rate_lines_per_second"`
}

// BuildImpactReport constructs the report from a snapshot. version is the
// logquiet build version string.
func BuildImpactReport(version string, snap Snapshot, now time.Time) ImpactReport {
	return ImpactReport{
		SchemaVersion:                     ImpactReportSchemaVersion,
		LogquietVersion:                   version,
		GeneratedAt:                       now.UTC().Format(time.RFC3339),
		SessionDurationSecs:               snap.ElapsedSeconds,
		RawLines:                          snap.InputLines,
		DisplayedEvents:                   snap.DisplayedEvents,
		SuppressedEvents:                  snap.SuppressedEvents,
		LogicalEventSuppressionPercentage: snap.LogicalEventSuppressionPercent,
		RawLineSuppressionPercentage:      snap.RawLineSuppressionPercent,
		DisplayedRawLines:                 snap.DisplayedRawLines,
		SuppressedRawLines:                snap.SuppressedRawLines,
		StructuralPatterns:                snap.StructuralPatterns,
		WarningEvents:                     snap.WarningEvents,
		ErrorEvents:                       snap.ErrorEvents,
		AnomalyEvents:                     snap.AnomalyEvents,
		ProcessingRateLPS:                 snap.LinesPerSecond,
	}
}

// MarshalIndent renders the report as pretty JSON.
func (r ImpactReport) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

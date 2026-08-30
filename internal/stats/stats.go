// Package stats accumulates session-level counters used by --stats and
// --impact-report. The impact report intentionally contains only aggregate
// numbers - never raw log content - so it is safe to share; see
// docs/IMPACT_REPORT.md.
package stats

import (
	"encoding/json"
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
	WarningEvents    uint64
	ErrorEvents      uint64
	AnomalyEvents    uint64
	BytesRead        uint64
	TruncatedLines   uint64
}

// New returns a Counters with Start set to now.
func New() *Counters {
	return &Counters{Start: time.Now()}
}

// Snapshot is a point-in-time, human- and machine-readable view of Counters
// plus derived metrics.
type Snapshot struct {
	InputLines           uint64  `json:"input_lines"`
	DisplayedEvents      uint64  `json:"displayed_events"`
	SuppressedEvents     uint64  `json:"suppressed_events"`
	SuppressionPercent   float64 `json:"suppression_percentage"`
	StructuralPatterns   int     `json:"structural_patterns"`
	PatternsEvicted      uint64  `json:"patterns_evicted"`
	WarningEvents        uint64  `json:"warning_events"`
	ErrorEvents          uint64  `json:"error_events"`
	AnomalyEvents        uint64  `json:"anomaly_events"`
	TruncatedLines       uint64  `json:"truncated_lines"`
	ElapsedSeconds       float64 `json:"elapsed_seconds"`
	LinesPerSecond       float64 `json:"lines_per_second"`
	MBPerSecond          float64 `json:"mb_per_second"`
	ApproxMemoryMB       float64 `json:"approx_memory_mb"`
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

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return Snapshot{
		InputLines:         c.InputLines,
		DisplayedEvents:    c.DisplayedEvents,
		SuppressedEvents:   c.SuppressedEvents,
		SuppressionPercent: suppressionPct,
		StructuralPatterns: patternCount,
		PatternsEvicted:    patternsEvicted,
		WarningEvents:      c.WarningEvents,
		ErrorEvents:        c.ErrorEvents,
		AnomalyEvents:      c.AnomalyEvents,
		TruncatedLines:     c.TruncatedLines,
		ElapsedSeconds:     elapsed,
		LinesPerSecond:     float64(c.InputLines) / elapsed,
		MBPerSecond:        (float64(c.BytesRead) / (1024 * 1024)) / elapsed,
		ApproxMemoryMB:     float64(mem.Alloc) / (1024 * 1024),
	}
}

// ImpactReport is the stable, documented schema written by --impact-report.
// It contains aggregate technical statistics only - no raw log content,
// hostnames, usernames, IPs, tokens, or file contents. See
// docs/IMPACT_REPORT.md for the full field-by-field contract.
type ImpactReport struct {
	LogquietVersion       string  `json:"logquiet_version"`
	GeneratedAt           string  `json:"generated_at"`
	SessionDurationSecs   float64 `json:"session_duration_seconds"`
	RawLines              uint64  `json:"raw_lines"`
	DisplayedEvents       uint64  `json:"displayed_events"`
	SuppressedEvents      uint64  `json:"suppressed_events"`
	SuppressionPercentage float64 `json:"suppression_percentage"`
	StructuralPatterns    int     `json:"structural_patterns"`
	WarningEvents         uint64  `json:"warning_events"`
	ErrorEvents           uint64  `json:"error_events"`
	AnomalyEvents         uint64  `json:"anomaly_events"`
	ProcessingRateLPS     float64 `json:"processing_rate_lines_per_second"`
}

// BuildImpactReport constructs the report from a snapshot. version is the
// logquiet build version string.
func BuildImpactReport(version string, snap Snapshot, now time.Time) ImpactReport {
	return ImpactReport{
		LogquietVersion:       version,
		GeneratedAt:           now.UTC().Format(time.RFC3339),
		SessionDurationSecs:   snap.ElapsedSeconds,
		RawLines:              snap.InputLines,
		DisplayedEvents:       snap.DisplayedEvents,
		SuppressedEvents:      snap.SuppressedEvents,
		SuppressionPercentage: snap.SuppressionPercent,
		StructuralPatterns:    snap.StructuralPatterns,
		WarningEvents:         snap.WarningEvents,
		ErrorEvents:           snap.ErrorEvents,
		AnomalyEvents:         snap.AnomalyEvents,
		ProcessingRateLPS:     snap.LinesPerSecond,
	}
}

// MarshalIndent renders the report as pretty JSON.
func (r ImpactReport) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

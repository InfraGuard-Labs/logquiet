// Package pattern tracks per-fingerprint occurrence state - counts, rolling
// rate, and a slow baseline - in bounded memory, and decides when a
// pattern's current frequency represents an anomalous spike rather than
// routine repetition.
//
// Algorithm summary (see docs/TECHNICAL_METHOD.md for full rationale):
//
//  1. Each pattern keeps a ring buffer of fixed-width time buckets covering
//     a short "current" window (default 15s / 3x5s buckets). This window
//     is deliberately short: a longer window (an earlier version used 60s)
//     dilutes a burst that has only just started, since the rate is
//     computed as events-in-window / window-duration and a fixed-length
//     denominator does not shrink just because the burst is new - a burst
//     lasting only a few seconds gets averaged down by the quiet time
//     still sitting in the rest of a long window, making detection slow
//     and, worse, marginal/inconsistent right at the threshold boundary.
//     Baseline quality is unaffected by this window's length: baseline is
//     a lifetime exponentially weighted average over every bucket ever
//     folded (point 2 below), not a function of how many buckets are held
//     at once.
//  2. Baseline formation is data-driven, not a flat wall-clock timer: every
//     time a bucket finishes accumulating (every BucketWidth, starting from
//     the very first one), its rate is folded into a slow exponentially
//     weighted moving average that represents the pattern's learned
//     "normal" rate. Once at least MinBaselineSamples buckets have been
//     folded in, that baseline is trusted for comparison - by default this
//     takes 15 seconds, not the 2 minutes-plus a first version of this
//     detector required.
//  3. A "standard" spike is flagged once a real baseline exists and the
//     current-window rate exceeds it by a configurable multiplier, gated
//     by a minimum absolute event count (so a near-zero baseline can't be
//     "exceeded" by a single stray event) and a per-pattern cooldown.
//  4. A separate "bootstrap" path covers a pattern with NO baseline yet at
//     all (brand new, or too young to have MinBaselineSamples) that is
//     already firing at a severe, sustained rate: for severities at or
//     above -severity-protect only, an assumed near-zero baseline is used
//     so a severe error class occurring for the first time ever, at high
//     volume, from the very first moment, is still surfaced - not silently
//     absorbed while "warming up". This does not apply to ordinary
//     severities, so a routine, merely-frequent INFO pattern occurring
//     from the start of a session is never bootstrap-flagged.
package pattern

import (
	"container/list"
	"time"

	"github.com/InfraGuard-Labs/logquiet/internal/fingerprint"
	"github.com/InfraGuard-Labs/logquiet/internal/severity"
)

// Config tunes anomaly sensitivity and store bounds. Zero-value Config is
// invalid; use DefaultConfig().
type Config struct {
	BucketWidth     time.Duration
	WindowBuckets   int
	BaselineAlpha   float64
	SpikeMultiplier float64
	MinWindowEvents int
	Cooldown        time.Duration

	// MinBaselineSamples is how many completed buckets must have been
	// folded into the baseline before it is trusted for the standard
	// (baseline-vs-current) comparison. This is a sample count, not a
	// wall-clock duration, so it scales correctly with BucketWidth and
	// unblocks detection as soon as enough real data exists rather than
	// after an arbitrary fixed delay.
	MinBaselineSamples int

	// AssumedBaselinePerMin and MinBootstrapEvents configure the
	// bootstrap path used only for severities at/above ProtectRank that
	// have no trustworthy baseline yet. See package doc point 4.
	AssumedBaselinePerMin float64
	MinBootstrapEvents    int

	MaxTrackedPatterns int

	// ProtectRank and ProtectMultiplier give severities at/above ProtectRank
	// a more sensitive (typically lower) spike multiplier, since a rare
	// event class that suddenly spikes is more likely to be the incident
	// itself; see docs/TECHNICAL_METHOD.md "Severity-aware sensitivity".
	ProtectRank       int
	ProtectMultiplier float64
}

// DefaultConfig returns sensible, documented defaults suitable for
// zero-configuration use.
func DefaultConfig() Config {
	return Config{
		BucketWidth:           5 * time.Second,
		WindowBuckets:         3, // 15s current-rate window - see package doc point 2
		BaselineAlpha:         0.05,
		SpikeMultiplier:       8.0,
		MinWindowEvents:       5,
		Cooldown:              60 * time.Second,
		MinBaselineSamples:    3, // ~15s of real history before trusting baseline
		AssumedBaselinePerMin: 1.0,
		MinBootstrapEvents:    10,
		MaxTrackedPatterns:    10000,
		ProtectRank:           int(severity.Error),
		ProtectMultiplier:     3.0,
	}
}

// Spike describes a detected frequency anomaly for one pattern.
type Spike struct {
	BaselinePerMin float64
	CurrentPerMin  float64
	// Bootstrap is true when no real learned baseline existed yet and
	// BaselinePerMin is the configured assumed value, not data observed
	// from this pattern's own history. The renderer labels these
	// differently so the distinction is never presented as more certain
	// than it is.
	Bootstrap bool
}

// State is the tracked history of one structural pattern.
type State struct {
	Fingerprint fingerprint.ID
	Template    string
	Severity    severity.Level
	Example     []string // first raw block observed for this pattern

	TotalCount uint64
	FirstSeen  time.Time
	LastSeen   time.Time

	cfg             Config
	buckets         []uint32
	bucketIdx       int
	bucketStart     time.Time
	baseline        float64
	baselineSamples int
	lastAlert       time.Time

	elem *list.Element // back-reference into Store's LRU list
}

func newState(cfg Config, fp fingerprint.ID, tmpl string, lvl severity.Level, example []string, now time.Time) *State {
	return &State{
		Fingerprint: fp,
		Template:    tmpl,
		Severity:    lvl,
		Example:     example,
		FirstSeen:   now,
		LastSeen:    now,
		cfg:         cfg,
		buckets:     make([]uint32, cfg.WindowBuckets),
		bucketStart: now.Truncate(cfg.BucketWidth),
	}
}

// Record registers one new occurrence at time now and returns a Spike if
// this occurrence causes the pattern's frequency to be classified as
// anomalous (subject to the cooldown described above).
func (s *State) Record(now time.Time) *Spike {
	s.advance(now)
	s.buckets[s.bucketIdx]++
	s.TotalCount++
	s.LastSeen = now

	windowEvents := s.windowEventCount()
	if windowEvents < s.cfg.MinWindowEvents {
		return nil
	}
	if now.Sub(s.lastAlert) < s.cfg.Cooldown {
		return nil
	}

	protected := s.Severity.Rank() >= s.cfg.ProtectRank
	multiplier := s.cfg.SpikeMultiplier
	if protected {
		multiplier = s.cfg.ProtectMultiplier
	}
	current := s.currentRatePerMin()

	if s.baselineSamples >= s.cfg.MinBaselineSamples && s.baseline > 0 {
		if current < s.baseline*multiplier {
			return nil
		}
		s.lastAlert = now
		return &Spike{BaselinePerMin: s.baseline, CurrentPerMin: current}
	}

	// No trustworthy learned baseline yet: only bootstrap for protected
	// severities, and only once a real sustained volume has been seen
	// (not merely MinWindowEvents), so an ordinary "a new error type
	// showed up a few times" does not get elevated to a spike banner.
	if !protected || windowEvents < s.cfg.MinBootstrapEvents {
		return nil
	}
	if current < s.cfg.AssumedBaselinePerMin*multiplier {
		return nil
	}
	s.lastAlert = now
	return &Spike{BaselinePerMin: s.cfg.AssumedBaselinePerMin, CurrentPerMin: current, Bootstrap: true}
}

// advance rotates the ring buffer forward to `now`, folding each bucket
// that finishes accumulating into the slow baseline EWMA - starting from
// the very first bucket, not only once a full window has cycled.
func (s *State) advance(now time.Time) {
	elapsed := now.Sub(s.bucketStart)
	n := int(elapsed / s.cfg.BucketWidth)
	if n <= 0 {
		return
	}
	steps := n
	if steps > len(s.buckets) {
		steps = len(s.buckets)
	}
	for i := 0; i < steps; i++ {
		completed := s.buckets[s.bucketIdx]
		rate := float64(completed) / s.cfg.BucketWidth.Minutes()
		s.updateBaseline(rate)
		s.bucketIdx = (s.bucketIdx + 1) % len(s.buckets)
		s.buckets[s.bucketIdx] = 0
	}
	s.bucketStart = s.bucketStart.Add(time.Duration(n) * s.cfg.BucketWidth)
}

func (s *State) updateBaseline(observedRate float64) {
	if s.baselineSamples == 0 {
		s.baseline = observedRate
	} else {
		s.baseline = s.cfg.BaselineAlpha*observedRate + (1-s.cfg.BaselineAlpha)*s.baseline
	}
	s.baselineSamples++
}

func (s *State) currentRatePerMin() float64 {
	var sum uint32
	for _, b := range s.buckets {
		sum += b
	}
	windowMinutes := s.cfg.BucketWidth.Minutes() * float64(len(s.buckets))
	return float64(sum) / windowMinutes
}

func (s *State) windowEventCount() int {
	var sum uint32
	for _, b := range s.buckets {
		sum += b
	}
	return int(sum)
}

// Package pattern tracks per-fingerprint occurrence state - counts, rolling
// rate, and a slow baseline - in bounded memory, and decides when a
// pattern's current frequency represents an anomalous spike rather than
// routine repetition.
//
// Algorithm summary (see docs/TECHNICAL_METHOD.md for full rationale):
//
//  1. Each pattern keeps a ring buffer of fixed-width time buckets covering
//     a short "current" window (default 60s / 12x5s buckets).
//  2. As buckets age out of that window, their rate is folded into a slow
//     exponentially-weighted moving average (EWMA) that represents the
//     pattern's learned "normal" rate.
//  3. A spike is flagged when the current-window rate exceeds the baseline
//     by a configurable multiplier AND clears an absolute minimum count (so
//     a baseline near zero cannot be "exceeded" by a single stray event),
//     AND the pattern has existed long enough to have a meaningful
//     baseline, AND a per-pattern cooldown has elapsed since the last
//     alert for that pattern.
package pattern

import (
	"container/list"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/severity"
)

// Config tunes anomaly sensitivity and store bounds. Zero-value Config is
// invalid; use DefaultConfig().
type Config struct {
	BucketWidth        time.Duration
	WindowBuckets      int
	BaselineAlpha      float64
	WarmupDuration     time.Duration
	SpikeMultiplier    float64
	MinWindowEvents    int
	Cooldown           time.Duration
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
		BucketWidth:        5 * time.Second,
		WindowBuckets:      12, // 60s current-rate window
		BaselineAlpha:      0.05,
		WarmupDuration:     2 * time.Minute,
		SpikeMultiplier:    8.0,
		MinWindowEvents:    5,
		Cooldown:           60 * time.Second,
		MaxTrackedPatterns: 10000,
		ProtectRank:        int(severity.Error),
		ProtectMultiplier:  3.0,
	}
}

// Spike describes a detected frequency anomaly for one pattern.
type Spike struct {
	BaselinePerMin float64
	CurrentPerMin  float64
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

	cfg         Config
	buckets     []uint32
	bucketIdx   int
	bucketStart time.Time
	baseline    float64
	warm        bool
	lastAlert   time.Time

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

	if !s.warm && now.Sub(s.FirstSeen) >= s.cfg.WarmupDuration {
		s.warm = true
	}
	if !s.warm {
		return nil
	}

	current := s.currentRatePerMin()
	windowEvents := s.windowEventCount()
	if s.baseline <= 0 || windowEvents < s.cfg.MinWindowEvents {
		return nil
	}
	multiplier := s.cfg.SpikeMultiplier
	if s.Severity.Rank() >= s.cfg.ProtectRank {
		multiplier = s.cfg.ProtectMultiplier
	}
	if current < s.baseline*multiplier {
		return nil
	}
	if now.Sub(s.lastAlert) < s.cfg.Cooldown {
		return nil
	}
	s.lastAlert = now
	return &Spike{BaselinePerMin: s.baseline, CurrentPerMin: current}
}

// advance rotates the ring buffer forward to `now`, folding any buckets
// that fall out of the current window into the slow baseline EWMA.
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
		retiring := s.buckets[s.bucketIdx]
		rate := float64(retiring) / s.cfg.BucketWidth.Minutes()
		s.updateBaseline(rate)
		s.bucketIdx = (s.bucketIdx + 1) % len(s.buckets)
		s.buckets[s.bucketIdx] = 0
	}
	s.bucketStart = s.bucketStart.Add(time.Duration(n) * s.cfg.BucketWidth)
}

func (s *State) updateBaseline(observedRate float64) {
	if s.baseline == 0 {
		s.baseline = observedRate
		return
	}
	s.baseline = s.cfg.BaselineAlpha*observedRate + (1-s.cfg.BaselineAlpha)*s.baseline
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

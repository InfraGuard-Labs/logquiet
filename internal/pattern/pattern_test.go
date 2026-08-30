package pattern

import (
	"testing"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/severity"
)

func testConfig() Config {
	cfg := DefaultConfig()
	cfg.WarmupDuration = 30 * time.Second
	cfg.Cooldown = 10 * time.Second
	return cfg
}

// TestRoutineRepetitionDoesNotAlarm feeds a steady, unchanging rate for a
// long time and asserts no spike is ever flagged - the core "repetition is
// not automatically noise, but it is also not automatically an anomaly"
// safety property in reverse: routine steady-state must stay quiet.
func TestRoutineRepetitionDoesNotAlarm(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(1), "tmpl", severity.Info, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	for i := 0; i < 600; i++ { // 10 minutes at 1/sec
		now = now.Add(1 * time.Second)
		if spike := st.Record(now); spike != nil {
			t.Fatalf("unexpected spike at steady 1/sec rate, iteration %d: %+v", i, spike)
		}
	}
}

// TestSuddenFrequencySpikeIsDetected simulates a rare event (well under
// 1/min) that suddenly starts firing hundreds of times per minute, and
// asserts the spike is caught rather than silently absorbed as "just more
// repetition" - this is the CRITICAL SAFETY property from the spec.
func TestSuddenFrequencySpikeIsDetected(t *testing.T) {
	cfg := testConfig()
	cfg.WarmupDuration = 1 * time.Minute
	st := newState(cfg, fingerprint.ID(2), "database timeout", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)

	// Establish a rare baseline: one event every 40 seconds for 5 minutes.
	for i := 0; i < 8; i++ {
		now = now.Add(40 * time.Second)
		st.Record(now)
	}

	// Now hammer it: 280/min equivalent, i.e. roughly one every ~214ms.
	var gotSpike *Spike
	for i := 0; i < 300; i++ {
		now = now.Add(200 * time.Millisecond)
		if s := st.Record(now); s != nil {
			gotSpike = s
			break
		}
	}
	if gotSpike == nil {
		t.Fatalf("expected a frequency spike to be detected during the burst, got none")
	}
	if gotSpike.CurrentPerMin <= gotSpike.BaselinePerMin {
		t.Fatalf("spike current rate (%.2f) should exceed baseline (%.2f)", gotSpike.CurrentPerMin, gotSpike.BaselinePerMin)
	}
}

// TestNewPatternDuringWarmupDoesNotAlarm ensures a brand-new pattern that
// happens to arrive rapidly (which is common - e.g. a burst of startup
// logs) isn't immediately flagged as a spike before it has any baseline;
// novelty surfacing (a separate mechanism) is what makes new patterns
// visible, not the anomaly detector.
func TestNewPatternDuringWarmupDoesNotAlarm(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(3), "tmpl", severity.Info, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	for i := 0; i < 50; i++ {
		now = now.Add(50 * time.Millisecond)
		if spike := st.Record(now); spike != nil {
			t.Fatalf("spike flagged before warmup elapsed: %+v", spike)
		}
	}
}

// TestCooldownPreventsAlertFlooding ensures a sustained spike does not
// produce a new alert on every single subsequent occurrence.
func TestCooldownPreventsAlertFlooding(t *testing.T) {
	cfg := testConfig()
	cfg.WarmupDuration = 1 * time.Minute
	cfg.Cooldown = 30 * time.Second
	st := newState(cfg, fingerprint.ID(4), "tmpl", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	for i := 0; i < 8; i++ {
		now = now.Add(40 * time.Second)
		st.Record(now)
	}

	alerts := 0
	for i := 0; i < 600; i++ {
		now = now.Add(200 * time.Millisecond)
		if s := st.Record(now); s != nil {
			alerts++
		}
	}
	if alerts == 0 {
		t.Fatalf("expected at least one alert during sustained spike")
	}
	// 600 iterations * 200ms = 120s of sustained spike with a 30s cooldown
	// should produce roughly 4-5 alerts, never anywhere near 600.
	if alerts > 10 {
		t.Fatalf("cooldown did not bound alert count: got %d alerts", alerts)
	}
}

func TestStoreEvictsOldestBeyondBound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTrackedPatterns = 3
	st := NewStore(cfg)
	now := time.Unix(0, 0)

	for i := 0; i < 5; i++ {
		fp := fingerprint.ID(i + 1)
		st.GetOrCreate(fp, "tmpl", severity.Info, nil, now)
	}
	if st.Len() != 3 {
		t.Fatalf("store size = %d, want bounded at 3", st.Len())
	}
	if st.Evicted() != 2 {
		t.Fatalf("evicted count = %d, want 2", st.Evicted())
	}

	// The most recently created patterns (4, 5) plus the touched one
	// should still be present; the oldest (1, 2) should be gone.
	if _, isNew := st.GetOrCreate(fingerprint.ID(5), "tmpl", severity.Info, nil, now); isNew {
		t.Fatalf("fingerprint 5 should still be tracked (recently created)")
	}
	if _, isNew := st.GetOrCreate(fingerprint.ID(1), "tmpl", severity.Info, nil, now); !isNew {
		t.Fatalf("fingerprint 1 should have been evicted and treated as new on reappearance")
	}
}

func TestStoreLRUTouchOnAccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTrackedPatterns = 2
	st := NewStore(cfg)
	now := time.Unix(0, 0)

	st.GetOrCreate(fingerprint.ID(1), "a", severity.Info, nil, now)
	st.GetOrCreate(fingerprint.ID(2), "b", severity.Info, nil, now)
	// touch fp1 so it becomes most-recently-used
	st.GetOrCreate(fingerprint.ID(1), "a", severity.Info, nil, now)
	// adding fp3 should evict fp2 (least recently used), not fp1
	st.GetOrCreate(fingerprint.ID(3), "c", severity.Info, nil, now)

	if _, isNew := st.GetOrCreate(fingerprint.ID(1), "a", severity.Info, nil, now); isNew {
		t.Fatalf("fp1 should have survived eviction (recently touched)")
	}
	if _, isNew := st.GetOrCreate(fingerprint.ID(2), "b", severity.Info, nil, now); !isNew {
		t.Fatalf("fp2 should have been evicted (least recently used)")
	}
}

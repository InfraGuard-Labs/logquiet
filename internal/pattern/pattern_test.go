package pattern

import (
	"testing"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/severity"
)

func testConfig() Config {
	return DefaultConfig()
}

// --- A: stable routine pattern -> no anomaly -------------------------------

func TestA_StableRoutinePatternNeverAlarms(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(1), "heartbeat ok", severity.Info, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	for i := 0; i < 600; i++ { // 10 minutes at 1/sec, perfectly steady
		now = now.Add(1 * time.Second)
		if spike := st.Record(now); spike != nil {
			t.Fatalf("unexpected spike at steady 1/sec rate, iteration %d: %+v", i, spike)
		}
	}
}

// --- B: established low-rate pattern -> sudden large increase -> anomaly ---

func TestB_EstablishedLowRatePatternThenSuddenSpikeIsDetected(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(2), "database timeout", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)

	// Establish a rare, real baseline: one event every 45s for 3 minutes.
	for i := 0; i < 4; i++ {
		now = now.Add(45 * time.Second)
		st.Record(now)
	}
	if st.baselineSamples < cfg.MinBaselineSamples {
		t.Fatalf("test setup: expected a real baseline to have formed by now, got %d samples", st.baselineSamples)
	}

	// Now hammer it: a sudden burst, several events per second.
	var got *Spike
	for i := 0; i < 300; i++ {
		now = now.Add(200 * time.Millisecond)
		if s := st.Record(now); s != nil {
			got = s
			break
		}
	}
	if got == nil {
		t.Fatalf("expected a frequency spike to be detected during the burst")
	}
	if got.Bootstrap {
		t.Fatalf("expected a standard (non-bootstrap) spike since a real baseline existed, got bootstrap=%v baseline=%v", got.Bootstrap, got.BaselinePerMin)
	}
	if got.CurrentPerMin <= got.BaselinePerMin {
		t.Fatalf("spike current rate (%.2f) should exceed baseline (%.2f)", got.CurrentPerMin, got.BaselinePerMin)
	}
}

// --- C: newly appearing severe ERROR/CRITICAL burst -> high-signal ---------

func TestC_BrandNewSevereErrorBurstIsFlaggedImmediately(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(3), "database timeout on host 10.0.1.45", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)

	// This pattern has NEVER been seen before - no prior history at all -
	// and immediately starts firing rapidly (the exact scenario reported:
	// "a rapid burst of repeated ERROR database timeout events").
	var got *Spike
	var elapsed time.Duration
	for i := 0; i < 50; i++ {
		now = now.Add(100 * time.Millisecond)
		elapsed += 100 * time.Millisecond
		if s := st.Record(now); s != nil {
			got = s
			break
		}
	}
	if got == nil {
		t.Fatalf("expected a brand-new severe error burst to be flagged without waiting for a learned baseline")
	}
	if !got.Bootstrap {
		t.Fatalf("expected a bootstrap spike (no real baseline yet), got Bootstrap=false with baseline=%.2f", got.BaselinePerMin)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("expected detection within a few seconds of the burst starting, took %s", elapsed)
	}
}

// TestC_NewErrorTypeAFewTimesDoesNotBootstrap ensures the bootstrap path
// requires genuine sustained volume, not just "a new error appeared a
// couple of times" - avoiding noisy false positives for ordinary new
// error occurrences (Config.MinBootstrapEvents).
func TestC_NewErrorTypeAFewTimesDoesNotBootstrap(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(4), "one-off validation error", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	for i := 0; i < 3; i++ { // well under MinBootstrapEvents
		now = now.Add(5 * time.Second)
		if s := st.Record(now); s != nil {
			t.Fatalf("did not expect a bootstrap spike from only a handful of occurrences: %+v", s)
		}
	}
}

// --- D: routine high-frequency INFO pattern -> no false anomaly ------------

func TestD_RoutineHighFrequencyInfoNeverFalselyAlarms(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(5), "handled request", severity.Info, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	// 20/sec sustained from the very first event, for a long time. Ordinary
	// severity, so the bootstrap path must never apply regardless of how
	// high the absolute rate is, and once a real baseline forms it should
	// reflect this same rate, so the standard path shouldn't fire either.
	for i := 0; i < 5000; i++ {
		now = now.Add(50 * time.Millisecond)
		if s := st.Record(now); s != nil {
			t.Fatalf("false positive on routine high-frequency INFO pattern at iteration %d: %+v", i, s)
		}
	}
}

// --- E: short-lived startup streams -----------------------------------------

// TestE_ShortStartupBurstOrdinarySeverityNoFalsePositive: a brief burst of
// ordinary-severity startup chatter in a stream that ends quickly must not
// be misflagged - there isn't enough history for a real baseline, and
// ordinary severities never use the bootstrap path.
func TestE_ShortStartupBurstOrdinarySeverityNoFalsePositive(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(6), "worker starting up", severity.Info, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	for i := 0; i < 20; i++ {
		now = now.Add(50 * time.Millisecond) // finishes in 1 second
		if s := st.Record(now); s != nil {
			t.Fatalf("unexpected spike in a short ordinary-severity startup burst: %+v", s)
		}
	}
}

// TestE_ShortStartupStreamSevereBurstStillDetected: a stream that runs for
// only a few seconds total must still be able to flag a genuine severe
// burst via the bootstrap path - "short-lived" must not mean "blind".
func TestE_ShortStartupStreamSevereBurstStillDetected(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(7), "panic: cannot connect to config store", severity.Fatal, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)
	var got *Spike
	for i := 0; i < 20; i++ {
		now = now.Add(100 * time.Millisecond) // whole test spans 2 seconds
		if s := st.Record(now); s != nil {
			got = s
			break
		}
	}
	if got == nil {
		t.Fatalf("expected a severe startup burst to be caught even in a 2-second-long stream")
	}
}

// --- F: long-running streams -------------------------------------------------

// TestF_LongRunningStreamCooldownBoundsAlertCount ensures a sustained spike
// over a long session produces periodic alerts, not one per occurrence,
// and that the baseline keeps adapting sensibly over the session's life.
func TestF_LongRunningStreamCooldownBoundsAlertCount(t *testing.T) {
	cfg := testConfig()
	st := newState(cfg, fingerprint.ID(8), "database timeout", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)

	// Establish a real, low baseline over several minutes.
	for i := 0; i < 8; i++ {
		now = now.Add(40 * time.Second)
		st.Record(now)
	}

	alerts := 0
	for i := 0; i < 1200; i++ { // 4 minutes of sustained 5/sec spike
		now = now.Add(200 * time.Millisecond)
		if s := st.Record(now); s != nil {
			alerts++
		}
	}
	if alerts == 0 {
		t.Fatalf("expected at least one alert during a long sustained spike")
	}
	// 1200 * 200ms = 240s of sustained spike with a 60s cooldown should
	// produce roughly 4 alerts, never anywhere near 1200.
	if alerts > 10 {
		t.Fatalf("cooldown did not bound alert count over a long session: got %d alerts", alerts)
	}
}

// TestF_LongRunningStreamBaselineEventuallyAdapts verifies that a rate
// change sustained long enough eventually becomes the pattern's own
// baseline (the EWMA catches up), so a permanently-elevated-but-stable
// rate does not alert forever.
func TestF_LongRunningStreamBaselineEventuallyAdapts(t *testing.T) {
	cfg := testConfig()
	cfg.Cooldown = 1 * time.Second // shrink cooldown so we can observe convergence faster in the test
	st := newState(cfg, fingerprint.ID(9), "database timeout", severity.Error, nil, time.Unix(0, 0))
	now := time.Unix(0, 0)

	for i := 0; i < 8; i++ {
		now = now.Add(40 * time.Second)
		st.Record(now)
	}

	// Sustain an elevated (but constant) rate for a long time.
	sawSpike := false
	quietStreak := 0
	maxQuietStreak := 0
	for i := 0; i < 20000; i++ {
		now = now.Add(500 * time.Millisecond) // 2/sec sustained
		if s := st.Record(now); s != nil {
			sawSpike = true
			quietStreak = 0
		} else {
			quietStreak++
			if quietStreak > maxQuietStreak {
				maxQuietStreak = quietStreak
			}
		}
	}
	if !sawSpike {
		t.Fatalf("expected the initial rate change to be flagged at least once")
	}
	// Once the baseline has converged to the new steady rate, alerts
	// should stop entirely for a long trailing stretch.
	if maxQuietStreak < 5000 {
		t.Fatalf("expected the baseline to converge and alerts to stop for a long trailing stretch, longest quiet streak was only %d records", maxQuietStreak)
	}
}

// --- store tests (unaffected by the anomaly-detection algorithm) -----------

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
	st.GetOrCreate(fingerprint.ID(1), "a", severity.Info, nil, now)
	st.GetOrCreate(fingerprint.ID(3), "c", severity.Info, nil, now)

	if _, isNew := st.GetOrCreate(fingerprint.ID(1), "a", severity.Info, nil, now); isNew {
		t.Fatalf("fp1 should have survived eviction (recently touched)")
	}
	if _, isNew := st.GetOrCreate(fingerprint.ID(2), "b", severity.Info, nil, now); !isNew {
		t.Fatalf("fp2 should have been evicted (least recently used)")
	}
}

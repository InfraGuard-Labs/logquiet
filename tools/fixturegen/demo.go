package main

// Demo fixtures for the README's visual before/after, frequency-spike, and
// stack-trace-preservation demonstrations (see demo/README.md). These are
// intentionally hand-shaped (not randomly generated) so a screenshot taken
// against them is reproducible byte-for-byte, and small enough to make a
// clean, readable terminal screenshot rather than an overwhelming wall of
// text. All content is synthetic/fabricated - see fixtures/synthetic/README.md
// for the same disclosure that applies here.

import (
	"fmt"
	"path/filepath"
)

func genDemo() {
	dir := "demo/fixtures"
	demoNoisyApp(dir)
	demoFrequencySpike(dir)
	demoStackTrace(dir)
}

// demoTS returns a fixed, deterministic timestamp string - never time.Now() -
// so regenerating these fixtures produces byte-identical output every time.
func demoTS(sec int) string {
	h := 9 + sec/3600
	m := (sec / 60) % 60
	s := sec % 60
	return fmt.Sprintf("2026-08-30 %02d:%02d:%02d", h, m, s)
}

// demoNoisyApp is the BEFORE/AFTER scenario: a realistic, high-volume
// stream of health-check and DB-status chatter with one buried CRITICAL
// failure and its full traceback.
func demoNoisyApp(dir string) {
	var lines []string
	sec := 0
	add := func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf("%s %s", demoTS(sec), fmt.Sprintf(format, args...)))
		sec++
	}

	for i := 0; i < 12; i++ {
		add("[INFO] Connection pool active. 42 connections open.")
	}
	add("[INFO] User 48213 requested page /dashboard")
	for i := 0; i < 8; i++ {
		add("[INFO] Connection pool active. 42 connections open.")
	}
	add("[INFO] Health check ok: /healthz 200 3ms")
	for i := 0; i < 6; i++ {
		add("[INFO] Connection pool active. 42 connections open.")
	}

	// The buried failure.
	add("[CRITICAL] DATABASE TIMEOUT: Host 10.0.1.45 failed to respond in 5000ms.")
	lines = append(lines, fmt.Sprintf("%s [ERROR] Traceback (most recent call last):", demoTS(sec)))
	sec++
	lines = append(lines, `  File "/app/db.py", line 42, in execute_query`)
	lines = append(lines, `    raise TimeoutError("DB connection lost")`)
	lines = append(lines, `TimeoutError: DB connection lost`)

	for i := 1; i <= 4; i++ {
		add("[WARNING] Retrying connection attempt %d...", i)
	}
	add("[INFO] Connection pool recovered. 41 connections open.")
	for i := 0; i < 10; i++ {
		add("[INFO] Connection pool active. 41 connections open.")
	}

	writeLines(filepath.Join(dir, "noisy-app.log"), lines)
}

// demoFrequencySpike is the anomaly-detection scenario: routine chatter,
// then a brand-new error class bursting at high volume with zero prior
// history - the exact shape the bootstrap anomaly path exists for (see
// docs/TECHNICAL_METHOD.md section 7). Deliberately has no delays and no
// dependence on wall-clock pacing: because the bootstrap path only needs
// enough *events* to accumulate, not real elapsed time, this fixture
// reproduces the same anomaly deterministically no matter how fast it is
// read.
func demoFrequencySpike(dir string) {
	var lines []string
	sec := 0
	add := func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf("%s %s", demoTS(sec), fmt.Sprintf(format, args...)))
		sec++
	}

	for i := 0; i < 18; i++ {
		add("[INFO] request handled path=/api/v1/orders status=200 dur=12ms")
	}
	add("[INFO] User 90210 requested page /account")
	for i := 0; i < 10; i++ {
		add("[INFO] request handled path=/api/v1/orders status=200 dur=14ms")
	}

	// A structural pattern with zero prior history in this stream,
	// suddenly repeating at high volume.
	for i := 0; i < 20; i++ {
		add("[ERROR] database timeout on host 10.0.1.45: connection refused")
	}

	writeLines(filepath.Join(dir, "frequency-spike.log"), lines)
}

// demoStackTrace is the multiline-preservation scenario: routine chatter
// before and after, with one full Python traceback buried in the middle,
// prefixed per-line the way Docker/Kubernetes actually emit it (every raw
// line - including the traceback's own indented lines - gets an identical
// per-line timestamp) - the exact shape documented in
// docs/TECHNICAL_METHOD.md section 1 as a real bug this project found and
// fixed.
func demoStackTrace(dir string) {
	var lines []string
	sec := 0
	add := func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf("%s %s", demoTS(sec), fmt.Sprintf(format, args...)))
		sec++
	}

	for i := 0; i < 16; i++ {
		add("[INFO] worker received task task-%d", 5000+i)
	}

	ts := demoTS(sec)
	lines = append(lines, ts+" [ERROR] Traceback (most recent call last):")
	lines = append(lines, ts+`   File "/app/worker.py", line 88, in process`)
	lines = append(lines, ts+`     result = transform(payload)`)
	lines = append(lines, ts+`   File "/app/transform.py", line 12, in transform`)
	lines = append(lines, ts+`     return payload["amount"] / payload["rate"]`)
	lines = append(lines, ts+" [ERROR] ZeroDivisionError: division by zero")
	sec++

	for i := 0; i < 16; i++ {
		add("[INFO] worker received task task-%d", 6000+i)
	}

	writeLines(filepath.Join(dir, "stack-trace.log"), lines)
}

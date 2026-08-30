package multiline

import (
	"testing"

	"github.com/azimsiddiqui/logquiet/internal/logline"
)

// feedAll feeds raw lines through Extract+Assembler exactly as the pipeline
// does, returning every completed block including the final flush.
func feedAll(t *testing.T, lines []string) []Block {
	t.Helper()
	var a Assembler
	var blocks []Block
	for _, raw := range lines {
		_, content := logline.Extract(raw)
		if b, ok := a.Feed(raw, content); ok {
			blocks = append(blocks, b)
		}
	}
	if b, ok := a.Flush(); ok {
		blocks = append(blocks, b)
	}
	return blocks
}

func TestPythonTracebackGrouping(t *testing.T) {
	lines := []string{
		`2026-08-30 03:01:07 [ERROR] Traceback (most recent call last):`,
		`2026-08-30 03:01:07 [ERROR]   File "/app/db.py", line 42, in execute_query`,
		`2026-08-30 03:01:07 [ERROR]     raise TimeoutError("DB connection lost")`,
		`2026-08-30 03:01:07 [ERROR] TimeoutError: DB connection lost`,
		`2026-08-30 03:01:08 [WARNING] Retrying connection attempt 1...`,
	}
	blocks := feedAll(t, lines)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (traceback + warning); blocks=%+v", len(blocks), blocks)
	}
	if len(blocks[0].Lines) != 4 {
		t.Fatalf("traceback block has %d lines, want 4: %+v", len(blocks[0].Lines), blocks[0].Lines)
	}
	if len(blocks[1].Lines) != 1 {
		t.Fatalf("warning block should stand alone, got %d lines", len(blocks[1].Lines))
	}
}

func TestJavaStackTraceGrouping(t *testing.T) {
	lines := []string{
		`2026-08-30 03:01:07 ERROR java.lang.NullPointerException: Cannot invoke "String.length()"`,
		`	at com.example.Service.process(Service.java:42)`,
		`	at com.example.Main.main(Main.java:10)`,
		`Caused by: java.lang.RuntimeException: root cause`,
		`	... 3 more`,
		`2026-08-30 03:01:09 INFO server resumed`,
	}
	blocks := feedAll(t, lines)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2; blocks=%+v", len(blocks), blocks)
	}
	if len(blocks[0].Lines) != 5 {
		t.Fatalf("java trace block has %d lines, want 5", len(blocks[0].Lines))
	}
}

func TestGoPanicGrouping(t *testing.T) {
	lines := []string{
		`panic: runtime error: index out of range [5] with length 3`,
		``,
		`goroutine 1 [running]:`,
		`main.process(...)`,
		`	/app/main.go:42 +0x1b`,
		`main.main()`,
		`	/app/main.go:10 +0x65`,
		`exit status 2`,
		`2026-08-30 03:02:00 [INFO] restarted`,
	}
	blocks := feedAll(t, lines)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2; blocks=%+v", len(blocks), blocks)
	}
	if len(blocks[0].Lines) != 8 {
		t.Fatalf("go panic block has %d lines, want 8: %+v", len(blocks[0].Lines), blocks[0].Lines)
	}
}

func TestUnrelatedSingleLinesStayIndependent(t *testing.T) {
	lines := []string{
		`2026-08-30 03:01:00 [INFO] Connection pool active. 42 connections open.`,
		`2026-08-30 03:01:01 [INFO] Connection pool active. 42 connections open.`,
		`2026-08-30 03:01:05 [INFO] User 10829 requested page /dashboard`,
	}
	blocks := feedAll(t, lines)
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3 independent single-line events", len(blocks))
	}
	for _, b := range blocks {
		if len(b.Lines) != 1 {
			t.Fatalf("expected single-line block, got %d lines: %+v", len(b.Lines), b.Lines)
		}
	}
}

func TestMaxBlockLinesBounds(t *testing.T) {
	lines := []string{`panic: runaway`}
	for i := 0; i < MaxBlockLines+50; i++ {
		lines = append(lines, `	/app/x.go:1 +0x1`)
	}
	blocks := feedAll(t, lines)
	if len(blocks) < 1 {
		t.Fatalf("expected at least one block to be forced out at the bound")
	}
	for _, b := range blocks {
		if len(b.Lines) > MaxBlockLines {
			t.Fatalf("block exceeded MaxBlockLines: %d", len(b.Lines))
		}
	}
}

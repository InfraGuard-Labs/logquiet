// Command logquiet is a local, zero-configuration Unix-style filter that
// makes noisy live application and infrastructure logs readable in real
// time. See README.md for usage and docs/ARCHITECTURE.md for design.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/config"
	"github.com/azimsiddiqui/logquiet/internal/pipeline"
	"github.com/azimsiddiqui/logquiet/internal/reader"
	"github.com/azimsiddiqui/logquiet/internal/render"
	"github.com/azimsiddiqui/logquiet/internal/stats"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	cfg, err := config.Parse(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "logquiet: %v\n", err)
		return 2
	}
	if cfg.ShowVersion {
		fmt.Fprintf(stdout, "logquiet %s\n", version)
		return 0
	}

	input, closeInput, err := openInput(cfg.InputFile, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "logquiet: %v\n", err)
		return 1
	}
	defer closeInput()

	isTTY := isTerminal(stdout) || cfg.Color
	ropts := render.DefaultOptions()
	ropts.IsTTY = isTTY
	ropts.Plain = cfg.Plain || !isTTY
	ropts.NoColor = cfg.NoColor
	ropts.JSON = cfg.JSON

	rnd := render.New(stdout, ropts)
	pl := pipeline.New(cfg, rnd)

	var mu sync.Mutex
	finished := false
	finish := func() {
		mu.Lock()
		defer mu.Unlock()
		if finished {
			return
		}
		finished = true
		pl.Finish(time.Now())
		if cfg.Stats {
			printStats(stderr, pl, time.Now())
		}
		if cfg.ImpactReportPath != "" {
			if err := writeImpactReport(cfg.ImpactReportPath, pl, time.Now()); err != nil {
				fmt.Fprintf(stderr, "logquiet: impact report: %v\n", err)
			}
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		finish()
		os.Exit(0)
	}()

	scanErr := processStream(input, pl, &mu)

	finish()
	if scanErr != nil {
		fmt.Fprintf(stderr, "logquiet: error reading input: %v\n", scanErr)
		return 1
	}
	return 0
}

// processStream reads lines from r and feeds them to the pipeline until
// EOF, the downstream renderer's output pipe breaks, or a read error.
// mu guards pl and must be the same mutex the caller's signal handler and
// finish() use.
//
// It returns the scanner's terminal error, or nil on a clean EOF.
// bufio.Scanner.Scan returns false on both clean EOF and a genuine read
// error and does not distinguish the two on its own - Err must be checked
// explicitly by the caller, or a real input failure (a disk read error, a
// device disconnecting, or any other error the underlying io.Reader
// returns) is silently indistinguishable from normal end-of-stream, and
// logquiet would exit 0 as if the whole input had been processed
// successfully when it may have stopped partway through.
func processStream(r io.Reader, pl *pipeline.Pipeline, mu *sync.Mutex) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), reader.MaxLineBytes+4096)
	scanner.Split(reader.BoundedLines(reader.MaxLineBytes, func() {
		mu.Lock()
		pl.Counters.TruncatedLines++
		mu.Unlock()
	}))

	for scanner.Scan() {
		mu.Lock()
		pl.ProcessLine(scanner.Text(), time.Now())
		broken := pl.RendererBroken()
		mu.Unlock()
		if broken {
			break
		}
	}
	return scanner.Err()
}

func openInput(path string, stdin *os.File) (*os.File, func(), error) {
	if path == "" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func printStats(w *os.File, pl *pipeline.Pipeline, now time.Time) {
	snap := pl.Counters.Snapshot(now, pl.PatternCount(), pl.PatternsEvicted())
	fmt.Fprintf(w, "\n--- logquiet stats ---\n")
	fmt.Fprintf(w, "%-27s %d\n", "input lines:", snap.InputLines)
	fmt.Fprintf(w, "%-27s %d\n", "displayed events:", snap.DisplayedEvents)
	fmt.Fprintf(w, "%-27s %d\n", "suppressed events:", snap.SuppressedEvents)
	fmt.Fprintf(w, "%-27s %.1f%%\n", "logical-event suppression:", snap.LogicalEventSuppressionPercent)
	// Distinct from the line above: this counts actual raw physical lines,
	// including every line of a collapsed multiline block, not just the
	// block itself - see docs/IMPACT_REPORT.md "Physical lines vs. logical
	// events" for why the two numbers legitimately differ.
	fmt.Fprintf(w, "%-27s %.1f%%\n", "raw-line suppression:", snap.RawLineSuppressionPercent)
	fmt.Fprintf(w, "%-27s %d (evicted %d)\n", "structural patterns:", snap.StructuralPatterns, snap.PatternsEvicted)
	// "observed", not "surfaced" or "displayed": these count every WARN/
	// ERROR+ occurrence seen in the input, including ones collapsed into a
	// repeat counter rather than shown individually - see
	// docs/IMPACT_REPORT.md for the precise definition.
	fmt.Fprintf(w, "%-27s %d\n", "warning events observed:", snap.WarningEvents)
	fmt.Fprintf(w, "%-27s %d\n", "error events observed:", snap.ErrorEvents)
	fmt.Fprintf(w, "%-27s %d\n", "anomalies detected:", snap.AnomalyEvents)
	fmt.Fprintf(w, "%-27s %d\n", "truncated lines:", snap.TruncatedLines)
	fmt.Fprintf(w, "%-27s %.2fs\n", "elapsed:", snap.ElapsedSeconds)
	fmt.Fprintf(w, "%-27s %.0f lines/sec, %.2f MB/sec\n", "rate:", snap.LinesPerSecond, snap.MBPerSecond)
	fmt.Fprintf(w, "%-27s %.1f MB\n", "approx memory:", snap.ApproxMemoryMB)
}

func writeImpactReport(path string, pl *pipeline.Pipeline, now time.Time) error {
	snap := pl.Counters.Snapshot(now, pl.PatternCount(), pl.PatternsEvicted())
	report := stats.BuildImpactReport(version, snap, now)
	b, err := report.MarshalIndent()
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

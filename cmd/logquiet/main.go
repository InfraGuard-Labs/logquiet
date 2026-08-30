// Command logquiet is a local, zero-configuration Unix-style filter that
// makes noisy live application and infrastructure logs readable in real
// time. See README.md for usage and docs/ARCHITECTURE.md for design.
package main

import (
	"bufio"
	"flag"
	"fmt"
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

	isTTY := isTerminal(stdout)
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

	scanner := bufio.NewScanner(input)
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

	finish()
	return 0
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
	fmt.Fprintf(w, "input lines:          %d\n", snap.InputLines)
	fmt.Fprintf(w, "displayed events:     %d\n", snap.DisplayedEvents)
	fmt.Fprintf(w, "suppressed events:    %d\n", snap.SuppressedEvents)
	fmt.Fprintf(w, "suppression:          %.1f%%\n", snap.SuppressionPercent)
	fmt.Fprintf(w, "structural patterns:  %d (evicted %d)\n", snap.StructuralPatterns, snap.PatternsEvicted)
	fmt.Fprintf(w, "warnings surfaced:    %d\n", snap.WarningEvents)
	fmt.Fprintf(w, "errors surfaced:      %d\n", snap.ErrorEvents)
	fmt.Fprintf(w, "anomalies detected:   %d\n", snap.AnomalyEvents)
	fmt.Fprintf(w, "truncated lines:      %d\n", snap.TruncatedLines)
	fmt.Fprintf(w, "elapsed:              %.2fs\n", snap.ElapsedSeconds)
	fmt.Fprintf(w, "rate:                 %.0f lines/sec, %.2f MB/sec\n", snap.LinesPerSecond, snap.MBPerSecond)
	fmt.Fprintf(w, "approx memory:        %.1f MB\n", snap.ApproxMemoryMB)
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

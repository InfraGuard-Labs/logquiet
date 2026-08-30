// Command correctness runs LogQuiet's pipeline directly (in-process, same
// as the shipped binary's core) against every fixture in
// fixtures/synthetic/ that has a manifest, and verifies that every
// "known important" event survives to the output. It reports retention
// (the safety-critical metric - not compression ratio) alongside
// suppression and throughput. See docs/BENCHMARKS.md.
//
// Usage: go run ./benchmarks/correctness [--json out.json]
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/config"
	"github.com/azimsiddiqui/logquiet/internal/pipeline"
	"github.com/azimsiddiqui/logquiet/internal/render"
)

type manifest struct {
	Scenario       string   `json:"scenario"`
	TotalLines     int      `json:"total_lines"`
	KnownImportant []string `json:"known_important_substrings"`
	Description    string   `json:"description"`
}

type result struct {
	Scenario           string   `json:"scenario"`
	File               string   `json:"file"`
	InputLines         uint64   `json:"input_lines"`
	DisplayedEvents    uint64   `json:"displayed_events"`
	SuppressedEvents   uint64   `json:"suppressed_events"`
	SuppressionPercent float64  `json:"suppression_percentage"`
	StructuralPatterns int      `json:"structural_patterns"`
	KnownImportant     int      `json:"known_important_total"`
	Retained           int      `json:"known_important_retained"`
	Missed             []string `json:"missed"`
	ThroughputLPS      float64  `json:"throughput_lines_per_second"`
	ElapsedSeconds     float64  `json:"elapsed_seconds"`
}

func main() {
	jsonOut := flag.String("json", "", "write full results as JSON to this path")
	dir := flag.String("dir", "fixtures/synthetic", "fixtures directory")
	flag.Parse()

	manifests, err := filepath.Glob(filepath.Join(*dir, "*.manifest.json"))
	if err != nil || len(manifests) == 0 {
		fmt.Fprintln(os.Stderr, "no manifests found in", *dir)
		os.Exit(1)
	}
	sort.Strings(manifests)

	var results []result
	for _, mPath := range manifests {
		r, err := runOne(mPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", mPath, err)
			os.Exit(1)
		}
		results = append(results, r)
	}

	printReport(results)

	if *jsonOut != "" {
		b, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*jsonOut, b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "writing json:", err)
			os.Exit(1)
		}
	}

	for _, r := range results {
		if r.Retained != r.KnownImportant {
			fmt.Fprintf(os.Stderr, "\nFAIL: %s lost %d/%d known-important events\n", r.Scenario, r.KnownImportant-r.Retained, r.KnownImportant)
			os.Exit(1)
		}
	}
}

func runOne(manifestPath string) (result, error) {
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		return result{}, err
	}
	var m manifest
	if err := json.Unmarshal(mb, &m); err != nil {
		return result{}, err
	}
	logPath := strings.TrimSuffix(manifestPath, ".manifest.json") + ".log"
	f, err := os.Open(logPath)
	if err != nil {
		return result{}, err
	}
	defer f.Close()

	var buf strings.Builder
	ropts := render.DefaultOptions()
	ropts.JSON = true
	r := render.New(&buf, ropts)
	cfg := config.Default()
	p := pipeline.New(cfg, r)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	start := time.Now()
	for sc.Scan() {
		p.ProcessLine(sc.Text(), time.Now())
	}
	p.Finish(time.Now())
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		elapsed = 1e-9
	}

	snap := p.Counters.Snapshot(time.Now(), p.PatternCount(), p.PatternsEvicted())

	output := buf.String()
	retained := 0
	var missed []string
	for _, needle := range m.KnownImportant {
		if strings.Contains(output, needle) {
			retained++
		} else {
			missed = append(missed, needle)
		}
	}

	return result{
		Scenario:           m.Scenario,
		File:               filepath.Base(logPath),
		InputLines:         snap.InputLines,
		DisplayedEvents:    snap.DisplayedEvents,
		SuppressedEvents:   snap.SuppressedEvents,
		SuppressionPercent: snap.SuppressionPercent,
		StructuralPatterns: snap.StructuralPatterns,
		KnownImportant:     len(m.KnownImportant),
		Retained:           retained,
		Missed:             missed,
		ThroughputLPS:      float64(snap.InputLines) / elapsed,
		ElapsedSeconds:     elapsed,
	}, nil
}

func printReport(results []result) {
	fmt.Printf("%-32s %10s %10s %10s %8s %8s %10s\n",
		"scenario", "lines", "displayed", "suppress%", "patterns", "retained", "lines/sec")
	for _, r := range results {
		retainedStr := fmt.Sprintf("%d/%d", r.Retained, r.KnownImportant)
		fmt.Printf("%-32s %10d %10d %9.1f%% %8d %8s %10.0f\n",
			r.Scenario, r.InputLines, r.DisplayedEvents, r.SuppressionPercent, r.StructuralPatterns, retainedStr, r.ThroughputLPS)
		if len(r.Missed) > 0 {
			for _, m := range r.Missed {
				fmt.Printf("    MISSED: %q\n", m)
			}
		}
	}
}

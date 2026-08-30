// Package pipeline wires together line parsing, multiline assembly,
// structural normalization, fingerprinting, pattern tracking, anomaly
// detection, and rendering into the single per-line decision LogQuiet makes:
// show it in full, collapse it into a repeat counter, or raise it as a
// frequency-spike anomaly.
package pipeline

import (
	"strings"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/config"
	"github.com/azimsiddiqui/logquiet/internal/fingerprint"
	"github.com/azimsiddiqui/logquiet/internal/logline"
	"github.com/azimsiddiqui/logquiet/internal/multiline"
	"github.com/azimsiddiqui/logquiet/internal/normalize"
	"github.com/azimsiddiqui/logquiet/internal/pattern"
	"github.com/azimsiddiqui/logquiet/internal/render"
	"github.com/azimsiddiqui/logquiet/internal/severity"
	"github.com/azimsiddiqui/logquiet/internal/stats"
)

// Pipeline holds all per-run state. It is not safe for concurrent use.
type Pipeline struct {
	store    *pattern.Store
	renderer *render.Renderer
	ml       multiline.Assembler
	Counters *stats.Counters
}

// New constructs a Pipeline from a resolved Config and a Renderer that has
// already been configured for the correct output mode.
func New(cfg config.Config, r *render.Renderer) *Pipeline {
	pcfg := pattern.DefaultConfig()
	windowSeconds := cfg.WindowSeconds
	if windowSeconds < 5 {
		windowSeconds = 5
	}
	pcfg.WindowBuckets = windowSeconds / 5
	if pcfg.WindowBuckets < 1 {
		pcfg.WindowBuckets = 1
	}
	pcfg.SpikeMultiplier = cfg.SpikeMultiplier
	pcfg.MinWindowEvents = cfg.MinWindowEvents
	pcfg.Cooldown = cfg.Cooldown()
	pcfg.MaxTrackedPatterns = cfg.MaxPatterns
	pcfg.ProtectRank = cfg.SeverityProtect.Rank()
	pcfg.ProtectMultiplier = cfg.ProtectMultiplier

	return &Pipeline{
		store:    pattern.NewStore(pcfg),
		renderer: r,
		Counters: stats.New(),
	}
}

// ProcessLine feeds one raw input line (without its trailing newline) into
// the pipeline. now should be time.Now() in production and a fixed clock in
// tests for determinism.
func (p *Pipeline) ProcessLine(raw string, now time.Time) {
	p.Counters.InputLines++
	p.Counters.BytesRead += uint64(len(raw)) + 1

	_, content := logline.Extract(raw)
	completed, ok := p.ml.Feed(raw, content)
	if ok {
		p.handleBlock(completed, now)
	}
	p.renderer.Tick(now)
}

// Finish flushes any trailing in-progress multiline block and finalizes
// rendering. Call this once at EOF or shutdown.
func (p *Pipeline) Finish(now time.Time) {
	if b, ok := p.ml.Flush(); ok {
		p.handleBlock(b, now)
	}
	p.renderer.Finalize(now)
}

func (p *Pipeline) handleBlock(block multiline.Block, now time.Time) {
	if len(block.Lines) == 0 {
		return
	}
	lvl, _ := logline.Extract(block.Lines[0])

	tmplLines := make([]string, 0, len(block.Contents))
	for _, c := range block.Contents {
		tmplLines = append(tmplLines, normalize.Template(c))
	}
	template := strings.Join(tmplLines, "\n")

	fp := fingerprint.Of(lvl, template)
	state, isNew := p.store.GetOrCreate(fp, template, lvl, block.Contents, now)
	spike := state.Record(now)

	if lvl == severity.Warn {
		p.Counters.WarningEvents++
	} else if lvl.Rank() >= severity.Error.Rank() {
		p.Counters.ErrorEvents++
	}

	switch {
	case spike != nil:
		p.Counters.AnomalyEvents++
		p.Counters.DisplayedEvents++
		p.renderer.EmitAnomaly(render.Anomaly{
			Fingerprint:    fp,
			Severity:       lvl,
			Template:       template,
			RawLines:       block.Contents,
			BaselinePerMin: spike.BaselinePerMin,
			CurrentPerMin:  spike.CurrentPerMin,
		}, now)

	case isNew:
		p.Counters.DisplayedEvents++
		p.renderer.Emit(render.Event{
			Fingerprint: fp,
			Severity:    lvl,
			Template:    template,
			RawLines:    block.Contents,
			IsNew:       isNew,
		})

	default:
		p.Counters.SuppressedEvents++
		p.renderer.Accumulate(fp, lvl, template, now)
	}
}

// PatternCount returns the number of distinct structural patterns currently
// tracked (used for --stats and impact reports).
func (p *Pipeline) PatternCount() int { return p.store.Len() }

// PatternsEvicted returns how many patterns have been dropped to stay
// within the configured memory bound.
func (p *Pipeline) PatternsEvicted() uint64 { return p.store.Evicted() }

// RendererBroken reports whether output is going nowhere (closed pipe), so
// the caller can stop reading input early.
func (p *Pipeline) RendererBroken() bool { return p.renderer.Broken() }

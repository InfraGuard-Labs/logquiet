// Package config parses LogQuiet's command-line flags into a validated
// Config. The zero-configuration default (no flags at all) is designed to
// be good enough for everyday use; every flag here is optional tuning.
package config

import (
	"flag"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/InfraGuard-Labs/logquiet/internal/severity"
)

// Config is the fully-resolved set of options for one run.
type Config struct {
	InputFile string // "" means stdin

	Plain   bool
	NoColor bool
	Color   bool
	JSON    bool
	Stats   bool

	ImpactReportPath string

	WindowSeconds     int
	SpikeMultiplier   float64
	ProtectMultiplier float64
	SeverityProtect   severity.Level
	MaxPatterns       int
	MinWindowEvents   int
	CooldownSeconds   int

	ShowHelp    bool
	ShowVersion bool
}

// Default returns the zero-config defaults.
func Default() Config {
	return Config{
		WindowSeconds:     15,
		SpikeMultiplier:   8.0,
		ProtectMultiplier: 3.0,
		SeverityProtect:   severity.Error,
		MaxPatterns:       10000,
		MinWindowEvents:   5,
		CooldownSeconds:   60,
	}
}

// Parse parses args (typically os.Args[1:]) into a Config.
func Parse(args []string, out io.Writer) (Config, error) {
	cfg := Default()
	fs := flag.NewFlagSet("logquiet", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.BoolVar(&cfg.Plain, "plain", false, "plain-text output: no ANSI color, for pipes/CI/saved files (auto-enabled when stdout is not a terminal)")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "disable ANSI color only; periodic repeat summaries are shown either way")
	fs.BoolVar(&cfg.Color, "color", false, "force ANSI color even when stdout is not a terminal (e.g. piping through `less -R`, or capturing colored output to a file); -no-color still wins if both are given")
	fs.BoolVar(&cfg.JSON, "json", false, "emit newline-delimited JSON events instead of human-formatted text")
	fs.BoolVar(&cfg.Stats, "stats", false, "print a summary of processing statistics to stderr when the stream ends")

	fs.StringVar(&cfg.ImpactReportPath, "impact-report", "", "write an aggregate, content-free statistics report to `path` when the stream ends")

	fs.IntVar(&cfg.WindowSeconds, "window", cfg.WindowSeconds, "rolling window (seconds) used to compute a pattern's current event rate for anomaly detection")
	fs.Float64Var(&cfg.SpikeMultiplier, "spike-multiplier", cfg.SpikeMultiplier, "current-rate / baseline-rate ratio that triggers a frequency-spike alert for ordinary severities")
	fs.Float64Var(&cfg.ProtectMultiplier, "protect-spike-multiplier", cfg.ProtectMultiplier, "spike multiplier applied to severities at/above -severity-protect (lower = more sensitive)")
	protectStr := fs.String("severity-protect", cfg.SeverityProtect.String(), "minimum severity (WARN, ERROR, CRITICAL, FATAL) that gets more sensitive anomaly detection")
	fs.IntVar(&cfg.MaxPatterns, "max-patterns", cfg.MaxPatterns, "maximum number of distinct structural patterns tracked at once (bounds memory; oldest are evicted first)")
	fs.IntVar(&cfg.MinWindowEvents, "min-window-events", cfg.MinWindowEvents, "minimum events within the rolling window required before a spike can be flagged (avoids alerting on tiny baselines)")
	fs.IntVar(&cfg.CooldownSeconds, "cooldown", cfg.CooldownSeconds, "minimum seconds between repeated spike alerts for the same pattern")

	fs.BoolVar(&cfg.ShowVersion, "version", false, "print the version and exit")

	fs.Usage = func() { printUsage(out) }

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if lvl, ok := parseLevel(*protectStr); ok {
		cfg.SeverityProtect = lvl
	} else {
		return cfg, fmt.Errorf("invalid -severity-protect value %q (want WARN, ERROR, CRITICAL, or FATAL)", *protectStr)
	}

	rest := fs.Args()
	if len(rest) > 1 {
		return cfg, fmt.Errorf("too many arguments: expected at most one file, got %v", rest)
	}
	if len(rest) == 1 {
		cfg.InputFile = rest[0]
	}

	if err := validate(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// validate rejects nonsensical numeric configuration outright rather than
// silently clamping it into something "safe" - a value like
// -max-patterns 0 or -spike-multiplier -1 is a user mistake, not a
// tuning choice, and running with a silently-substituted value would
// produce behavior the user did not ask for and might not notice.
func validate(cfg Config) error {
	if cfg.MaxPatterns <= 0 {
		return fmt.Errorf("invalid -max-patterns value %d: must be greater than 0", cfg.MaxPatterns)
	}
	if cfg.WindowSeconds <= 0 {
		return fmt.Errorf("invalid -window value %d: must be greater than 0 (seconds)", cfg.WindowSeconds)
	}
	if cfg.MinWindowEvents <= 0 {
		return fmt.Errorf("invalid -min-window-events value %d: must be greater than 0", cfg.MinWindowEvents)
	}
	if cfg.CooldownSeconds < 0 {
		return fmt.Errorf("invalid -cooldown value %d: must be 0 or greater (seconds); 0 means no cooldown between alerts", cfg.CooldownSeconds)
	}
	if err := validateMultiplier("-spike-multiplier", cfg.SpikeMultiplier); err != nil {
		return err
	}
	if err := validateMultiplier("-protect-spike-multiplier", cfg.ProtectMultiplier); err != nil {
		return err
	}
	return nil
}

// validateMultiplier checks a spike-multiplier-shaped flag: it gates
// "current rate > baseline rate * multiplier", so a multiplier at or below
// 1 would flag any rate that merely equals or exceeds baseline as a
// "spike", which is not what a spike-detection threshold means, and NaN/Inf
// (both of which flag.Float64Var will happily parse from a command line
// like "-spike-multiplier NaN") would make the comparison meaningless.
func validateMultiplier(flagName string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("invalid %s value %v: must be a finite number greater than 1", flagName, v)
	}
	if v <= 1 {
		return fmt.Errorf("invalid %s value %v: must be greater than 1 (a spike must exceed baseline, not merely match it)", flagName, v)
	}
	return nil
}

func parseLevel(s string) (severity.Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "WARN", "WARNING":
		return severity.Warn, true
	case "ERROR", "ERR":
		return severity.Error, true
	case "CRITICAL", "CRIT":
		return severity.Critical, true
	case "FATAL", "PANIC", "EMERG":
		return severity.Fatal, true
	}
	return severity.Unknown, false
}

// Window returns the configured rolling window as a time.Duration.
func (c Config) Window() time.Duration {
	return time.Duration(c.WindowSeconds) * time.Second
}

// Cooldown returns the configured spike-alert cooldown as a time.Duration.
func (c Config) Cooldown() time.Duration {
	return time.Duration(c.CooldownSeconds) * time.Second
}

func printUsage(out io.Writer) {
	fmt.Fprint(out, `logquiet - make noisy live logs readable

USAGE
  <command> | logquiet [flags]
  logquiet [flags] <file>

EXAMPLES
  kubectl logs -f deployment/api | logquiet
  docker logs -f api | logquiet
  docker compose logs -f | logquiet
  tail -F application.log | logquiet
  journalctl -f | logquiet
  logquiet app.log

LogQuiet reads a log stream, structurally normalizes each line (timestamps,
UUIDs, IPs, IDs, durations, and similar variable data become placeholders),
collapses routine repetition into periodic compact repeat summaries, and
always surfaces new, error-level, or anomalously-frequent events. No
configuration is required.

FLAGS
`)
	fs := flag.NewFlagSet("logquiet", flag.ContinueOnError)
	fs.SetOutput(out)
	dummy := Default()
	fs.BoolVar(&dummy.Plain, "plain", false, "plain-text output for pipes, CI, and saved files (auto-enabled when stdout is not a terminal)")
	fs.BoolVar(&dummy.NoColor, "no-color", false, "disable ANSI color only")
	fs.BoolVar(&dummy.Color, "color", false, "force ANSI color even when stdout is not a terminal")
	fs.BoolVar(&dummy.JSON, "json", false, "newline-delimited JSON output")
	fs.BoolVar(&dummy.Stats, "stats", false, "print processing statistics to stderr at the end")
	fs.String("impact-report", "", "write an aggregate, content-free statistics report to this path")
	fs.Int("window", dummy.WindowSeconds, "anomaly rolling window in seconds")
	fs.Float64("spike-multiplier", dummy.SpikeMultiplier, "spike sensitivity for ordinary severities")
	fs.Float64("protect-spike-multiplier", dummy.ProtectMultiplier, "spike sensitivity for protected severities")
	fs.String("severity-protect", dummy.SeverityProtect.String(), "minimum severity treated as protected")
	fs.Int("max-patterns", dummy.MaxPatterns, "maximum distinct patterns tracked (memory bound)")
	fs.Int("min-window-events", dummy.MinWindowEvents, "minimum events before a spike can be flagged")
	fs.Int("cooldown", dummy.CooldownSeconds, "seconds between repeated spike alerts per pattern")
	fs.Bool("version", false, "print version and exit")
	fs.PrintDefaults()
}

// PrintUsage exposes the usage text for -help/-h.
func PrintUsage(out io.Writer) { printUsage(out) }

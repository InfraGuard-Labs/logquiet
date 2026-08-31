package config

import (
	"io"
	"testing"
)

// TestInvalidNumericFlagsRejected covers the exact examples that must not
// be able to run normally: invalid or nonsensical numeric configuration is
// a user mistake, not a tuning choice, and must be rejected before
// processing begins rather than silently clamped into something else.
func TestInvalidNumericFlagsRejected(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"max-patterns zero", []string{"--max-patterns", "0"}},
		{"max-patterns negative", []string{"--max-patterns", "-1"}},
		{"spike-multiplier negative", []string{"--spike-multiplier", "-1"}},
		{"spike-multiplier exactly one", []string{"--spike-multiplier", "1"}},
		{"spike-multiplier NaN", []string{"--spike-multiplier", "NaN"}},
		{"spike-multiplier +Inf", []string{"--spike-multiplier", "+Inf"}},
		{"protect-spike-multiplier zero", []string{"--protect-spike-multiplier", "0"}},
		{"window zero", []string{"--window", "0"}},
		{"window negative", []string{"--window", "-5"}},
		{"min-window-events zero", []string{"--min-window-events", "0"}},
		{"min-window-events negative", []string{"--min-window-events", "-3"}},
		{"cooldown negative", []string{"--cooldown", "-1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.args, io.Discard)
			if err == nil {
				t.Fatalf("Parse(%v) succeeded, want a rejection error", c.args)
			}
		})
	}
}

// TestValidNumericFlagsAccepted is the converse: legitimate tuning values,
// including edge-adjacent ones, must not be rejected by the new validation.
func TestValidNumericFlagsAccepted(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"defaults", nil},
		{"max-patterns one", []string{"--max-patterns", "1"}},
		{"spike-multiplier just above one", []string{"--spike-multiplier", "1.01"}},
		{"cooldown zero is a valid choice (no throttling)", []string{"--cooldown", "0"}},
		{"window small positive", []string{"--window", "1"}},
		{"min-window-events one", []string{"--min-window-events", "1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.args, io.Discard); err != nil {
				t.Fatalf("Parse(%v) = %v, want success", c.args, err)
			}
		})
	}
}

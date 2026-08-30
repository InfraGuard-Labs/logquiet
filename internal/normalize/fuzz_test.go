package normalize

import "testing"

func FuzzTemplate(f *testing.F) {
	seeds := []string{
		"",
		"2026-08-30 03:01:00 [INFO] Connection pool active. 42 connections open.",
		"User 10481 connected from 10.0.1.2",
		"deployed commit a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
		"\x00\x01\xff binary-ish \xfe data",
		"550e8400-e29b-41d4-a716-446655440000",
		"::1 fe80::1%eth0 2001:db8::8a2e:370:7334",
		"100% 99.9% -5 -5.5ms 0x1 0xDEADBEEF",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// Must never panic, and must terminate promptly (guards against a
		// pathological regex backtracking blowup on adversarial input).
		_ = Template(s)
	})
}

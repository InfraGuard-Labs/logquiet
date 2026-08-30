package normalize

import "testing"

func TestTemplateCollapsesVariables(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			// A purely numeric token normalizes to [NUM] regardless of the
			// surrounding word ("User", "port", "attempt", ...); LogQuiet
			// does not attempt semantic, keyword-based classification of
			// bare numbers into more specific classes like "user ID",
			// since that heuristic does not generalize across log formats
			// and languages. See docs/TECHNICAL_METHOD.md.
			"numeric-id-is-NUM",
			"User 10481 connected from 10.0.1.2",
			"User [NUM] connected from [IP]",
		},
		{
			// Apache/nginx combined log format. Without a dedicated rule,
			// the "HH:MM:SS" portion looks exactly like a colon-separated
			// IPv6 address and gets misclassified - found via manual
			// testing against a real nginx access log fixture.
			"apache-combined-log-timestamp",
			`192.168.1.10 - - [30/Aug/2026:03:01:00 +0000] "GET /x HTTP/1.1" 200 15`,
			`[IP] - - [TIMESTAMP] "GET /x HTTP/[NUM]" [NUM] [NUM]`,
		},
		{
			"uuid",
			"request 550e8400-e29b-41d4-a716-446655440000 completed",
			"request [UUID] completed",
		},
		{
			"duration-and-ip",
			"Host 10.0.1.45 failed to respond in 5000ms",
			"Host [IP] failed to respond in [DURATION]",
		},
		{
			"bytesize",
			"uploaded 42.5MB in total",
			"uploaded [SIZE] in total",
		},
		{
			"ipv4-port",
			"connecting to 10.0.1.9:5432",
			"connecting to [IP]:[PORT]",
		},
		{
			"iso-timestamp",
			"2026-08-30T03:01:00Z something happened",
			"[TIMESTAMP] something happened",
		},
		{
			"git-sha",
			"deployed commit a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
			"deployed commit [HASH]",
		},
		{
			"memory-address",
			"panic at 0xc0000a4000",
			"panic at [ADDR]",
		},
		{
			"percentage",
			"disk usage at 87% capacity",
			"disk usage at [PCT]% capacity",
		},
		{
			"bare-numbers",
			"retrying attempt 3 of 5",
			"retrying attempt [NUM] of [NUM]",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Template(c.in)
			if got != c.want {
				t.Fatalf("Template(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestNoCollisionOnUnrelatedMessages verifies that normalization does not
// destroy the distinction between genuinely different messages - the most
// important correctness property for the fingerprinter.
func TestNoCollisionOnUnrelatedMessages(t *testing.T) {
	a := Template("User 10481 connected from 10.0.1.2")
	b := Template("User 10481 disconnected from 10.0.1.2")
	if a == b {
		t.Fatalf("distinct verbs collapsed to the same template: %q", a)
	}

	c := Template("Connection pool active. 42 connections open.")
	d := Template("Connection pool exhausted. 0 connections open.")
	if c == d {
		t.Fatalf("distinct states collapsed to the same template: %q", c)
	}
}

// TestShortWordsWithDigitsPreserved guards against the alnum-id rule eating
// very short technical words that merely contain a digit (below the
// minimum length threshold, so common short tokens like "utf8" or "md5"
// survive untouched). Longer alphanumeric words that coincidentally look
// like identifiers (e.g. "sha256") are a known, documented over-eager case;
// see docs/TECHNICAL_METHOD.md "Normalization precision trade-offs".
func TestShortWordsWithDigitsPreserved(t *testing.T) {
	cases := []string{"utf8", "ipv6", "b64", "md5"}
	for _, w := range cases {
		got := Template("encoding is " + w + " ok")
		want := "encoding is " + w + " ok"
		if got != want {
			t.Fatalf("Template(%q) = %q, want unchanged %q", w, got, want)
		}
	}
}

func TestSameStructureSameTemplate(t *testing.T) {
	a := Template("10:00:01 User 10481 connected from 10.0.1.2")
	b := Template("10:00:02 User 49210 connected from 10.0.1.8")
	if a != b {
		t.Fatalf("structurally identical lines diverged: %q vs %q", a, b)
	}
}

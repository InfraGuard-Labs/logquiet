package severity

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Level
		found   bool
		wantRem string
	}{
		{"bracketed", "[INFO] hello", Info, true, "hello"},
		{"bracketed-lower", "[error] boom", Error, true, "boom"},
		{"bare-colon", "ERROR: boom", Error, true, "boom"},
		{"bare-space", "WARN something happened", Warn, true, "something happened"},
		{"with-timestamp", "2026-08-30 03:01:07 [CRITICAL] DB down", Critical, true, "DB down"},
		{"syslog-style", "Aug 30 03:01:00 myhost myapp[123]: ERROR disk full", Error, true, "disk full"},
		{"no-level", "just a plain message", Unknown, false, ""},
		{"path-not-level", "GET /info HTTP/1.1", Unknown, false, ""},
		{"qualified-name-not-level", "com.example.Info: something", Unknown, false, ""},
		{"fatal-alias-panic", "[PANIC] runtime error", Fatal, true, "runtime error"},
		{"warning-full-word", "[WARNING] retry", Warn, true, "retry"},
		{"preserves-indentation", "[ERROR]   File \"x.py\", line 1", Error, true, "  File \"x.py\", line 1"},
		{"single-space-only-consumed", "[ERROR]  double space", Error, true, " double space"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lvl, found, consumed := Detect(c.in)
			if lvl != c.want || found != c.found {
				t.Fatalf("Detect(%q) = (%v,%v), want (%v,%v)", c.in, lvl, found, c.want, c.found)
			}
			if found {
				rem := c.in[consumed:]
				if rem != c.wantRem {
					t.Fatalf("Detect(%q) remainder = %q, want %q", c.in, rem, c.wantRem)
				}
			}
		})
	}
}

func TestRank(t *testing.T) {
	if Unknown.Rank() != Info.Rank() {
		t.Fatalf("Unknown should rank as Info for suppression purposes")
	}
	if !(Trace.Rank() < Debug.Rank() && Debug.Rank() < Info.Rank() && Info.Rank() < Warn.Rank() &&
		Warn.Rank() < Error.Rank() && Error.Rank() < Critical.Rank() && Critical.Rank() < Fatal.Rank()) {
		t.Fatalf("severity ranks must be strictly increasing")
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, l := range []Level{Trace, Debug, Info, Notice, Warn, Error, Critical, Fatal} {
		s := l.String()
		if s == "" || s == "UNKNOWN" {
			t.Fatalf("level %d stringified to %q", l, s)
		}
	}
}

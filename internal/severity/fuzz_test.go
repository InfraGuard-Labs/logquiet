package severity

import "testing"

func FuzzDetect(f *testing.F) {
	seeds := []string{
		"",
		"[INFO] hello",
		"ERROR: boom",
		"\x00\x01[\xffERROR",
		"[[[[[[[[[[",
		"a b c d e f g h i j k l m n o p",
		"2026-08-30 03:01:07 [ERROR]   File \"x.py\"",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		lvl, found, consumed := Detect(s)
		if found && (consumed < 0 || consumed > len(s)) {
			t.Fatalf("Detect(%q) returned out-of-range consumed=%d", s, consumed)
		}
		_ = lvl
	})
}

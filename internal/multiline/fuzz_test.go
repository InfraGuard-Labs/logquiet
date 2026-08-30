package multiline

import (
	"strings"
	"testing"

	"github.com/azimsiddiqui/logquiet/internal/logline"
)

func FuzzAssembler(f *testing.F) {
	seeds := []string{
		"panic: x\n\ngoroutine 1 [running]:\nmain.main()\n\t/a.go:1\nexit status 2\n",
		"Traceback (most recent call last):\n  File \"x.py\", line 1\nValueError: x\n",
		"a\nb\nc\n",
		"\n\n\n",
		"\x00\x01\xff\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, blob string) {
		lines := strings.Split(blob, "\n")
		var a Assembler
		var totalIn, totalOut int
		for _, l := range lines {
			totalIn++
			_, content := logline.Extract(l)
			if b, ok := a.Feed(l, content); ok {
				totalOut += len(b.Lines)
				if len(b.Lines) != len(b.Contents) {
					t.Fatalf("block Lines/Contents length mismatch: %d vs %d", len(b.Lines), len(b.Contents))
				}
			}
		}
		if b, ok := a.Flush(); ok {
			totalOut += len(b.Lines)
		}
		if totalOut != totalIn {
			t.Fatalf("line conservation violated: fed %d, emitted %d", totalIn, totalOut)
		}
	})
}

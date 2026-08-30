package pipeline

import (
	"io"
	"testing"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/config"
	"github.com/azimsiddiqui/logquiet/internal/render"
)

func BenchmarkProcessLineRepetitive(b *testing.B) {
	ropts := render.DefaultOptions()
	ropts.Plain = true
	r := render.New(io.Discard, ropts)
	p := New(config.Default(), r)
	now := time.Unix(0, 0)
	line := "2026-08-30 03:01:00 [INFO] api: connection pool active, 42 connections open"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now = now.Add(time.Microsecond)
		p.ProcessLine(line, now)
	}
}

func BenchmarkProcessLineDiverse(b *testing.B) {
	ropts := render.DefaultOptions()
	ropts.Plain = true
	r := render.New(io.Discard, ropts)
	p := New(config.Default(), r)
	now := time.Unix(0, 0)
	lines := []string{
		"2026-08-30 03:01:00 [INFO] api: handled request id=863125 dur=411ms addr=10.153.7.201",
		"2026-08-30 03:01:00 [INFO] worker: processed job id=112233 dur=98ms addr=10.9.1.5",
		"2026-08-30 03:01:00 [INFO] cache: flushed batch id=778812 dur=5ms addr=10.2.9.1",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now = now.Add(time.Microsecond)
		p.ProcessLine(lines[i%len(lines)], now)
	}
}

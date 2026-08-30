package normalize

import "testing"

const benchLine = "2026-08-30 03:01:00 [INFO] worker: processed job id=863125 dur=411ms addr=10.153.7.201"

func BenchmarkTemplate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Template(benchLine)
	}
}

func BenchmarkSpanFinder(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = spanFinder.FindAllStringIndex(benchLine, -1)
	}
}

package reader

import (
	"bufio"
	"strings"
	"testing"
)

func scanAll(t *testing.T, input string, maxLen int) ([]string, int) {
	t.Helper()
	truncations := 0
	sc := bufio.NewScanner(strings.NewReader(input))
	sc.Buffer(make([]byte, 4096), maxLen+4096)
	sc.Split(BoundedLines(maxLen, func() { truncations++ }))
	var out []string
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	return out, truncations
}

func TestNormalLines(t *testing.T) {
	lines, trunc := scanAll(t, "one\ntwo\nthree\n", 1024)
	if trunc != 0 {
		t.Fatalf("unexpected truncations: %d", trunc)
	}
	want := []string{"one", "two", "three"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestNoTrailingNewline(t *testing.T) {
	lines, _ := scanAll(t, "one\ntwo", 1024)
	if len(lines) != 2 || lines[1] != "two" {
		t.Fatalf("got %v, want last line 'two' without trailing newline", lines)
	}
}

func TestOverlongLineIsTruncatedNotUnbounded(t *testing.T) {
	huge := strings.Repeat("x", 10_000)
	input := "short\n" + huge + "\nafter\n"
	lines, trunc := scanAll(t, input, 100)
	if trunc == 0 {
		t.Fatalf("expected at least one truncation")
	}
	if lines[0] != "short" {
		t.Fatalf("first line corrupted: %q", lines[0])
	}
	if len(lines[1]) > 200 { // 100 bytes + marker text, generously bounded
		t.Fatalf("truncated line was not bounded: len=%d", len(lines[1]))
	}
	if lines[2] != "after" {
		t.Fatalf("line after the overlong one was corrupted: %q", lines[2])
	}
}

func TestCRLFHandled(t *testing.T) {
	lines, _ := scanAll(t, "one\r\ntwo\r\n", 1024)
	for _, l := range lines {
		if strings.Contains(l, "\r") {
			t.Fatalf("carriage return leaked into line: %q", l)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	lines, trunc := scanAll(t, "", 1024)
	if len(lines) != 0 || trunc != 0 {
		t.Fatalf("expected no lines and no truncations for empty input, got %v %d", lines, trunc)
	}
}

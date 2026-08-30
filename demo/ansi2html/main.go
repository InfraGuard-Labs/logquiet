// Command ansi2html converts real logquiet terminal output (captured with
// `logquiet --color`, which forces the same ANSI codes a real terminal
// session would see) into an HTML fragment for embedding in a terminal-
// mockup screenshot page. It understands exactly the SGR codes LogQuiet's
// renderer emits (internal/render/render.go: reset, bold, dim, yellow,
// red) - nothing more - since the goal is a faithful reproduction of real
// output, not a general-purpose ANSI terminal emulator.
//
// Usage: go run ./demo/ansi2html < captured-output.txt > fragment.html
package main

import (
	"bufio"
	"fmt"
	"html"
	"os"
	"strings"
	"unicode/utf8"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var out strings.Builder
	for scanner.Scan() {
		out.WriteString(convertLine(scanner.Text()))
		out.WriteString("\n")
	}
	fmt.Print(out.String())
}

type style struct {
	bold       bool
	colorClass string // "" | "red" | "yellow" | "dim"
}

func (s style) spanOpen() string {
	if !s.bold && s.colorClass == "" {
		return ""
	}
	classes := []string{}
	if s.colorClass != "" {
		classes = append(classes, "c-"+s.colorClass)
	}
	if s.bold {
		classes = append(classes, "c-bold")
	}
	return `<span class="` + strings.Join(classes, " ") + `">`
}

// convertLine walks one line of raw captured bytes, translating SGR escape
// sequences into <span> boundaries and HTML-escaping everything else.
func convertLine(line string) string {
	var b strings.Builder
	cur := style{}
	open := false

	closeIfOpen := func() {
		if open {
			b.WriteString("</span>")
			open = false
		}
	}

	i := 0
	for i < len(line) {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			end := strings.IndexByte(line[i:], 'm')
			if end == -1 {
				break
			}
			code := line[i+2 : i+end]
			switch code {
			case "0":
				closeIfOpen()
				cur = style{}
			case "1":
				cur.bold = true
			case "2":
				cur.colorClass = "dim"
			case "31":
				cur.colorClass = "red"
			case "33":
				cur.colorClass = "yellow"
			}
			i += end + 1
			continue
		}
		// Lazily (re)open a span the first time we hit real content under
		// the current style, so consecutive escape codes (e.g. bold then
		// red) collapse into one span instead of one per code.
		if !open && (cur.bold || cur.colorClass != "") {
			b.WriteString(cur.spanOpen())
			open = true
		}
		// Decode a full rune (not a raw byte) so multi-byte UTF-8
		// characters - the severity icons are emoji - are not corrupted.
		r, size := utf8.DecodeRuneInString(line[i:])
		b.WriteString(html.EscapeString(string(r)))
		i += size
	}
	closeIfOpen()
	return b.String()
}

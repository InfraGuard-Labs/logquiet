// Command gen-screenshot wraps real, captured LogQuiet terminal output
// (already converted to HTML spans by demo/ansi2html) in a clean terminal-
// window mockup for a README screenshot. It does not alter the captured
// content in any way - only the surrounding chrome (title bar, font,
// colors) is templated here.
//
// Usage:
//
//	go run ./demo/ansi2html < captured.ansi | \
//	  go run ./demo/gen-screenshot -title "..." -command "..." -out page.html
package main

import (
	"flag"
	"fmt"
	"html"
	"io"
	"os"
)

const tmpl = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html, body {
    background: #0d1117;
    font-family: ui-monospace, "Cascadia Code", Consolas, "SF Mono", Menlo, monospace;
  }
  .window {
    display: inline-block;
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 8px 30px rgba(0,0,0,0.45);
    border: 1px solid #30363d;
    background: #0d1117;
  }
  .titlebar {
    background: #161b22;
    padding: 10px 14px;
    display: flex;
    align-items: center;
    gap: 8px;
    border-bottom: 1px solid #30363d;
  }
  .dot { width: 12px; height: 12px; border-radius: 50%%; display: inline-block; }
  .dot.red { background: #ff5f56; }
  .dot.yellow { background: #ffbd2e; }
  .dot.green { background: #27c93f; }
  .titlebar .label {
    flex: 1;
    text-align: center;
    color: #8b949e;
    font-size: 12.5px;
    font-family: -apple-system, "Segoe UI", sans-serif;
    margin-right: 54px;
  }
  .body {
    padding: 18px 22px 22px 22px;
    font-size: 15px;
    line-height: 1.55;
    color: #c9d1d9;
    white-space: pre;
  }
  .prompt { color: #58a6ff; }
  .prompt .cmd { color: #e6edf3; }
  .c-red { color: #ff7b72; }
  .c-yellow { color: #d29922; }
  .c-dim { color: #8b949e; }
  .c-bold { font-weight: 700; }
</style>
</head>
<body>
<div class="window">
  <div class="titlebar">
    <span class="dot red"></span><span class="dot yellow"></span><span class="dot green"></span>
    <span class="label">%s</span>
  </div>
  <div class="body"><span class="prompt">$ <span class="cmd">%s</span></span>

%s</div>
</div>
</body>
</html>
`

func main() {
	title := flag.String("title", "terminal", "titlebar label")
	command := flag.String("command", "", "the shell command line to show at the prompt")
	out := flag.String("out", "", "output HTML file path")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "gen-screenshot: -out is required")
		os.Exit(2)
	}

	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-screenshot:", err)
		os.Exit(1)
	}

	page := fmt.Sprintf(tmpl, html.EscapeString(*title), html.EscapeString(*command), string(body))
	if err := os.WriteFile(*out, []byte(page), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-screenshot:", err)
		os.Exit(1)
	}
}

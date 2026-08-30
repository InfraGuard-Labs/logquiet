# Security Policy

## Reporting a vulnerability

If you find a security issue in LogQuiet, please open a private security
advisory via GitHub ("Security" tab -> "Report a vulnerability") rather
than a public issue. If that is not available, open an issue asking for a
private contact channel without describing the vulnerability publicly.

We will acknowledge reports within a reasonable time and aim to ship a fix
before any public disclosure of details.

## Security model and assumptions

LogQuiet is a local command-line filter. Understanding what it does and
does not do is the actual security-relevant content of this document:

- **LogQuiet makes no network connections, ever, during normal operation.**
  It reads from stdin or a local file and writes to stdout/stderr and,
  optionally, a local file (`--impact-report`). There is no client for any
  remote service anywhere in the source tree.
- **LogQuiet has zero third-party dependencies.** `go.mod` declares no
  `require` beyond the module itself; every line of code that runs is in
  this repository. There is no transitive supply-chain surface to audit.
- **LogQuiet does not persist anything by default.** It does not write log
  content to disk, does not maintain a database, and does not create any
  file unless you explicitly pass `--impact-report <path>`, in which case
  it writes only the aggregate statistics documented in
  [docs/IMPACT_REPORT.md](docs/IMPACT_REPORT.md) - never raw log content.
- **LogQuiet trusts its input as data, not code.** Log lines are treated
  as plain text throughout the pipeline; there is no code execution,
  templating, or shell interpretation of log content. Terminal rendering
  writes raw bytes plus a small, fixed set of ANSI SGR color codes chosen
  by LogQuiet itself - it does not pass through or interpret arbitrary
  ANSI/control sequences that might appear *inside* a malicious log line
  (a stream containing terminal escape sequences designed to manipulate
  the viewer's terminal is a known class of issue for any tool that prints
  untrusted text to a terminal; see "Terminal escape sequence handling"
  below for the current state and its limits).
- **Process boundary is the trust boundary.** LogQuiet runs with the
  permissions of the user who invokes it and reads only what is piped to
  it or named on its command line. It does not read any file, environment
  variable, or configuration beyond its command-line flags and its input
  stream.

## Terminal escape sequence handling

LogQuiet does not currently sanitize or strip ANSI/control sequences that
already exist inside the raw log content it is displaying (it passes
displayed line content through largely as-is, prefixed with its own
color codes in non-plain modes). If you are running LogQuiet against logs
from a source you do not trust to *not* contain malicious terminal escape
sequences, prefer `--plain` or `--no-color` mode, or pipe LogQuiet's output
through a sanitizer before it reaches your terminal. Hardening this
further (stripping non-printable/control bytes from displayed content by
default) is a reasonable future improvement and is tracked as a known gap
rather than silently assumed away.

## Dependency and build-supply-chain posture

- Zero runtime dependencies (see above).
- Build tooling is the stock Go toolchain only; release binaries are built
  with `go build` (no third-party build plugins).
- Release artifacts are published with SHA-256 checksums (see
  [docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md)) so downloads can be
  verified.
- CI runs `go vet`, the full test suite (including fuzz tests for the
  parsing-heavy packages), and `govulncheck` against the standard library
  and any dependency that is ever added in the future, on every change.

## Scope

This policy covers the LogQuiet source code and official release
artifacts published from this repository. It does not cover third-party
packaging (unofficial distro packages, forks) unless the vulnerability
also exists in the upstream source.

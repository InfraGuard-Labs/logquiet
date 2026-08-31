# Changelog

All notable changes to this project are documented in this file. Format
loosely follows [Keep a Changelog](https://keepachangelog.com/), and this
project follows [Semantic Versioning](https://semver.org/) (see
[docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md) for what that means
before 1.0.0).

## [Unreleased]

Packaging/distribution and adoption-evidence tooling. No behavior, flag,
or schema change - see [docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md)
"Recommended next version" for why this is planned as v0.1.1, not v0.2.0.

### Added

- GoReleaser + [nfpm](https://nfpm.goreleaser.com)-based release
  automation (`.goreleaser.yaml`, `packaging/nfpm/nfpm.yaml`): generates
  `.deb`/`.rpm` packages for linux/amd64 and linux/arm64 and a Homebrew
  formula, in addition to the existing five raw binaries and
  `SHA256SUMS.txt`. Publishing to a tap/bucket remains a separate, manual
  step (`skip_upload: true`; see docs/RELEASE_PROCESS.md).
- Hardened `scripts/install.sh`: `LOGQUIET_BASE_URL` override, upfront
  `curl`/checksum-tool availability checks, actionable errors (including
  a sudo hint) on a non-writable install directory, POSIX `sh`
  compatibility (no bashisms), and a 12-scenario behavioral test suite
  (`scripts/tests/test-install.sh`).
- `docs/ADOPTION_EVIDENCE.md`, `docs/METRICS_DEFINITIONS.md`,
  `docs/ADOPTER_FEEDBACK_FORM.md`: a privacy-preserving framework for
  measuring and honestly characterizing external adoption - no
  telemetry, nothing collected automatically, every metric sourced from
  a public API or a user's own voluntary submission.
- `.github/ISSUE_TEMPLATE/real-world-feedback.yml`: a structured,
  entirely-optional issue template for real-world feedback, with an
  explicit warning against pasting confidential logs/credentials.
- `evidence/adoption/monthly-metrics.csv` (header only, no fabricated
  data) and `scripts/snapshot-public-metrics.sh`, which pulls public
  GitHub stars/forks/open-issue/release-download numbers with no
  required credentials.
- `docs/RELEASE_BUILD_RECORD.md`: records the exact source commit, Go
  version, build flags, and SHA256 hashes behind a real local build of
  the release artifacts.

### Fixed

- CLI `--help` text and the README both previously claimed important
  events "always break through" / "never" silently absorbed - corrected
  to accurately describe prioritization without an unproven absolute
  guarantee.

## [0.1.0] - 2026-08-30

Initial public release.

### Added

- Streaming stdin and file input with bounded-length line handling
  (`internal/reader`) safe against unusually long or malformed lines.
- Severity detection tolerant of bracketed and bare level tags and an
  arbitrary prefix (`internal/severity`).
- Structural normalization of timestamps, dates, UUIDs, MAC/IPv4/IPv6
  addresses, memory addresses, durations, byte sizes, percentages, hex
  hashes, generic alphanumeric IDs, and bare numbers (`internal/normalize`).
- Multiline grouping for Python tracebacks, Java/JVM stack traces, and Go
  panics, plus a generic indentation rule (`internal/multiline`).
- Structural fingerprinting and bounded (LRU-evicted), per-pattern state
  tracking (`internal/fingerprint`, `internal/pattern`).
- Deterministic, non-AI frequency-spike (anomaly) detection using a
  rolling-window rate compared against an exponentially weighted baseline,
  with severity-aware sensitivity.
- Adaptive repeat suppression: novel patterns shown in full immediately;
  repeats (including interleaved, non-consecutive repeats) accumulated and
  flushed on a bounded, severity-aware cadence.
- Four output modes: TTY (color), plain, no-color, and newline-delimited
  JSON.
- `--stats` summary and `--impact-report` aggregate, content-free JSON
  export.
- Configurable anomaly window, spike multiplier, severity protection
  threshold, and pattern-tracking bound.
- Clean shutdown on EOF, Ctrl+C, and SIGTERM; graceful handling of a
  closed downstream pipe.
- Full unit, integration, and fuzz test coverage for the parsing-heavy
  packages; a correctness benchmark suite verifying 100% retention of
  known-important events across ten realistic synthetic scenarios.
- Documentation: README, architecture, prior-art review, technical
  method, benchmarks, impact-report schema, release process, security,
  and privacy documents.

### Known limitations

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) "Known limitations" and
[docs/TECHNICAL_METHOD.md](docs/TECHNICAL_METHOD.md) section 9 for a
plain accounting of what is heuristic versus deterministic, and where each
heuristic's edge cases are.

[Unreleased]: https://github.com/InfraGuard-Labs/logquiet/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/InfraGuard-Labs/logquiet/releases/tag/v0.1.0

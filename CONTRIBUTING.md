# Contributing to LogQuiet

Thanks for considering a contribution. LogQuiet is young; process is kept
light on purpose.

## License and why Apache 2.0

LogQuiet is licensed under the [Apache License 2.0](LICENSE), chosen over
MIT or a copyleft license for two reasons: its explicit patent grant (and
patent-litigation retaliation clause) gives adopters - including
organizations running this against production infrastructure logs - more
confidence than a bare permissive license offers, and it remains fully
permissive for commercial and internal use, which matters for a tool meant
to be installed widely with no friction. By submitting a contribution, you
agree it is licensed under the same terms (see "Submission of
Contributions" in the LICENSE file).

## Before you start

For anything beyond a small fix (typo, obvious bug), please open an issue
first describing what you want to change and why. This avoids someone
spending time on a PR that conflicts with the project's direction.

## Development setup

LogQuiet has zero runtime dependencies and only needs the Go toolchain:

```bash
go build ./...
go test ./...
go vet ./...
gofmt -l .        # should print nothing
```

Regenerate fixtures/benchmark data (only needed if you touch
`tools/fixturegen` or want fresh large corpora):

```bash
go run ./tools/fixturegen correctness
go run ./tools/fixturegen perf 1000000 mixed benchmarks/data/mixed-1m.log
```

Run the correctness benchmark suite (verifies known-important events are
never silently suppressed):

```bash
go run ./benchmarks/correctness
```

## What to test

- Add a unit test for any new normalization pattern, severity alias, or
  multiline shape, in the relevant `internal/*` package.
- If you change suppression/rendering behavior, add or update a test in
  `internal/render` or `internal/pipeline` - the pipeline tests are the
  ones that catch "this collapses correctly for interleaved, not just
  back-to-back, repeats" regressions, which is the single easiest thing to
  break in this codebase.
- If you touch `internal/normalize` or `internal/multiline`, run the fuzz
  tests for at least a few seconds locally before opening a PR:
  `go test ./internal/normalize/ -fuzz=FuzzTemplate -fuzztime=30s`.
- If you change anything performance-sensitive, run the relevant benchmark
  before and after (`go test ./internal/normalize/ -bench=. -run=^$`) and
  mention the numbers in your PR description - this project has already
  hit one real regex-performance regression that only benchmarking caught.

## Code style

- Standard `gofmt`; no linter config beyond that is required.
- No new third-party dependencies without a specific, discussed reason -
  the zero-dependency property is a stated design goal (see
  [SECURITY.md](SECURITY.md)), not an accident.
- Comments explain *why*, not *what* - see the existing code for the
  house style. Don't add a comment restating what a well-named function
  already says.
- Prefer adding a test that encodes the correct behavior over a comment
  asserting it.

## Reporting bugs

Please include: the LogQuiet version (`logquiet --version`), your OS/arch,
a minimal input that reproduces the issue, and what you expected vs. what
happened. A synthetic log snippet reproducing the bug is far more useful
than a description - see `fixtures/synthetic/` for the style used
throughout this project (never paste real production log content into an
issue).

## Security issues

Do not open a public issue for a security vulnerability - see
[SECURITY.md](SECURITY.md) for the reporting process.

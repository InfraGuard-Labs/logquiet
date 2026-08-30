# Evidence

This directory tracks objective, verifiable evidence of LogQuiet's
development and (as it accumulates over time) real-world impact. It exists
because this project may later be presented as evidence of an original
technical contribution, and evidence collected only after the fact, from
memory, is much weaker than a contemporaneous record.

**The governing rule for everything in this directory: only record what
actually happened, with a link or artifact proving it.** Do not record
aspirational, hoped-for, or inferred facts. An empty subsection below is
more honest, and more useful later, than a fabricated one - a claim that
turns out to be invented undermines every true claim alongside it.

## Categories

Each category below is a place to *add* evidence as it genuinely occurs,
not a checklist to fill in immediately.

### 1. Prior art and originality
- [docs/PRIOR_ART.md](../docs/PRIOR_ART.md) - the competitive review
  conducted before finalizing the design, with sources and a git-tracked
  timestamp (commit history) showing it predates, not follows, the
  implementation.

### 2. Original technical design
- [docs/TECHNICAL_METHOD.md](../docs/TECHNICAL_METHOD.md) - the algorithm,
  with reasoning for each design decision, including decisions that were
  *reversed* during development (e.g. the cursor-redraw-vs-periodic-
  summarization change) and why - a design document that only shows the
  final answer is weaker evidence of original thinking than one that shows
  the reasoning trail.
- [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) - the language-choice
  reasoning and system design.

### 3. Development history
- The git commit history of this repository itself, which records real
  engineering work as it happened: initial implementation, a specific
  correctness bug found and fixed via the benchmark suite (interleaved-
  pattern suppression), a specific performance bottleneck found via
  profiling and fixed (structural normalization), etc. Commit messages in
  this repository are written to be a truthful record, not marketing.

### 4. Releases
- (To be added as tags are cut.) Each release tag and its
  `SHA256SUMS.txt` is itself evidence of a specific, verifiable, dated
  artifact. See [docs/RELEASE_PROCESS.md](../docs/RELEASE_PROCESS.md).

### 5. Benchmark results
- [docs/BENCHMARKS.md](../docs/BENCHMARKS.md) and
  `benchmarks/results/*.json` - real, reproducible measurements, with the
  exact commands to reproduce them, run on a specific, disclosed machine.
  Nothing in that document is an estimate.

### 6. Package/download analytics
- Not yet applicable - no package has been published to a registry or
  package manager yet. When one is (see
  [docs/RELEASE_PROCESS.md](../docs/RELEASE_PROCESS.md)), genuine download
  counts from that registry's own public API/dashboard belong here, with
  the date they were pulled and a link to the source.

### 7. Repository analytics
- Not yet applicable. When meaningful (stars, forks, clone traffic from
  GitHub's own insights), a dated screenshot or API pull belongs here -
  never a hand-typed number.

### 8. Independent users
- None recorded yet. This section should only ever contain a real person
  or organization who can be named (with their permission) or a verifiable
  public reference (a blog post, a public issue/PR from an unaffiliated
  account, a public forum mention) - never an invented or assumed user.

### 9. External organizations
- None recorded yet. Same standard as above: a real, named, verifiable
  relationship, or nothing.

### 10. Documented use cases
- The ten synthetic scenarios in `fixtures/synthetic/` document *intended*
  use cases (clearly marked synthetic, per their own README) - they are
  evidence of design intent, not of external adoption, and should never be
  cited as if they were the latter.

### 11. Integrations
- None recorded yet.

### 12. External mentions
- None recorded yet. A mention must be a real, linkable, dated source.

### 13. Independent benchmarks
- None recorded yet - everything in `docs/BENCHMARKS.md` today was run by
  this project's own author on their own machine, and is labeled as such.
  An independent benchmark, if one is ever run by someone else, is
  categorically stronger evidence and belongs here with a link to their
  own published methodology and results.

### 14. Publications / presentations
- None recorded yet.

### 15. Expert feedback
- None recorded yet. Feedback from a named, identifiable person with
  relevant expertise (an SRE, a maintainer of a comparable tool) belongs
  here with their permission to be quoted/named - never summarized from
  memory without that permission, and never invented.

### 16. Impact reports (voluntarily shared by users)
- None collected yet. See [docs/IMPACT_REPORT.md](../docs/IMPACT_REPORT.md)
  for the schema a user could voluntarily generate and choose to share -
  aggregate statistics only, never raw log content, never collected
  without the user's explicit action (`--impact-report` is opt-in, and
  sharing the resulting file is entirely the user's choice).

## Maintenance

Update this file as evidence genuinely accumulates - do not leave it
stale, and do not backfill it with anything that cannot be substantiated
with a link, a file, or a verifiable public record.

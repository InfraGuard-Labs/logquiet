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

See [docs/ADOPTION_EVIDENCE.md](../docs/ADOPTION_EVIDENCE.md) for what
each category of evidence can and cannot prove, and
[docs/METRICS_DEFINITIONS.md](../docs/METRICS_DEFINITIONS.md) for exact
definitions (including what "verified" requires) before adding a number
to [evidence/adoption/monthly-metrics.csv](adoption/monthly-metrics.csv).

## Preservation workflow

Every piece of evidence added to this directory (or referenced from it)
should record these six things, so it stands on its own later without
relying on anyone's memory of the context:

1. **Date** - when the evidence was observed/captured, not when it was
   added to this repository if the two differ.
2. **Source URL** - where it came from, if it has one (a GitHub page, an
   API endpoint, an external article).
3. **Screenshot / PDF / original file** - the artifact itself, not just a
   description of it. A number without the underlying screenshot/export
   is not preserved evidence, it's a claim.
4. **What it proves** (and, ideally, what it does NOT prove) - see
   [docs/ADOPTION_EVIDENCE.md](../docs/ADOPTION_EVIDENCE.md) for the
   per-category version of this distinction. Copy the relevant caveat
   alongside the evidence rather than making a reader look it up.
5. **Public or private** - whether the source itself is publicly visible
   (a public GitHub issue) or was shared privately (an email, a private
   Slack message) and therefore needs the next field before it can ever
   be shown to anyone else.
6. **Consent status**, if an identifiable person or organization is
   involved - what they agreed to be recorded/named for, per the consent
   fields in [docs/ADOPTER_FEEDBACK_FORM.md](../docs/ADOPTER_FEEDBACK_FORM.md)
   and [.github/ISSUE_TEMPLATE/real-world-feedback.yml](../.github/ISSUE_TEMPLATE/real-world-feedback.yml).
   Default to treating anything without explicit "yes, you may name us
   publicly" consent as private, even if it arrived through a public
   channel like a GitHub issue - a person filing an issue under their own
   handle has not automatically agreed to being cited in a case study.

Suggested per-category preservation targets, once something genuinely
exists to preserve:

- **GitHub releases**: a dated screenshot or saved HTML/PDF of the
  release page, plus `SHA256SUMS.txt` for that release (already a
  self-verifying artifact).
- **Monthly release-download totals**: the JSON snapshot written by
  [scripts/snapshot-public-metrics.sh](../scripts/snapshot-public-metrics.sh),
  timestamped, kept alongside the corresponding row in
  [evidence/adoption/monthly-metrics.csv](adoption/monthly-metrics.csv).
- **Stars/forks snapshots**: same script/file as above.
- **GitHub traffic** (unique visitors/clones): GitHub's traffic insights
  page (`Insights > Traffic`) requires authenticated owner/maintainer
  access and only retains 14 days of history - it is not reachable via
  the public API used by `snapshot-public-metrics.sh`. Preserve this
  manually and monthly: log in as a maintainer, screenshot the Traffic
  page (both the visitor and clone graphs), and save the screenshot here
  with the date. There is no way to automate this without owner
  credentials, which this project's scripts deliberately do not require
  or store.
- **Issue/PR evidence**: the issue/PR's own URL (permanent and public by
  default) is usually sufficient; add a saved copy only if the content
  might be edited/deleted later.
- **Impact reports**: the raw JSON file, reviewed against
  [docs/IMPACT_REPORT.md](../docs/IMPACT_REPORT.md)'s schema before being
  stored, with a note on how it was received (issue attachment, email,
  etc.) and from whom (only if they consented to being identified).
- **User confirmations / organization adoption / case studies**: the
  original message or document (with the sender's permission to keep it),
  or a summary plus the consent record, never a summary alone if consent
  to quote/name was given.
- **External mentions / integrations**: the URL, archived if the source
  is the kind of page that tends to disappear (a saved PDF or
  web-archive link alongside the live URL).

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

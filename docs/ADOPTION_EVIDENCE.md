# Adoption and Impact Evidence

This document explains what LogQuiet's project maintainers can measure
about real-world adoption, what each measurement actually proves (and
does not prove), and how those measurements are collected - without
compromising the no-telemetry promise in [PRIVACY.md](PRIVACY.md).

**LogQuiet has no telemetry, no phone-home behavior, and no analytics
library, and this document does not change that.** Every metric below is
either (a) pulled from a public API/dashboard that GitHub or a package
manager already operates and already exposes about any public repository
- nothing LogQuiet's own code reports - or (b) voluntarily submitted by a
user, through an explicit, visible action they take (attaching a file to
an issue, filling out a form), never collected automatically. See
[docs/METRICS_DEFINITIONS.md](METRICS_DEFINITIONS.md) for the precise
definition of every field named below, and
[evidence/README.md](../evidence/README.md) for how each piece of
evidence is preserved once collected.

## Why this document exists

A project can be genuinely useful and still have no record of that
usefulness a year later, because nobody wrote down what was measurable,
when, or how. This document exists so that measurement is planned in
advance, consistently defined, and honestly bounded - not backfilled from
memory or inflated under pressure to show a bigger number.

## The categories

### 1. GitHub Release asset downloads

**Source:** `GET /repos/InfraGuard-Labs/logquiet/releases` (public, no
auth required), field `download_count` per asset. **What it proves:** a
release asset was fetched N times. **What it does NOT prove:** that N
people use LogQuiet, that any download resulted in a successful install,
or that any two downloads came from different people or machines. See
[METRICS_DEFINITIONS.md](METRICS_DEFINITIONS.md) "downloads != unique
users" - this is the single most commonly misused metric in software
adoption claims, and this project will not misuse it.

### 2. GitHub repository stars

**Source:** `GET /repos/InfraGuard-Labs/logquiet`, field
`stargazers_count`. **What it proves:** N GitHub accounts clicked
"star." **What it does NOT prove:** that any of them installed, ran, or
even read past the README - a star is a bookmark, not a usage signal.

### 3. Forks

**Source:** same API call, field `forks_count`. **What it proves:** N
GitHub accounts created a fork. **What it does NOT prove:** active use -
most forks of most projects are never synced or built.

### 4. External contributors

**Source:** the repository's contributors list/API, filtered to exclude
the project's own maintainer(s). **What it proves:** at least one person
outside the project cared enough to submit a change that was merged -
meaningfully stronger evidence of engagement than a star, since it
required reading the code. **What it does NOT prove:** production usage
- a contributor can fix a typo without ever running the tool against real
logs.

### 5. Issues/PRs from outside users

**Source:** the issues/PRs API, filtered by author, excluding the
project's own maintainer(s). **What it proves:** external engagement -
someone read the project closely enough to file something. **What it
does NOT prove:** that the issue reflects successful, ongoing use; a bug
report can come from someone who tried LogQuiet once and hit a wall.

### 6. Package-manager downloads/install counts (where publicly available)

**Source:** the package manager's own public analytics, if and when
LogQuiet is actually published to one (not yet true as of this writing -
see [RELEASE_PROCESS.md](RELEASE_PROCESS.md) "Package manager metadata").
**What it would prove/not prove:** the same "downloads != unique users"
caveat as category 1, plus whatever caveats that specific package
manager documents about its own counting methodology.

### 7. Voluntarily submitted LogQuiet impact reports

**Source:** `--impact-report` JSON files a user chooses to send the
project (see [IMPACT_REPORT.md](IMPACT_REPORT.md) "Voluntarily sharing
one"). **What it proves:** LogQuiet ran, end-to-end, against a real input
stream of the reported size, with the reported suppression/retention
behavior - a genuine, technical, first-party data point about a real run.
**What it does NOT prove:** who ran it, at what organization, in
production or in a five-second test, or more than once - an impact
report by itself carries zero identifying information (see the schema
audit in [IMPACT_REPORT.md](IMPACT_REPORT.md)), which is the point, but
it also means a report alone cannot establish "organization X uses this
in production" without the submitter also saying so.

### 8. Identifiable external organizations using LogQuiet

**Source:** direct confirmation per the "verified" standard in
[METRICS_DEFINITIONS.md](METRICS_DEFINITIONS.md). **What it proves:**
real adoption by a named or nameable entity - the strongest category here
when it exists. **What it does NOT prove anything about:** scale (one
verified organization is one data point, not a trend) unless multiple,
independent organizations are recorded.

### 9. Public integrations into scripts/CI/Kubernetes workflows

**Source:** a public, linkable repository/config file that invokes
`logquiet`, found independently or reported and then confirmed by
visiting the link. **What it proves:** someone built LogQuiet into a real
pipeline, publicly, which is a stronger signal than a star or a download
because it required actual integration work. **What it does NOT prove:**
that the pipeline runs regularly or in production, absent further
confirmation (a commit history showing ongoing maintenance of that
integration is corroborating, not conclusive).

### 10. Independent articles/posts/mentions

**Source:** a dated, linkable, third-party publication. **What it
proves:** external awareness. **What it does NOT prove:** usage - a
mention can be purely informational.

### 11. Independent benchmark reproductions

**Source:** a performance/correctness measurement published by someone
other than this project's maintainer(s), with disclosed methodology.
**What it proves:** the strongest form of technical validation available
- it removes this project's own potential bias in fixture selection and
measurement. **What it does NOT prove:** adoption or scale by itself,
only that the technical claims independently replicate (or don't - a
negative independent result belongs here too, per
[evidence/README.md](../evidence/README.md)'s "only record what actually
happened" rule).

### 12. Public presentations/talks/references

**Source:** a recording, deck, or transcript, not authored/delivered by
this project's maintainer(s). **What it proves:** third-party validation
strong enough for someone to present it to an audience under their own
name. **What it does NOT prove:** usage scale beyond the presenter's own
stated experience.

## What this document will never recommend

- Presenting a download count as a user count.
- Presenting a star or fork count as evidence of production use.
- Counting an unverified, unconfirmed, or anonymous claim in
  `evidence/adoption/monthly-metrics.csv` (see
  [METRICS_DEFINITIONS.md](METRICS_DEFINITIONS.md) "verified").
- Adding any code path that collects, transmits, or infers any of the
  above automatically. Every category above is either a read of a public
  API about the *public repository itself* (not about any user's
  machine, network, or logs) or something a user hands over voluntarily,
  in the open, by their own action.

## How this is collected in practice

- Public GitHub/API-sourced numbers (categories 1-3, partially 4-6): see
  [scripts/snapshot-public-metrics.sh](../scripts/snapshot-public-metrics.sh)
  for a script that pulls them on demand, and
  [evidence/adoption/monthly-metrics.csv](../evidence/adoption/monthly-metrics.csv)
  for where a dated snapshot is recorded.
- Voluntary submissions (categories 7-12): see
  [ADOPTER_FEEDBACK_FORM.md](ADOPTER_FEEDBACK_FORM.md) and
  [.github/ISSUE_TEMPLATE/real-world-feedback.yml](../.github/ISSUE_TEMPLATE/real-world-feedback.yml)
  for how a user can choose to submit one.
- Everything collected is preserved per the workflow in
  [evidence/README.md](../evidence/README.md), including its date,
  source, and whether it is public or requires consent to name.

# Metrics Definitions

This document defines, precisely, every metric tracked in
[ADOPTION_EVIDENCE.md](ADOPTION_EVIDENCE.md) and recorded in
`evidence/adoption/monthly-metrics.csv`. The goal is that a number
recorded in month 3 and a number recorded in month 30 were counted the
same way - a metric that silently changes definition over time is worse
than no metric at all, because it looks comparable when it isn't.

**Governing rule:** every metric below is either (a) a number GitHub's or
a package registry's own API/UI reports, cited with its source and pull
date, or (b) a count of individually verifiable, linkable items. Nothing
here is a survey estimate, a projection, or a number inferred without a
primary source.

## GitHub repository metrics

- **`github_stars`** - the star count on `InfraGuard-Labs/logquiet` as
  reported by the GitHub API (`GET /repos/{owner}/{repo}`,
  `stargazers_count`) or the repository page, at the moment of the
  snapshot. A star is a single click by a GitHub account; it does not
  imply the account holder installed, ran, or even read the project.
- **`github_forks`** - the fork count from the same API response
  (`forks_count`). A fork does not imply active use; many forks are
  never synced or run.
- **`github_external_contributors`** - the count of distinct GitHub
  accounts, other than the project's own maintainer(s), with at least one
  commit merged into `main`, as shown by the repository's contributors
  API/page. "External" excludes any account controlled by the project's
  maintainer(s).
- **`external_issues`** / **`external_prs`** - issues or pull requests
  opened by a GitHub account other than the project's own maintainer(s),
  counted from the issues/PRs API filtered by author. This counts
  *engagement*, not resolved-and-confirmed adoption - an issue can be a
  question, a false report, or spam, and should be read accordingly, not
  automatically credited as evidence of a working deployment.

## Package-manager metrics

- **`release_downloads`** (GitHub Releases) - the sum of
  `download_count` across all assets attached to a given release, from
  `GET /repos/{owner}/{repo}/releases`. **A download event, not a unique
  user or a unique machine.** The same person re-running the install
  script, a CI pipeline that re-downloads on every run, and a security
  scanner that fetches every release asset all increment this number
  identically to a first-time human install. See "downloads != unique
  users" below - this distinction must never be collapsed in reporting.
- **Homebrew/Scoop install counts** - if and when a formula/manifest is
  actually published to a real tap/bucket (see
  [RELEASE_PROCESS.md](RELEASE_PROCESS.md) - not yet done as of this
  writing), only counts pulled from that package manager's own public
  analytics (e.g. Homebrew's public analytics, when opted into and
  published by Homebrew itself) belong here, dated and sourced. Nothing
  is estimated or extrapolated from GitHub download counts.

## "Downloads != unique users"

**A release-asset download count must never be reported, cited, or
implied as a user count, an install count, or an adoption count.** One
person can trigger many downloads (re-running an install script,
retrying a failed download, testing multiple platforms); automated
systems (CI, mirrors, vulnerability scanners, dependency bots) can
trigger downloads with no human behind them at all; one download can
also represent zero real installs (a build that failed after the
download, a person who downloaded and never ran it). The only honest
framing is: "N release-asset download events were recorded by GitHub
between dates X and Y" - never "N people use LogQuiet."

## Voluntary / user-submitted evidence

- **`voluntary_impact_reports`** - the count of `--impact-report` JSON
  files that have actually been received (attached to an issue, emailed,
  or otherwise handed over) from someone other than the project's own
  maintainer(s), each one individually reviewed before being counted (see
  [IMPACT_REPORT.md](IMPACT_REPORT.md) "Voluntarily sharing one" for the
  reviewer's own privacy checklist before forwarding or publishing one).
- **`verified_raw_lines_processed`** - the sum of the `raw_lines` field
  across all voluntarily submitted impact reports counted above. This is
  a real, machine-reported number *from that submitter's own run*, but it
  is only as trustworthy as the submission is genuine - see "verified"
  below for what makes a submission countable here at all.

## "Verified" - the standard for every category below

A claim only qualifies as **verified** for the purposes of this project's
evidence records if at least one of the following is true, and the
supporting artifact is preserved per
[evidence/README.md](../evidence/README.md):

1. **An identifiable person** (a real name or a persistent, attributable
   public account - not "someone on Reddit said") states the fact,
   ideally in a context where saying something false would have a
   reputational cost (a public GitHub issue/PR, a signed email, a LinkedIn
   post under their real identity).
2. **A public, linkable integration** - a public repository, CI config,
   Dockerfile, or Kubernetes manifest that visibly invokes `logquiet`,
   found independently (not self-reported) or reported and then
   independently confirmed by visiting the link.
3. **Written confirmation** from the organization or person in question,
   obtained with their knowledge that it may be recorded as adoption
   evidence (see the consent fields in
   [ADOPTER_FEEDBACK_FORM.md](ADOPTER_FEEDBACK_FORM.md) and
   [real-world-feedback.yml](../.github/ISSUE_TEMPLATE/real-world-feedback.yml)).
4. **Other documentary evidence** with a preservable, checkable source
   (a dated screenshot with a URL, an archived page, a public talk
   recording).

Applying this standard:

- **"verified organization"** = an external organization whose use of
  LogQuiet is supported by at least one of the four evidence types above
  - not an assumption from, e.g., a company-sounding username on an
  issue, and not a organization merely *mentioned* as hypothetical.
- **"verified external user"** = an identifiable person (per type 1
  above) who has confirmed, in their own words, that they ran LogQuiet -
  not a download, not a star, not an assumed reader.
- **release downloads** = asset download events reported by GitHub, per
  the definition above - explicitly **not** unique users, and never to be
  presented as such.

An unverified signal (an anonymous mention, an unconfirmed claim, a
plausible-sounding but unconfirmed username) is not "weak evidence of
adoption" - it is not evidence for these records at all. It can be noted
in `evidence/README.md` prose as a lead to follow up on, but must not be
counted in `evidence/adoption/monthly-metrics.csv`.

## Independent evidence

- **`independent_integrations`** - a public integration (script, CI
  pipeline, Kubernetes manifest, Dockerfile) that uses LogQuiet, authored
  by someone other than this project's maintainer(s), found and linked.
- **`independent_mentions`** - an article, blog post, forum post, or
  social media post about LogQuiet, written by someone unaffiliated with
  the project, with a stable link.
- **`independent_benchmarks`** - a performance or correctness measurement
  of LogQuiet run and published by someone other than this project's
  maintainer(s), with their own disclosed methodology - categorically
  stronger evidence than this project's own numbers in
  [BENCHMARKS.md](BENCHMARKS.md), because it removes the possibility of
  the author's own bias in methodology or fixture selection.
- **Public presentations/talks/references** - a recording, slide deck, or
  published transcript of a talk or presentation that references
  LogQuiet, not authored or delivered by this project's maintainer(s).

## What is deliberately never a metric here

No metric in this document or in `monthly-metrics.csv` is: a projection,
an extrapolation from a smaller sample, a number "estimated" from
indirect signals, or a count that includes the project's own
maintainer(s) activity presented as external engagement. If a number
cannot be sourced and dated, it does not go in the ledger - an honest
blank is more useful later than a plausible-looking guess.

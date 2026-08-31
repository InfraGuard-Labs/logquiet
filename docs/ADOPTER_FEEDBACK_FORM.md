# Adopter / Feedback Form (specification)

LogQuiet does not run any external form provider today, so there is
nothing to embed or link to yet. This document specifies the form's
content so that:

- feedback can already be given right now, the low-tech way (the fields
  below map directly onto
  [.github/ISSUE_TEMPLATE/real-world-feedback.yml](../.github/ISSUE_TEMPLATE/real-world-feedback.yml)),
  and
- if/when a real form is stood up (e.g. a static form service, or a
  simple self-hosted one), it has an exact, already-reviewed field list
  to implement, instead of being designed ad hoc.

**Every identifying field is optional.** Nothing here requires a name, a
company, an email address, or any log content to participate.

## Principles

- No field that could identify a person or organization is required.
- No field asks for confidential logs, credentials, tokens, or secrets.
  If a respondent wants to illustrate a problem, the request is always
  for a *description* of what happened, or a share of the
  content-free `--impact-report` output - never a log excerpt.
- Consent to be named or contacted is captured explicitly and separately
  from the rest of the feedback, and defaults to "No."
- Submitting the form (however it is eventually hosted) does not enroll
  anyone in anything else - no mailing list signup is implied.

## Fields

| Field | Required? | Notes |
|---|---|---|
| Role (e.g. SRE, backend engineer, platform team, student, hobbyist) | Optional | Free text or a short list; helps interpret feedback, not identify anyone. |
| Organization / company | Optional | Explicitly optional - see consent field below before this is ever named publicly. |
| Country / region | Optional | Coarse-grained only (country/region, not city or address). |
| Environment (multi-select) | Optional | Kubernetes / Docker / journalctl-systemd / CI-CD / application logs / other (free text) |
| Runtime / language (multi-select) | Optional | Java / Go / Python / Node / other (free text) |
| Approximate log volume | Optional | A rough order of magnitude (e.g. "~10K lines/day", "~5M lines/day") - never a request for an actual log file. |
| Did LogQuiet help? | Optional | Free text or short scale. |
| Anything incorrectly suppressed? | Optional | Free text description of the *symptom* (e.g. "a WARN I cared about was collapsed into a repeat counter") - not a log excerpt. See the warning below. |
| Any repetitive noise it failed to collapse? | Optional | Same - describe the shape, not the content. |
| Would you keep using LogQuiet? | Optional | Yes / No / Not sure |
| Impact report upload/attachment | Optional | A `--impact-report` JSON file (see [IMPACT_REPORT.md](IMPACT_REPORT.md)) - content-free by construction, reviewed by the respondent before sending either way. |
| May we mention your organization publicly? | Optional, defaults to "No" | Yes / No |
| May we contact you about a case study? | Optional, defaults to "No" | Yes / No - if "Yes," a contact method is needed *only then*, and only for that purpose. |

## The warning every version of this form must carry

> **Do not paste confidential logs, credentials, tokens, or PII.**
> Describe what happened instead (e.g. "a CRITICAL line got folded into a
> repeat counter") rather than pasting the line itself. A voluntarily
> generated `--impact-report` file (see
> [IMPACT_REPORT.md](IMPACT_REPORT.md)) is safe to attach - it is
> aggregate counts and percentages only, never raw log content.

## How a submission becomes evidence

A filled-out form (or issue, using the GitHub template) is not
automatically added to
[evidence/adoption/monthly-metrics.csv](../evidence/adoption/monthly-metrics.csv).
It is reviewed against the "verified" standard in
[METRICS_DEFINITIONS.md](METRICS_DEFINITIONS.md) first, and only counted
or named publicly according to the consent the respondent actually gave.
See [evidence/README.md](../evidence/README.md) for the full preservation
workflow.

## Today, without a hosted form

Until a hosted form exists, the equivalent way to give this same feedback
is to open an issue using
[.github/ISSUE_TEMPLATE/real-world-feedback.yml](../.github/ISSUE_TEMPLATE/real-world-feedback.yml)
in this repository - it mirrors every field above.

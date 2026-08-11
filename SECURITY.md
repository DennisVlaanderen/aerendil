# Security Policy

Aerendil is pre-v1 and not yet deployed in production anywhere, but we still
want to know about security issues as soon as possible.

## Reporting a Vulnerability

Please report suspected vulnerabilities privately using
[GitHub's private vulnerability reporting](https://github.com/DennisVlaanderen/toggly/security/advisories/new)
for this repository, rather than opening a public issue. If that's not
enabled yet, open a regular issue asking for a private channel and we'll
follow up.

Include as much detail as you can: affected component (`backend/`, `ui/`,
`fqdp/`, etc.), reproduction steps, and potential impact. We'll acknowledge
reports as promptly as we can, given this is a small pre-v1 project.

## Scope

Automated scanning already runs on every change via CodeQL, `govulncheck`,
and Trivy (container images) — see [.github/workflows](./.github/workflows).
Reports about findings already surfaced there are still welcome, especially
if you believe one is exploitable rather than theoretical.

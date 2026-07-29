# TLSFerry CE release evidence

This file records evidence for the first CE release candidate without weakening the requirements in `release-checklist.md`. A checkbox is complete only when its evidence can be reproduced from the named commit or real test environment.

## Automated evidence

| Requirement | Status | Evidence |
| --- | --- | --- |
| Clean CE/Cloud boundary | Pass | `internal/edition` tests and `docs/edition-boundary.md`; Cloud implementation remains in the separate private repository. |
| Local release gate | Pass | `make release-check` on commit `899d5ace4e237ed5d356dc2feed33a2d2288496e`; formatting, tests, vet, `govulncheck`, versioned build, example validation, GoReleaser validation, six archives, and checksums passed. |
| GitHub cross-platform CI | Pass | [CI run 30421097158](https://github.com/nonozone/TLSFerry/actions/runs/30421097158) on the same commit; verification plus Ubuntu, macOS, and Windows test/build jobs passed. |
| Public repository metadata | Pass | Repository is public and reports Apache-2.0; `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, changelog, deployment guide, example config, and public protocol documentation are tracked. |

## Required real-environment evidence

These items remain release blockers. Unit tests, rendered scheduler definitions, and cross-compilation do not replace them.

| Environment | Install | Status | Run now | Logs/diagnostics | Uninstall | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| macOS launchd user agent | Pending | Pending | Pending | Pending | Pending | Record OS version, commands, sanitized output, and date. |
| Linux systemd user timer | Pending | Pending | Pending | Pending | Pending | Include missed-run recovery and whether user linger was required. |
| Windows Task Scheduler | Pending | Pending | Pending | Pending | Pending | Include interactive-user identity and last-result inspection. |

| Functional smoke | Status | Evidence required |
| --- | --- | --- |
| Let's Encrypt staging DNS-01 issuance | Pending | Dedicated test hostname, staging directory, sanitized command result, certificate metadata, and confirmation that no secret or challenge value entered logs. |
| Non-production provider deployment | Pending | Least-privilege test credential, target resource, sanitized provider request id, and rollback/removal result. |

## Publication boundary

No release tag has been pushed and no GitHub Release exists. Creating `v0.1.0-rc.1` remains a deliberate maintainer action after all pending evidence above is complete. Do not reinterpret a green automated gate as permission to publish.

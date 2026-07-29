# TLSFerry CE release evidence

This file is the evidence matrix for a CE release candidate. It does not weaken the requirements in `release-checklist.md`. Complete the candidate fields during release review; a status is Pass only when its evidence can be reproduced from that exact commit or real test environment.

## Candidate under review

| Field | Value |
| --- | --- |
| Version | Pending (for example, `v0.1.0-rc.1`) |
| Commit | Pending (record the full candidate commit SHA) |
| Review date | Pending (UTC) |
| Reviewer | Pending |

## Automated evidence

| Requirement | Status | Evidence |
| --- | --- | --- |
| Clean CE/Cloud boundary | Pending | Run `internal/edition` tests on the candidate and review `docs/edition-boundary.md`; confirm Cloud implementation remains in the separate private repository. |
| Local release gate | Pending | Run `make release-check` from a clean checkout of the candidate commit. Record the command, commit SHA, date, and sanitized output or artifact link. |
| GitHub cross-platform CI | Pending | Record the CI run URL for the candidate commit. Verification plus Ubuntu, macOS, and Windows test/build jobs must pass. |
| Public repository metadata | Pending | Confirm the candidate repository is public and reports Apache-2.0; verify `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, changelog, deployment guide, example config, and public protocol documentation are tracked. |

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

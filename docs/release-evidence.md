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
| Release archive integrity | Pass | Commit `37c1a885b0b150e75a26ae7bc086d9bc7a6ecda2` passed the local release gate and native archive smoke on Ubuntu, macOS, and Windows; see the reproducible evidence below. |
| Public repository metadata | Pass | Verified against commit `41c0604c14be394c6db5aa86cfca95e87d14b211`: GitHub reports a public Apache-2.0 repository, all required public files are tracked, and the repository security/reporting settings below are enabled. |

Generate the local, credential-free part of this table with `make release-audit AUDIT_VERSION=<candidate> AUDIT_REVIEWER=<name>`. Preserve its JSON output with the candidate review. A passing audit records the exact commit and time, but deliberately lists CI, public repository metadata, real staging issuance, provider deployment, and tag authorization as manual gates.

### Public repository metadata and security evidence

- Date and source: `2026-07-29`, commit `41c0604c14be394c6db5aa86cfca95e87d14b211`, [public repository](https://github.com/nonozone/TLSFerry), and [CI run 30433444115](https://github.com/nonozone/TLSFerry/actions/runs/30433444115).
- GitHub metadata: the repository API reported `visibility: public`, default branch `main`, and SPDX license `Apache-2.0`. Topics are `acme`, `certificate-automation`, `certificate-management`, `dns-01`, `golang`, `letsencrypt`, `self-hosted`, and `tls`.
- Public source files: Git confirmed that `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, README, the deployment guide, example configuration, edition boundary, release checklist/evidence, and remote DNS protocol are tracked.
- Security reporting: private vulnerability reporting is enabled, so the **Security → Report a vulnerability** route documented in `SECURITY.md` is available. Dependabot vulnerability alerts and security updates, secret scanning, and secret-scanning push protection are enabled.
- Workflow permissions: GitHub's default workflow permission is read-only and Actions cannot approve pull requests. Release actions are pinned to full commit SHAs; the source audit enforces a read-only verification job, a dependent publication job with the only `contents: write` grant, non-persisted checkout credentials, and a single explicit `GITHUB_TOKEN` handoff.
- Maintainer policy: `main` currently has no GitHub branch-protection rule, preserving the current single-maintainer direct-push workflow. This is not release authorization: pushing a `v*` tag remains a separate deliberate action, and branch protection must be reconsidered before granting additional maintainers write access.

### Dependency vulnerability response evidence

- Discovery: enabling GitHub vulnerability alerts exposed [Dependabot alert 1](https://github.com/nonozone/TLSFerry/security/dependabot/1), CVE-2026-40611 / GHSA-qqx8-2xmm-jrv8, against the direct `github.com/go-acme/lego/v4 v4.33.0` dependency. The advisory rates the HTTP-01 webroot path traversal High (CVSS 8.8) and identifies `v4.34.0` as the first patched version.
- Fix: commit `a3ff15fb0777841d6059c32f87feb65ef6a55352` upgrades lego to `v4.34.0` and accepts the patched module's required indirect dependency versions. TLSFerry CE exposes DNS-01 rather than HTTP-01 issuance, but the vulnerable implementation is still removed from the release dependency graph instead of being dismissed as unreachable.
- Local verification: `make verify`, `go mod verify`, and `go list -m github.com/go-acme/lego/v4` passed. Verbose `govulncheck v1.6.0` reported zero symbol-level and package-level vulnerabilities. Its only remaining module notice is GO-2026-5932 for the unmaintained `golang.org/x/crypto/openpgp` package; `go mod why` confirms that the main module does not need that package and no fixed version exists.
- GitHub verification: [Dependency Graph run 30438798736](https://github.com/nonozone/TLSFerry/actions/runs/30438798736) passed, and GitHub changed alert 1 to `fixed` at `2026-07-29T09:15:44Z` without dismissal.
- Cross-platform CI: [run 30438806132](https://github.com/nonozone/TLSFerry/actions/runs/30438806132) passed verification plus [Ubuntu 90532860052](https://github.com/nonozone/TLSFerry/actions/runs/30438806132/job/90532860052), [macOS 90532860063](https://github.com/nonozone/TLSFerry/actions/runs/30438806132/job/90532860063), [Windows 90532860092](https://github.com/nonozone/TLSFerry/actions/runs/30438806132/job/90532860092), systemd, and Task Scheduler jobs.

## Required real-environment evidence

These items remain release blockers. Unit tests, rendered scheduler definitions, and cross-compilation do not replace them.

| Environment | Install | Status | Run now | Logs/diagnostics | Uninstall | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| macOS launchd user agent | Pass | Pass | Pass | Pass | Pass | See the macOS evidence below for commit, OS, isolated commands, sanitized results, and cleanup. |
| Linux systemd user timer | Pass | Pass | Pass | Pass | Pass | See the Linux evidence below, including a real `Persistent=true` missed-run recovery and linger cleanup. |
| Windows Task Scheduler | Pass | Pass | Pass | Pass | Pass | See the Windows evidence below, including interactive least-privilege identity and last-result inspection. |

| Functional smoke | Status | Evidence required |
| --- | --- | --- |
| Credential-free CLI flow | Pass | See the evidence below for the exact commit, local release gate, three native CI jobs, no-network isolation, read-only assertions, selected-domain-only enrollment, and credential-output checks. |
| Let's Encrypt staging DNS-01 issuance | Pending | Dedicated test hostname, staging directory, sanitized command result, certificate metadata, and confirmation that no secret or challenge value entered logs. |
| Non-production provider deployment | Pending | Least-privilege test credential, target resource, sanitized provider request id, and rollback/removal result. |

### Credential-free CLI functional evidence

- Date and source: `2026-07-29`, commit `e5ece56dbfa67b4269e60b34ad51a5f0635be283`, [CI run 30431046013](https://github.com/nonozone/TLSFerry/actions/runs/30431046013).
- Native jobs: [Ubuntu 90508122872](https://github.com/nonozone/TLSFerry/actions/runs/30431046013/job/90508122872), [macOS 90508122924](https://github.com/nonozone/TLSFerry/actions/runs/30431046013/job/90508122924), and [Windows 90508122856](https://github.com/nonozone/TLSFerry/actions/runs/30431046013/job/90508122856) each passed `make functional-smoke` and the platform build.
- Local gate: `make release-check` passed from the clean source commit. It included the functional smoke, all Go tests, vet, a fixed-version vulnerability scan with zero reachable vulnerabilities, example validation, GoReleaser configuration validation, six snapshot archives, and checksums.
- Isolation: the smoke used reserved `.invalid` domains, the Let's Encrypt staging directory, synthetic environment credential values, and an in-memory authorized Tencent CDN inventory. It made no ACME, DNS, credential-store, or cloud API request.
- Read-only behavior: byte-for-byte configuration comparisons proved that `validate`, `plan`, `preflight`, JSON discovery, and enrollment preview did not modify the configuration.
- Enrollment behavior: `--execute` preserved the existing certificate, appended exactly one certificate for the selected account-owned CDN domain, excluded the second unselected inventory domain, and produced a configuration that passed a second `preflight`.
- Secret safety: the test checked all command output against every synthetic credential value. Failure diagnostics redact those values before writing test output.

### Release archive integrity evidence

- Date and source: `2026-07-29`, commit `37c1a885b0b150e75a26ae7bc086d9bc7a6ecda2`, [CI run 30431879003](https://github.com/nonozone/TLSFerry/actions/runs/30431879003).
- Native jobs: [Ubuntu 90510730449](https://github.com/nonozone/TLSFerry/actions/runs/30431879003/job/90510730449), [macOS 90510730527](https://github.com/nonozone/TLSFerry/actions/runs/30431879003/job/90510730527), and [Windows 90510730577](https://github.com/nonozone/TLSFerry/actions/runs/30431879003/job/90510730577) each built all release targets and executed the archive verifier on its native platform.
- Archive set: the verifier required exactly six expected archives for Linux, macOS, and Windows on amd64 and arm64, plus a checksum entry for every archive. Missing, duplicate, unexpected, path-traversing, or otherwise unsafe archive entries fail the check.
- Source binding: verification required a clean tracked worktree, bound GoReleaser metadata to the exact Git `HEAD`, and compared the archived `LICENSE`, `README.md`, and `config.example.json` byte-for-byte with the source commit.
- Runtime and modes: Unix archives were required to preserve an executable binary mode. Each native job extracted its own archive and successfully ran `tlsferry version`, proving that the packaged binary—not only the source tree—was executable.
- Local gate: `make release-check` passed from the clean source commit and included `make artifact-smoke`, all Go tests, vet, a fixed-version vulnerability scan with zero reachable vulnerabilities, example validation, GoReleaser configuration validation, six snapshot archives, and checksum verification.

### macOS launchd user-agent evidence

- Date: `2026-07-29T06:29:44Z`.
- Source commit and binary version: `03c133e6e151d5c151e68203badddc89b267ebad`.
- Host: macOS 27.0, build `26A5388g`, Apple silicon.
- Isolation: the test supplied a temporary `HOME`, state directory, output directory, log directory, plist path, and binary path under `/tmp/tlsferry-launchd-evidence.*`. It used a reserved `.invalid` hostname, the Let's Encrypt staging directory, and an intentionally absent environment credential. No production credential or resource was used.
- Install: `tlsferry service install --config <test-home>/config.json --state-dir <test-home>/state --output-dir <test-home>/certificates --hour 3 --minute 17 --accept-tos --execute` succeeded; `plutil -lint` reported `OK` for the generated plist.
- Status: `tlsferry service status` reported `installed: true` and `running: true`; `launchctl print` reported `state = running`, `runs = 1`, and PID `15269`.
- Run now: `tlsferry service run-now` reported `renewal service started`. A second `launchctl print` retained PID `15269` and `runs = 1`, proving the active renewal was not killed or overlapped; the error log contained no renewal-lock conflict.
- Diagnostics: `tlsferry service logs` returned the isolated standard-output and standard-error paths under `<test-home>/.tlsferry/logs/`.
- Uninstall: `tlsferry service uninstall` succeeded. A final status reported `installed: false` and `running: false`; the plist was absent and `launchctl print gui/<uid>/com.nonozone.tlsferry` no longer found the service. The temporary test directory was removed.

### Linux systemd user-timer evidence

- Date and source: `2026-07-29`, commit `cb074c37f686760a0163e074b76cdbaeb1322dd4`, [CI run 30429574464](https://github.com/nonozone/TLSFerry/actions/runs/30429574464), [systemd job 90503439305](https://github.com/nonozone/TLSFerry/actions/runs/30429574464/job/90503439305).
- Host: Ubuntu 24.04.4 LTS, kernel `6.17.0-1020-azure`, systemd `255.4-1ubuntu8.16` on the GitHub-hosted ephemeral runner.
- Isolation: the job generated an ephemeral self-signed certificate valid for one year and loaded it through the production certificate store. The reserved `.invalid` hostname was outside the 24-hour renewal window, so both executions reported `renewal skipped`; no ACME, DNS, cloud credential, or provider call was made.
- Install and status: `service install` created and enabled `tlsferry-renew.timer`; `service status` reported `installed: true` and `running: true`. The rendered timer contained `Persistent=true`.
- Missed-run recovery and linger: the job enabled user linger and confirmed `Linger=yes`, activated the timer to establish its persistent timestamp, stopped it before the `06:53` schedule, crossed the missed boundary, and restarted it. systemd reported `LastTriggerUSec=2026-07-29 06:53:02 UTC`, `Result=success`, and `ExecMainStatus=0`.
- Run now and diagnostics: `service run-now` completed with `Result=success` and `ExecMainStatus=0`; `service logs` returned `journalctl --user --unit tlsferry-renew.service`, whose sanitized output showed the expected skip result twice.
- Uninstall: `service uninstall` removed both units and disabled the timer. Final status reported `installed: false` and `running: false`; temporary test data was removed and linger was returned to its original setting.

### Windows Task Scheduler evidence

- Date and source: `2026-07-29`, commit `cb074c37f686760a0163e074b76cdbaeb1322dd4`, [CI run 30429574464](https://github.com/nonozone/TLSFerry/actions/runs/30429574464), [Task Scheduler job 90503439351](https://github.com/nonozone/TLSFerry/actions/runs/30429574464/job/90503439351).
- Host: Microsoft Windows NT `10.0.26100.0` with PowerShell `7.6.3` on the GitHub-hosted ephemeral runner.
- Isolation: the job used the same generated non-due certificate and reserved `.invalid` hostname as the Linux smoke. No ACME, DNS, cloud credential, or provider call was made, and all task XML, state, certificate, and binary paths were temporary.
- Install and status: `service install` successfully registered `TLSFerry Renewal`; status reported `installed: true` and `running: true`. Task Scheduler reported `State=Ready`, `LogonType=Interactive`, and `RunLevel=Limited`.
- Run now and diagnostics: `service run-now` completed at `2026-07-29 06:53:27`; `Get-ScheduledTaskInfo` and the documented `schtasks.exe /Query /TN "TLSFerry Renewal" /V /FO LIST` command both reported last result `0`.
- Uninstall: `service uninstall` removed the task and temporary XML. Final status reported `installed: false` and `running: false`; a native task lookup confirmed the task no longer existed, and temporary test data was removed.

## Publication boundary

No release tag has been pushed and no GitHub Release exists. Creating `v0.1.0-rc.1` remains a deliberate maintainer action after all pending evidence above is complete. Do not reinterpret a green automated gate as permission to publish.

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

Generate the local, credential-free part of this table with `make release-audit AUDIT_VERSION=<candidate> AUDIT_REVIEWER=<name>`. Preserve its JSON output with the candidate review. A passing audit records the exact commit and time, but deliberately lists CI, public repository metadata, real staging issuance, provider deployment, and tag authorization as manual gates.

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

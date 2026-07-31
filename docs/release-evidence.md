# TLSFerry CE release evidence

This file is the human-readable evidence matrix for completed CE candidate reviews. It does not weaken the requirements in `release-checklist.md`. A status is Pass only when its evidence can be reproduced from the recorded commit or real test environment.

Updating this tracked file creates a new Git commit, so it cannot truthfully embed its own current `HEAD`. The commit below is therefore the exact source candidate that was audited, not necessarily the commit containing this evidence update. For a tag candidate, the authoritative current evidence is the preserved `make release-audit` JSON plus GitHub CI for that same commit; the tag must point to that audited commit.

## Latest completed candidate review

| Field | Value |
| --- | --- |
| Version | Unversioned CE candidate (no `v*` tag authorized or created) |
| Commit | `5b6dfe82fcf03832e84d096bbf7ec71a2add50bd` |
| Review date | `2026-07-31T07:34:45Z` |
| Reviewer | Local release audit (`Codex`) plus GitHub Actions |

## Automated evidence

| Requirement | Status | Evidence |
| --- | --- | --- |
| Clean CE/Cloud boundary | Pass | The candidate passed `make release-audit` and all `internal/edition` tests. The audit found no Cloud product implementation in CE; the only crossing point remains the public, versioned remote DNS protocol. |
| Local release gate | Pass | `env GOCACHE=/private/tmp/tlsferry-go-cache make release-check AUDIT_VERSION=v0.1.0-rc.1 AUDIT_REVIEWER=Codex` passed from the clean candidate commit at `2026-07-31T07:34:45Z`; see the candidate evidence below. |
| GitHub cross-platform CI | Pass | [CI run 30611685069](https://github.com/nonozone/TLSFerry/actions/runs/30611685069) passed for the exact candidate commit, including verification, Ubuntu, macOS, Windows, systemd, and Task Scheduler jobs. |
| Release archive integrity | Pass | The candidate passed local six-platform snapshot archive verification; the exact commit also passed native archive smoke on Ubuntu, macOS, and Windows in CI. See the candidate and reproducible archive evidence below. |
| Public repository metadata | Pass | Verified for the candidate review: GitHub reports a public, active Apache-2.0 repository with `main` as its default branch. Required source files and security settings are covered by the evidence below. |

Generate the local, credential-free part of this table with `make release-audit AUDIT_VERSION=<candidate> AUDIT_REVIEWER=<name>`. Preserve its JSON output with the candidate review. A passing audit records the exact commit and time, but deliberately lists CI, public repository metadata, real staging issuance, provider deployment, and tag authorization as manual gates.

### Current candidate automated evidence

- Date and source: `2026-07-31T07:34:45Z`, commit `5b6dfe82fcf03832e84d096bbf7ec71a2add50bd`, and [CI run 30611685069](https://github.com/nonozone/TLSFerry/actions/runs/30611685069).
- Local gate: `env GOCACHE=/private/tmp/tlsferry-go-cache make release-check AUDIT_VERSION=v0.1.0-rc.1 AUDIT_REVIEWER=Codex` passed from a clean tracked worktree. It ran the source and edition audit, all Go tests, credential-free functional smoke, `go vet`, `govulncheck v1.6.0`, both example configuration validations, GoReleaser configuration validation, six-platform snapshot builds, checksum verification, archive-content checks, and the native packaged-binary version smoke.
- Vulnerability result: `govulncheck` reported zero reachable vulnerabilities and zero vulnerabilities in imported packages. It reported one module-only advisory that is not called by TLSFerry, consistent with the dependency evidence below.
- Archive result: Linux, macOS, and Windows archives for amd64 and arm64 were present with checksums. Every archive contained `LICENSE`, `README.md`, `config.example.json`, `config.release-smoke.example.json`, and the platform binary; the native archived binary reported `TLSFerry 0.0.0-SNAPSHOT-5b6dfe8`.
- Exact-commit CI: [verify 91095652968](https://github.com/nonozone/TLSFerry/actions/runs/30611685069/job/91095652968), [Ubuntu 91095652945](https://github.com/nonozone/TLSFerry/actions/runs/30611685069/job/91095652945), [macOS 91095652981](https://github.com/nonozone/TLSFerry/actions/runs/30611685069/job/91095652981), [Windows 91095652961](https://github.com/nonozone/TLSFerry/actions/runs/30611685069/job/91095652961), [systemd 91095652937](https://github.com/nonozone/TLSFerry/actions/runs/30611685069/job/91095652937), and [Task Scheduler 91095652865](https://github.com/nonozone/TLSFerry/actions/runs/30611685069/job/91095652865) all completed successfully.
- Publication state: verified on `2026-07-31`; the local repository has no tags and GitHub has no Releases. This records evidence only and does not authorize a tag or publication.

### Public repository metadata and security evidence

- Date and source: metadata refreshed on `2026-07-30` while reviewing commit `5a99ac6dac80e586665811acc161d255082f8421`; see the [public repository](https://github.com/nonozone/TLSFerry) and [CI run 30510200766](https://github.com/nonozone/TLSFerry/actions/runs/30510200766).
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

These items are mandatory release gates. Unit tests, rendered scheduler definitions, and cross-compilation do not replace them.

| Environment | Install | Status | Run now | Logs/diagnostics | Uninstall | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| macOS launchd user agent | Pass | Pass | Pass | Pass | Pass | See the macOS evidence below for commit, OS, isolated commands, sanitized results, and cleanup. |
| Linux systemd user timer | Pass | Pass | Pass | Pass | Pass | See the Linux evidence below, including a real `Persistent=true` missed-run recovery and linger cleanup. |
| Windows Task Scheduler | Pass | Pass | Pass | Pass | Pass | See the Windows evidence below, including interactive least-privilege identity and last-result inspection. |

| Functional smoke | Status | Evidence required |
| --- | --- | --- |
| Credential-free CLI flow | Pass | See the evidence below for the exact commit, local release gate, three native CI jobs, no-network isolation, read-only assertions, selected-domain-only enrollment, and credential-output checks. |
| Let's Encrypt staging DNS-01 issuance | Pass | `go.nonopen.com` completed delegated DNS-01 issuance through `auth-staging.tlsferry.com`; see the sanitized staging evidence below. |
| Non-production provider deployment | Pass | Tencent CDN served a fresh staging certificate whose D1 and public-edge fingerprints matched, then Cloud protocol v2 restored the available previous certificate. D1 and a second public TLS probe verified the completed rollback; see the bounded follow-up cycle below. |

### Real staging ACME and Tencent CDN evidence

- Date and environment: `2026-07-30`, TLSFerry Cloud staging control plane at `staging.tlsferry.com`, delegated validation zone `auth-staging.tlsferry.com`, and Tencent CDN hostname `go.nonopen.com`.
- Delegation: public DNS resolved `_acme-challenge.go.nonopen.com` to the stored per-hostname target under `auth-staging.tlsferry.com`; no Cloudflare API credential, ACME TXT value, certificate PEM, private key, or Tencent permanent credential was returned by the control plane or written to this evidence.
- Issuance: Let's Encrypt staging issued `CN=go.nonopen.com`, valid from `2026-07-30T08:40:18Z` through `2026-10-28T08:40:17Z`, with SHA-256 fingerprint `C3:76:DA:E8:19:27:2F:D4:2C:81:D6:8D:BF:10:D9:0A:4D:28:95:F4:45:2D:CB:CA:8C:AF:45:3C:6F:C6:DC:F5`.
- Provider deployment: Tencent SSL certificate ID `Zbw23o2r` and CDN deploy record `239414` completed successfully. A public TLS probe returned the same subject, validity, staging issuer, and SHA-256 fingerprint from the CDN edge.
- Previous binding: the executor captured Tencent certificate ID `S84T6Taf` before replacement. It was an already expired TrustAsia C1 DV Free certificate for the same hostname.
- Initial rollback result: Cloud protocol v2 queued rollback job `f7b05c31-2746-44be-aeed-43dd96e5a3ed` using only the server-saved previous certificate ID and issued no DNS job token. Tencent rejected the expired `S84T6Taf` certificate with bounded error `FailedOperation.CertificateNotAvailable`; D1 recorded the job as failed, left `rolled_back_at` unset, and retained `Zbw23o2r` as current. A fresh public TLS probe confirmed the staging certificate remained served.
- Follow-up issuance: after `Zbw23o2r` was known to be an available rollback target, job `42283f08-ba71-433f-980d-635d1e780be9` issued and deployed staging certificate `ZcWx0tDg`, recording `Zbw23o2r` as `previous_provider_certificate_id`. The certificate was valid from `2026-07-30T17:37:51Z` through `2026-10-28T17:37:50Z`; D1 stored SHA-256 `32365b563468505d65cfcf1ff91d6b097b83e9c40fab61e4eecee6b76bcabca6`, and a public TLS probe returned the identical fingerprint `32:36:5B:56:34:68:50:5D:65:CF:CF:1F:F9:1D:6B:09:7B:83:E9:C4:0F:AB:61:E4:EE:CE:E6:B7:6B:CA:BC:A6` from the Tencent CDN edge.
- Successful rollback: Cloud protocol v2 queued job `feb234ef-741f-463a-a091-b4c8fd79ae8e` for the server-saved target `Zbw23o2r`. Tencent completed the restore at `2026-07-30T18:38:29Z` with no error; D1 recorded the rollback job, target, and `rolled_back_at`. A post-rollback public TLS probe returned the original `Zbw23o2r` validity and SHA-256 fingerprint `C3:76:DA:E8:19:27:2F:D4:2C:81:D6:8D:BF:10:D9:0A:4D:28:95:F4:45:2D:CB:CA:8C:AF:45:3C:6F:C6:DC:F5`.
- Cleanup boundary: this successful second cycle completes the non-production provider deployment-and-rollback gate. `go.nonopen.com` deliberately remains on a Let’s Encrypt staging certificate and is not publicly trusted; the evidence must not be represented as production readiness. A production ACME issuance, production Cloud provisioning, and any `v*` tag remain separate authorization boundaries.

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
- Source binding: the current verifier requires a clean tracked worktree, binds GoReleaser metadata to the exact Git `HEAD`, and compares four archived public files—`LICENSE`, `README.md`, `config.example.json`, and `config.release-smoke.example.json`—byte-for-byte with the source commit. This recorded `37c1a885` run predates the smoke template and compared the first three files.
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

As verified during this candidate review, no release tag has been pushed and no GitHub Release exists. Choosing and creating a version such as `v0.1.0-rc.1` remains a deliberate maintainer action after all pending real-environment evidence above is complete. Do not reinterpret a green automated gate as permission to publish.

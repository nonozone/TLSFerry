# TLSFerry CE release checklist

This checklist is the release gate for CE. A green unit test alone is not sufficient evidence for a release.

Record reproducible results in [release-evidence.md](release-evidence.md). Pending rows in that file are release blockers, not optional follow-up work.

## 1. Source and edition boundary

- The tracked worktree is clean and points at the intended release commit.
- `internal/edition` confirms no Cloud account, tenant, billing, control-plane, database migration, or console units entered the public repository.
- `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, README, deployment guide, example config, and public protocol documentation are current.
- No configuration, credential, private key, certificate, state directory, or generated release artifact is tracked.

## 2. Automated release gate

Run from a clean tracked worktree:

```bash
make release-check
```

This first emits a JSON source audit, then runs formatting checks, all Go tests, `go vet`, fixed-version `govulncheck`, a versioned build, example configuration validation, GoReleaser schema validation, six cross-platform snapshot builds, archive creation, and checksums. It then verifies all checksums and archive contents and executes the current platform's binary from its archive. It must leave tracked files unchanged.

For a real candidate, record the exact semantic version and reviewer rather than the local defaults:

```bash
make release-audit AUDIT_VERSION=v0.1.0-rc.1 AUDIT_REVIEWER=maintainer-name
```

The audit records the full `HEAD` commit and UTC review time. It fails on tracked changes, non-ignored untracked sensitive files, tracked credentials/certificates/generated output, missing public release files, CE/Cloud boundary violations, or a non-Apache-2.0 license. Its `manual_gates` remain mandatory; the command cannot replace GitHub CI, repository visibility review, real staging issuance, a non-production provider deployment, or tag authorization.

GitHub CI must also pass the full verification job and native test/build jobs on Ubuntu, macOS, and Windows for the same commit.

The artifact gate can be repeated after an existing snapshot build:

```bash
make artifact-smoke
```

It requires a clean tracked worktree, exactly six archives, and matching checksum entries; rejects path traversal, duplicate or unexpected content; compares bundled `LICENSE`, README, and example configuration bytes with the candidate source; requires executable Unix binaries; rejects artifacts built from another commit; and confirms the native archived binary reports the GoReleaser version. The explicit clean-tree check is required because GoReleaser snapshot metadata alone may not report a modified worktree. The tag release workflow runs the same gate before publication.

## 3. Platform scheduler evidence

Before the first stable release, record one clean install, status, run-now, diagnostic/log, and uninstall cycle on each platform:

- macOS launchd user agent.
- Linux systemd user timer, including a missed-run recovery check.
- Windows Task Scheduler under an interactive user, including last-result inspection.

Platform limitations must remain explicit: Linux headless operation may need user linger; the Windows installer does not store a password and therefore does not silently create a logged-out server identity.

## 4. Safe functional smoke test

Run the credential-free functional gate locally:

```bash
make functional-smoke
```

The gate uses reserved `.invalid` domains, the Let's Encrypt staging directory, synthetic environment credentials, and an in-memory cloud inventory. It must make no network request. CI runs it on Ubuntu, macOS, and Windows.

- `validate`, `plan`, and `preflight` pass with a synthetic or staging configuration.
- Discovery is read-only.
- Enrollment preview leaves the configuration unchanged.
- Enrollment with `--execute` writes only the selected account-owned domain.
- The post-enrollment configuration passes `preflight`, and neither successful output nor failure diagnostics expose credential values.
- A Let's Encrypt staging issuance and one non-production provider deployment are tested with least-privilege credentials.
- Logs and errors contain no credentials, certificate private keys, bearer tokens, or ACME challenge values.

## 5. Publish and rollback

- Choose a semantic version such as `v0.1.0-rc.1`; the tag must point to the audited commit.
- Review the generated changelog before pushing the tag.
- The Release workflow must pass `artifact-smoke` before publishing six archives and `checksums.txt`.
- Download one archive, verify its checksum, and confirm `tlsferry version` reports the tag.
- Keep the previous release available. Binary rollback does not undo certificates already uploaded to cloud providers.

Pushing a `v*` tag is the external publication boundary and requires deliberate maintainer authorization.

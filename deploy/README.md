# TLSFerry CE deployment and operations

## Deployment scope

This repository ships one deployment unit: the open-source TLSFerry CE command-line binary. It runs on a machine controlled by the operator and calls ACME, DNS, and cloud APIs. It is intentionally excluded from Docker Compose because certificate files, operating-system credential stores, and the host scheduler are its natural boundaries.

TLSFerry Cloud is not deployed from this repository. The `tlsferry-cloud` DNS provider is only the public executor-side protocol adapter; hosted accounts, billing, authoritative DNS, orchestration, databases, and the web console remain private Cloud units.

| Unit | Hosting | Runtime | State | Release | Rollback |
| --- | --- | --- | --- | --- | --- |
| TLSFerry CE | self-hosted, host-managed | Go static binary | local files plus external cloud state | GitHub Release archives | replace the binary with a previous tagged release |

## Ownership and release split

- The operator owns the machine, configuration, credentials, ACME account, certificate private keys, scheduler, and cloud permissions.
- GitHub Actions verifies every change. A `v*` tag publishes macOS, Linux, and Windows archives plus `checksums.txt`.
- `make deploy`, Compose deployment, and container rollback do not apply to this host-level CLI.
- Cloud provider changes made by `tlsferry deploy` or `tlsferry renew` are external state and are not reversed by rolling back the binary.

## Prerequisites

- A supported release archive, or Go 1.25+ for a source build.
- A writable configuration directory and a private state directory.
- Least-privilege credentials in Keychain, Credential Manager, Secret Service, or an isolated service environment.
- DNS and cloud API connectivity during renewal.

## Install from source

```bash
make verify
make install
"$HOME/.local/bin/tlsferry" version
```

Override the install prefix when needed:

```bash
make install PREFIX=/usr/local
```

For a tagged release, download the archive matching the operating system and architecture from GitHub Releases, verify it against `checksums.txt`, and place `tlsferry` in a stable directory on `PATH`. Do not install a scheduler from a temporary `go run` binary.

## Local run mode

The development mode is hybrid/native Go with no shared infrastructure:

```bash
go run ./cmd/tlsferry validate --config config.example.json
go run ./cmd/tlsferry plan --config config.example.json
```

There is no long-running development server and no `make dev` target. A scheduled renewal process starts, performs one bounded run, and exits.

## Standard directory layout

Recommended per-user layout:

```text
~/.local/bin/tlsferry
~/.config/tlsferry/config.json
~/.local/state/tlsferry/
  accounts/
  certificates/
  logs/
```

The CLI currently defaults to paths relative to the working directory, so production scheduler installation must pass absolute configuration, state, output, and log paths.

## Configuration and secrets

`config.example.json` is the canonical schema example. Copy it to a private location; `config.json` is ignored by Git.

- `keychain:PROFILE` is preferred for desktop schedulers.
- `env:PROFILE` is intended for servers and CI. Environment files are not created or managed by this repository.
- Never put literal secrets, certificate private keys, or short-lived Cloud job tokens in the JSON configuration or service definition.
- Restrict the state directory to the service user. TLSFerry writes account and private keys with mode `0600` and private directories with mode `0700`.

## Deploy checks

Before installing or tagging a release:

```bash
make verify
git diff --check
```

`make verify` checks formatting, tests, `go vet`, a versioned local build, and the example configuration. Maintainers with GoReleaser installed can also run:

```bash
make release-snapshot
make artifact-smoke
```

The artifact smoke verifies all six checksums and archive manifests, compares bundled public files with the source commit, and executes the current platform's archived binary to confirm its embedded version before installation.

## Install and startup contract

1. Install the binary at a permanent path.
2. Copy and edit the example configuration outside the repository.
3. Store each credential with `tlsferry auth login PROVIDER`, or inject an isolated server environment.
4. Run `tlsferry validate`, `tlsferry plan`, and `tlsferry preflight`.
5. Perform one explicitly authorized manual issuance or renewal.
6. Install the host scheduler.

Installation is complete only when `tlsferry version` reports the intended tag, preflight passes, a manual run succeeds, and scheduler status/logs are readable.

## Schedulers, logs, and health

macOS has a native user-level launchd adapter:

```bash
tlsferry service install --config /absolute/path/config.json --accept-tos --execute
tlsferry service status
tlsferry service run-now
tlsferry service logs
```

Linux uses a native user-level systemd timer:

```bash
tlsferry service install --config /absolute/path/config.json --accept-tos --execute
systemctl --user status tlsferry-renew.timer
journalctl --user --unit tlsferry-renew.service
```

The timer uses `Persistent=true`, `UMask=0077`, and a one-shot service. The user manager must be available; enable linger for a dedicated headless service account when the schedule must run while that account is logged out.

Windows uses a least-privilege task for the current interactive user:

```powershell
tlsferry.exe service install --config C:\TLSFerry\config.json --accept-tos --execute
tlsferry.exe service status
tlsferry.exe service run-now
tlsferry.exe service logs
```

The generated Task Scheduler XML contains only absolute paths and renewal flags, never cloud credentials or a Windows password. It starts missed runs when possible, waits for network availability, and ignores overlapping instances. Because the task uses the current interactive token, a Windows server that must run while nobody is signed in needs a manually assigned dedicated task identity and a protected service credential environment.

Across platforms the process is periodic, so health means the latest run exited successfully and the next schedule is installed.

## Upgrade and rollback

1. Keep the current binary as `tlsferry.previous`.
2. Verify the new archive checksum and install the new binary atomically at the same path.
3. Run `tlsferry version`, `validate`, `preflight`, and one `service run-now` check.
4. If the new binary fails, restore `tlsferry.previous` and run the same checks.

Configuration and state formats must remain backward compatible within a release line. Back up the state directory before a major-version upgrade. Binary rollback does not revert certificates already uploaded to a provider; provider-side rollback or redeployment of the previous certificate is a separate operator action.

## Persistence and provider caveats

- Back up the ACME account directory and certificate metadata securely; never publish private keys.
- A lost state directory can cause unnecessary reissuance and rate-limit pressure.
- Use Let's Encrypt staging for rollout tests.
- Restrict DNS credentials to required zones and cloud credentials to required certificate/CDN resources.
- Tencent CDN/COS deployment is asynchronous; a submitted job is not proof that every edge already serves the certificate.
- The remote DNS protocol token must be short-lived and scoped to one enrolled hostname and job.

## Troubleshooting

- `TLSFerry dev` means the binary was built locally without release metadata; rebuild with `make build` or install a tagged archive.
- Scheduler cannot read credentials: use the operating-system keychain or explicitly inject the service environment; interactive shell exports are not inherited reliably.
- Renewal overlaps: TLSFerry's state lock blocks concurrent runs; inspect the previous process and logs before removing stale operating-system artifacts.
- Certificate was uploaded but HTTPS is unchanged: verify the provider deployment status and the exact CDN hostname binding.

## Project-specific notes

The public CE repository owns certificate automation and provider integrations. It must not gain Cloud account tables, payment code, hosted DNS credentials, tenant orchestration, or console UI. Shared behavior crosses the edition boundary only through documented, versioned protocols such as `docs/remote-dns-protocol.md`.

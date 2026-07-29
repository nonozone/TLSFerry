# TLSFerry Community Edition

TLSFerry Community Edition (CE) is the open-source, self-hosted edition of TLSFerry. It is a Go-based TLS certificate automation tool for issuing certificates through ACME and delivering them to multiple cloud platforms.

The project can issue real certificates through ACME DNS-01 and deploy them to Tencent Cloud CDN/COS, Alibaba Cloud CDN, and Qiniu CDN.

## Editions

- **TLSFerry CE** is this Apache-2.0 repository. It runs on infrastructure controlled by the user, keeps cloud credentials local, and provides the CLI, DNS providers, renewal scheduling, discovery, and multi-cloud certificate delivery.
- **TLSFerry Cloud** is the planned managed service. Its hosted DNS validation, account system, billing, orchestration, and operations control plane are separate from this repository.

The shared certificate and provider behavior remains in CE. The commercial service sells hosted operation and reduced maintenance rather than a different certificate format or a deliberately broken community build. The enforced source and protocol boundary is documented in [docs/edition-boundary.md](docs/edition-boundary.md).

## Installation

Tagged releases publish standalone archives for macOS, Linux, and Windows, together with `checksums.txt`. Until the first tag is published, build and install from source:

```bash
make verify
make install
"$HOME/.local/bin/tlsferry" version
```

See [the deployment and operations guide](deploy/README.md) for release checks, directory layout, scheduler setup, upgrades, and rollback boundaries.

## Why Go

- One static binary is easy to run on servers, NAS devices, containers, and CI systems.
- Strong concurrency primitives fit renewal scheduling and multi-provider delivery.
- Tencent Cloud, Alibaba Cloud, Qiniu, and ACME all have usable Go SDKs or HTTP APIs.
- A small standard-library CLI keeps the bootstrap dependency-free.

## Current commands

```bash
go run ./cmd/tlsferry version
go run ./cmd/tlsferry validate --config config.example.json
go run ./cmd/tlsferry plan --config config.example.json
go run ./cmd/tlsferry preflight --config config.example.json
go run ./cmd/tlsferry dns-check --config config.example.json --certificate assets-example --domain assets.example.com
go run ./cmd/tlsferry issue --config config.example.json --certificate assets-example --accept-tos
go run ./cmd/tlsferry deploy --config config.example.json --certificate assets-example --provider tencent-cdn --execute
go run ./cmd/tlsferry renew --config config.example.json --accept-tos --execute
go run ./cmd/tlsferry release-smoke --config config.release-smoke.example.json --certificate staging-cdn --provider tencent-cdn
go run ./cmd/tlsferry auth login cloudflare
go run ./cmd/tlsferry auth login tencent
go run ./cmd/tlsferry completion zsh
```

## Command help and shell completion

Every top-level command supports contextual help, and spelling mistakes suggest the closest command:

```bash
tlsferry help discover
tlsferry discover --help
tlsferry discovr
# Did you mean "discover"?
```

Install Tab completion for the current shell in one step:

```bash
tlsferry completion install
```

The installer detects `$SHELL`. Override it when needed:

```bash
tlsferry completion install --shell zsh
tlsferry completion install --shell bash
tlsferry completion install --shell fish
```

Zsh installs `_tlsferry` under `~/.zfunc` and adds an idempotent managed activation block to `~/.zshrc`. Bash installs into the user bash-completion directory and adds a managed source block to `~/.bashrc`. Fish uses `~/.config/fish/completions`, which Fish loads automatically. Re-running the installer updates the completion script without duplicating shell configuration.

To inspect or integrate the scripts manually, print them to standard output:

```bash
tlsferry completion zsh
tlsferry completion bash
tlsferry completion fish
```

Completion covers commands, subcommands, provider names, output formats, common flags, and file/directory arguments. Generated scripts are written only to stdout; unsupported shells and installation failures are written to stderr and return a non-zero exit status.

Example plan:

```text
TLSFerry plan (renew when validity is below 720h)

assets-example
  domains: assets.example.com
  issue:   acme via dns-01 using cloudflare
  deploy:  tencent-cdn -> assets.example.com
  deploy:  aliyun-cdn -> assets.example.com
  deploy:  qiniu-cdn -> assets.example.com
```

## Configuration principles

- Secrets are never stored directly in the main configuration file.
- A credential value such as `keychain:TENCENTCLOUD` references credentials stored by the operating-system credential manager.
- `env:PROFILE` remains available for servers, containers, and CI systems.
- `preflight` checks provider support and required credential fields without printing secret values or contacting cloud APIs.
- DNS-01 is the initial ACME challenge because it works reliably for CDN and object-storage domains and supports wildcard certificates.
- Issuers and deployment providers remain separate so one certificate can be delivered to several cloud platforms.
- Deployments are optional while using issuance-only workflows.

See [`config.example.json`](config.example.json) for the current schema. The guarded real-environment release gate has a separate [`config.release-smoke.example.json`](config.release-smoke.example.json) template.

## Cloud authentication

For a local workstation, store credentials in macOS Keychain, Windows Credential Manager, or Linux Secret Service. `auth login` opens the provider's credential page, reads the values without terminal echo, and stores them outside `config.json`:

```bash
tlsferry auth login tencent
tlsferry auth login aliyun
tlsferry auth login qiniu
tlsferry auth login cloudflare
tlsferry auth status --profile TENCENTCLOUD
```

The resulting references are `keychain:CLOUDFLARE`, `keychain:TENCENTCLOUD`, `keychain:ALIYUN`, and `keychain:QINIU`. Remove one with:

```bash
tlsferry auth logout --profile TENCENTCLOUD
```

The initial release uses a browser-assisted key import rather than claiming a common OAuth flow that the providers do not share. Environment profiles remain compatible; for example, `env:TENCENTCLOUD` reads `TENCENTCLOUD_SECRET_ID` and `TENCENTCLOUD_SECRET_KEY`.

## Discovering CDN domains

After connecting an account, TLSFerry can read the CDN domain inventory without changing cloud resources:

```bash
tlsferry discover cloud --provider tencent
tlsferry discover cloud --provider aliyun
tlsferry discover cloud --provider qiniu
```

The default profiles are `keychain:TENCENTCLOUD`, `keychain:ALIYUN`, and `keychain:QINIU`. Select another account profile or machine-readable output with:

```bash
tlsferry discover cloud \
  --provider tencent \
  --credential keychain:TENCENT_PRODUCTION \
  --format json
```

The table reports provider, domain, cloud-side status, HTTPS state, and CNAME. Tencent Cloud discovery uses the CDN domain configuration list, Alibaba Cloud discovery follows every `DescribeUserDomains` page, and Qiniu uses a signed read-only domain-list request.

Discovery does not issue certificates, enable HTTPS, or import domains into `config.json`. A discovered domain must be selected explicitly through the enrollment preview:

```bash
tlsferry enroll cloud \
  --provider tencent \
  --domain nos.example.com \
  --email ops@example.com \
  --dns-provider cloudflare \
  --dns-credential keychain:CLOUDFLARE
```

The command scans the authorized account and refuses a domain that is absent or already enrolled. It prints the exact ACME and deployment plan without changing the file. Add `--execute` to append only that domain atomically to `config.json`, then run `tlsferry preflight`. Enrollment never issues a certificate, changes DNS, or enables CDN HTTPS by itself.

## DNS-01 providers

TLSFerry changes only the temporary `_acme-challenge` TXT record. It does not change the business CNAME that points a hostname at a CDN. The DNS provider is selected independently from the deployment provider, so a Cloudflare-hosted zone can deliver its certificate to Tencent Cloud CDN.

| DNS provider | Config value | Credential fields | Recommended access |
| --- | --- | --- | --- |
| TLSFerry remote control plane | `tlsferry-cloud` | `API_URL`, short-lived `API_TOKEN` | Job-scoped access to one enrolled hostname and delegated target |
| Cloudflare | `cloudflare` | `API_TOKEN` | `Zone:DNS:Edit` and `Zone:Zone:Read`, restricted to the required zone |
| DNSPod | `dnspod` | `SECRET_ID`, `SECRET_KEY` | DNS record read/write access for the required public zone |
| Alibaba Cloud DNS | `aliyun` | `ACCESS_KEY_ID`, `ACCESS_KEY_SECRET` | DNS record read/write access for the required zone |

For Cloudflare, create a token from the **Edit zone DNS** template, restrict its resources to the required zone, and store it locally:

```bash
tlsferry auth login cloudflare
# Saves keychain:CLOUDFLARE
```

For DNSPod, the existing Tencent Cloud profile can be reused for issuance and Tencent CDN deployment when its CAM policy includes both DNSPod record management and the required SSL/CDN actions:

```bash
tlsferry auth login tencent
# Saves keychain:TENCENTCLOUD
```

If DNS is hosted by Cloudflare while CDN is hosted by Tencent Cloud, use separate credential references in the same certificate entry: `keychain:CLOUDFLARE` under `issuer` and `keychain:TENCENTCLOUD` under `deployments`.

The `tlsferry-cloud` provider is the public executor-side contract for a hosted control plane. It uses a short-lived job token to present and clean one delegated ACME challenge without exposing the control plane's authoritative DNS credentials. The client is implemented in CE, but the hosted endpoint is not included or generally available. See [the remote DNS challenge protocol](docs/remote-dns-protocol.md).

Before the first production issuance, optionally verify that a direct DNS credential can both create and remove the selected ACME record. Preview is read-only:

```bash
tlsferry dns-check \
  --config ~/.config/tlsferry/config.json \
  --certificate assets-example \
  --domain assets.example.com
```

After checking the displayed `_acme-challenge` name, authorize one random temporary TXT record and its immediate cleanup:

```bash
tlsferry dns-check \
  --config ~/.config/tlsferry/config.json \
  --certificate assets-example \
  --domain assets.example.com \
  --confirm-domain assets.example.com \
  --execute
```

This supports direct Cloudflare, DNSPod, and Alibaba Cloud DNS credentials. It does not issue a certificate, change the CDN CNAME, or wait for public DNS propagation. Run it once per certificate domain or DNS zone that needs validation. A cleanup failure means the temporary TXT may still exist and must be removed in the DNS provider console before continuing. The `tlsferry-cloud` provider is intentionally excluded because its token is job-scoped; the hosted executor validates that lifecycle instead.

## Real-environment release smoke

Before a CE release, validate one disposable hostname and one non-production cloud target with the guarded orchestration command. Start by copying the tracked template to a private path. `example.com` is only a reserved placeholder: replace the domain, target, and email with a test hostname you own before execution. Keep the exact Let's Encrypt staging directory; the command refuses the production directory.

```bash
mkdir -p ~/.config/tlsferry
cp config.release-smoke.example.json ~/.config/tlsferry/release-smoke.json
# Edit ~/.config/tlsferry/release-smoke.json before continuing.
tlsferry auth login cloudflare --profile CLOUDFLARE_STAGING
tlsferry auth login tencent --profile TENCENTCLOUD_STAGING
```

The default mode only prints the selected DNS and cloud path:

```bash
tlsferry release-smoke \
  --config ~/.config/tlsferry/release-smoke.json \
  --certificate staging-cdn \
  --provider tencent-cdn
```

After reviewing the preview, explicitly type the configured test target:

```bash
tlsferry release-smoke \
  --config ~/.config/tlsferry/release-smoke.json \
  --certificate staging-cdn \
  --provider tencent-cdn \
  --confirm-test-target staging.example.com \
  --accept-tos \
  --execute
```

This runs the existing `preflight`, `issue`, and single-target `deploy` paths and writes `.tlsferry/release-smoke/evidence.json` with certificate domains, issue time, public-certificate SHA-256, provider target, and provider request reference. It never writes credentials, challenge values, certificate PEM, or the private key to evidence. The evidence deliberately remains `pending_cleanup`: restore the previous cloud certificate binding and remove the uploaded test certificate, then record the sanitized rollback result in `docs/release-evidence.md`. TLSFerry does not guess or automate provider rollback because replacing a live binding is an externally consequential operation.

After restoring the previous binding and removing the uploaded test certificate, record the operator result without overwriting the original evidence:

```bash
tlsferry release-smoke cleanup \
  --evidence .tlsferry/release-smoke/evidence.json \
  --confirm-test-target staging.example.com \
  --cleanup-reference ticket/cleanup-42
```

This creates `evidence.json.ready.json` with `operator_confirmed` cleanup and `ready_for_review` gate status. The reference must be a sanitized provider request ID, ticket ID, or audit reference—not a token or credential. The status deliberately does not say Pass: a release reviewer must still compare the record with the provider-side state.

Use an isolated test configuration, test hostname, state directory, and least-privilege credentials. A typed target confirmation is a guardrail, not proof that a hostname is non-production.

## Issuing a certificate

Store the DNS credential referenced by the selected certificate, then run `issue`. The example configuration uses Cloudflare DNS and Tencent Cloud deployment:

```bash
tlsferry auth login cloudflare
tlsferry auth login tencent
go run ./cmd/tlsferry issue \
  --config config.example.json \
  --certificate assets-example \
  --accept-tos
```

For a headless server or CI job, change the issuer reference to `env:CLOUDFLARE` and export:

```bash
export CLOUDFLARE_API_TOKEN=...
go run ./cmd/tlsferry issue \
  --config config.example.json \
  --certificate assets-example \
  --accept-tos
```

The command creates or reuses an ACME account under `.tlsferry/accounts` and writes `cert.pem`, `chain.pem`, `fullchain.pem`, `key.pem`, and `metadata.json` under `.tlsferry/certificates/<name>`. Account keys and certificate private keys use restricted filesystem permissions.

Use the Let's Encrypt staging directory while testing to avoid production rate limits:

```json
"directory_url": "https://acme-staging-v02.api.letsencrypt.org/directory"
```

Passing `--accept-tos` records explicit acceptance of the ACME provider's terms. The command performs real DNS and ACME API operations; `validate`, `plan`, and `preflight` remain read-only.

## Deploying a certificate

`deploy` loads the previously issued certificate, verifies that its private key matches, and executes exactly one configured deployment:

```bash
go run ./cmd/tlsferry deploy \
  --config config.example.json \
  --certificate assets-example \
  --provider tencent-cdn \
  --execute
```

Supported deployment providers:

| Provider | Target | Options | Credential variables |
| --- | --- | --- | --- |
| `tencent-cdn` | CDN domain | `billing`: `on` (default) or `off` | `<PROFILE>_SECRET_ID`, `<PROFILE>_SECRET_KEY` |
| `tencent-cos` | Custom domain | required `region` and `bucket` | `<PROFILE>_SECRET_ID`, `<PROFILE>_SECRET_KEY` |
| `aliyun-cdn` | CDN domain | optional `region` (default `cn-hangzhou`) | `<PROFILE>_ACCESS_KEY_ID`, `<PROFILE>_ACCESS_KEY_SECRET` |
| `qiniu-cdn` | CDN domain | optional `force_https` and `http2` | `<PROFILE>_ACCESS_KEY`, `<PROFILE>_SECRET_KEY` |

Tencent Cloud deployment uses SSL certificate management. CDN and COS operations are asynchronous; the command reports `submitted` with the Tencent deployment record ID. Alibaba Cloud and Qiniu return `applied` after their update API accepts the configuration.

`--execute` is mandatory because deployment changes external cloud resources. TLSFerry never prints secret values.

## Automated renewal

`renew` checks each stored certificate against `renew_before`. Certificates outside the renewal window are skipped; due or missing certificates are issued, saved, and delivered to every configured deployment in order.

```bash
go run ./cmd/tlsferry renew \
  --config config.example.json \
  --accept-tos \
  --execute
```

Operational safeguards:

- A state-directory lock prevents overlapping renewal processes.
- Every ACME, DNS, and cloud deployment operation has bounded retries; configure the limit with `--retry-attempts`.
- `--certificate NAME` limits a run to one certificate.
- `--force` bypasses the expiry check for recovery or testing.
- Stage events are emitted through a pluggable notifier interface; the CLI currently writes them to standard output for cron, systemd, and log collectors.
- Both `--accept-tos` and `--execute` are mandatory because a due renewal performs real external operations.

### Automatic checks on macOS, Linux, and Windows

Build or install TLSFerry at a permanent path first. Do not install the service from `go run`, because Go's temporary executable is removed when the command exits.

```bash
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/tlsferry" ./cmd/tlsferry
"$HOME/.local/bin/tlsferry" service install \
  --config "$PWD/config.json" \
  --accept-tos \
  --execute
```

On macOS, the user-level `launchd` service runs once at login and daily at 03:17. On Linux, a native systemd user timer runs daily and uses `Persistent=true` to catch a missed check after the machine comes back online. On Windows, a least-privilege Task Scheduler entry runs daily, starts a missed run when possible, requires network availability, and ignores overlapping runs. No platform keeps TLSFerry running between checks.

```bash
tlsferry service status
tlsferry service run-now
tlsferry service logs
tlsferry service uninstall
```

`service install` converts the configuration, state, output, and binary paths to absolute paths. Its launchd plist, systemd units, or Task Scheduler XML contain no cloud secrets. Scheduled runs should use `keychain:` credentials on desktops or a service-scoped environment on servers because interactive shell exports are not inherited reliably.

Linux installs `tlsferry-renew.service` and `tlsferry-renew.timer` under the current user's systemd directory and enables the timer with `systemctl --user`. The user manager must be available; on a headless server, enable linger for the dedicated service account if timers must run while it is logged out.

Windows registers `TLSFerry Renewal` for the current interactive user. `tlsferry service logs` prints the `schtasks.exe` query needed to inspect the last result. The installer does not request or store a Windows password; a server task that must run while no user is signed in should be assigned manually to a dedicated task identity with its own protected credential environment.

## Planned milestones

1. Add renewable STS/OIDC/SSO and cloud instance-role credential adapters.
2. Add webhook and email notification adapters.
3. Add Alibaba Cloud OSS and Qiniu Kodo discovery/deployment where provider APIs support custom-domain certificate binding.

## Development

```bash
make verify
```

Source builds require Go 1.25 or newer. `make verify` includes a fixed-version reachable-code vulnerability scan.

The full release gate is documented in [docs/release-checklist.md](docs/release-checklist.md). From a clean tracked worktree, run:

```bash
make release-check
```

To generate reproducible JSON for a specific candidate's source and edition-boundary review:

```bash
make release-audit AUDIT_VERSION=v0.1.0-rc.1 AUDIT_REVIEWER=maintainer-name
```

The report records the exact commit and keeps real DNS-01 issuance, provider deployment, GitHub metadata, CI, and tag publication as explicit manual gates.

Exercise the complete credential-free CLI flow with reserved domains and synthetic credentials:

```bash
make functional-smoke
```

This checks validation, planning, preflight, read-only cloud discovery, enrollment preview, selected-domain-only persistence, and secret-safe output without contacting ACME, DNS, or a cloud API.

After building a snapshot, verify all six archives and execute the current platform's archived binary:

```bash
make release-snapshot
make artifact-smoke
```

The artifact report binds the archives to the current commit and GoReleaser version, verifies every checksum, rejects unsafe or unexpected files, and confirms bundled public files match the source tree byte for byte.

Create a local release archive with [GoReleaser](https://goreleaser.com/) installed:

```bash
make release-snapshot
```

Pushing a `v*` tag runs the same verification and publishes versioned cross-platform archives plus SHA-256 checksums.

The canonical Go module path is `github.com/nonozone/TLSFerry`.

## License

TLSFerry is open source under the [Apache License 2.0](LICENSE). Contributions are welcome under the same license.

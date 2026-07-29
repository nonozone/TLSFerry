# TLSFerry

TLSFerry is a Go-based TLS certificate automation tool for issuing certificates through ACME and delivering them to multiple cloud platforms.

The project can issue real certificates through ACME DNS-01 and deploy them to Tencent Cloud CDN/COS, Alibaba Cloud CDN, and Qiniu CDN.

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
go run ./cmd/tlsferry issue --config config.example.json --certificate assets-example --accept-tos
go run ./cmd/tlsferry deploy --config config.example.json --certificate assets-example --provider tencent-cdn --execute
go run ./cmd/tlsferry renew --config config.example.json --accept-tos --execute
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

See [`config.example.json`](config.example.json) for the current schema.

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

Discovery does not issue certificates, enable HTTPS, or import domains into `config.json`. Selective import and DNS-control verification are the next safety boundary; a discovered domain must not be silently taken over.

## DNS-01 providers

TLSFerry changes only the temporary `_acme-challenge` TXT record. It does not change the business CNAME that points a hostname at a CDN. The DNS provider is selected independently from the deployment provider, so a Cloudflare-hosted zone can deliver its certificate to Tencent Cloud CDN.

| DNS provider | Config value | Credential fields | Recommended access |
| --- | --- | --- | --- |
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

### Automatic checks on macOS

Build or install TLSFerry at a permanent path first. Do not install the service from `go run`, because Go's temporary executable is removed when the command exits.

```bash
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/tlsferry" ./cmd/tlsferry
"$HOME/.local/bin/tlsferry" service install \
  --config "$PWD/config.json" \
  --accept-tos \
  --execute
```

The user-level `launchd` service runs once at login and daily at 03:17. It does not keep a window or daemon open between checks. The computer must be awake and online occasionally; launchd runs a sleeping machine's scheduled check after wake.

```bash
tlsferry service status
tlsferry service run-now
tlsferry service logs
tlsferry service uninstall
```

`service install` converts the configuration, state, output, and binary paths to absolute paths. Its plist contains no cloud secrets. Scheduled runs should use `keychain:` credentials because a GUI launch agent does not inherit credentials exported in an interactive shell.

For unattended Linux servers, cron and systemd timers remain supported manually. Native `systemd timer` installation is the next platform adapter.

## Planned milestones

1. Add selective import, DNS-control verification, and explicit enrollment for discovered domains.
2. Add native systemd timer and Windows Task Scheduler installers.
3. Add renewable STS/OIDC/SSO and cloud instance-role credential adapters.
4. Add webhook and email notification adapters.
5. Add Alibaba Cloud OSS and Qiniu Kodo discovery/deployment where provider APIs support custom-domain certificate binding.

## Development

```bash
make verify
```

The module path currently uses `github.com/nonozone/TLSFerry` and can be changed before the repository is published if the final GitHub owner differs.

## License

TLSFerry is open source under the [Apache License 2.0](LICENSE). Contributions are welcome under the same license.

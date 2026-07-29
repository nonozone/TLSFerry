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
```

Example plan:

```text
TLSFerry plan (renew when validity is below 720h)

assets-example
  domains: assets.example.com
  issue:   acme via dns-01 using dnspod
  deploy:  tencent-cdn -> assets.example.com
  deploy:  aliyun-cdn -> assets.example.com
  deploy:  qiniu-cdn -> assets.example.com
```

## Configuration principles

- Secrets are never stored directly in the main configuration file.
- A credential value such as `env:TENCENTCLOUD` references environment-based credentials that a provider adapter will resolve.
- `preflight` checks provider support and required environment variables without printing secret values or contacting cloud APIs.
- Credential profiles expand into provider-specific variables, such as `env:TENCENTCLOUD` requiring `TENCENTCLOUD_SECRET_ID` and `TENCENTCLOUD_SECRET_KEY`.
- DNS-01 is the initial ACME challenge because it works reliably for CDN and object-storage domains and supports wildcard certificates.
- Issuers and deployment providers remain separate so one certificate can be delivered to several cloud platforms.
- Deployments are optional while using issuance-only workflows.

See [`config.example.json`](config.example.json) for the current schema.

## Issuing a certificate

Set the credential variables referenced by the selected certificate, then run `issue`. For example, `env:TENCENTCLOUD` uses:

```bash
export TENCENTCLOUD_SECRET_ID=...
export TENCENTCLOUD_SECRET_KEY=...
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

For unattended operation, schedule the compiled binary with cron or a systemd timer. Run it at least daily; the expiry window prevents unnecessary certificate orders.

## Planned milestones

1. Add webhook and email notification adapters.
2. Add Alibaba Cloud OSS and Qiniu Kodo deployment where provider APIs support custom-domain certificate binding.
3. Add daemon mode and an optional Web console after the automation core is stable.

## Development

```bash
make verify
```

The module path currently uses `github.com/nonozone/TLSFerry` and can be changed before the repository is published if the final GitHub owner differs.

## License

TLSFerry is open source under the [Apache License 2.0](LICENSE). Contributions are welcome under the same license.

# TLSFerry

TLSFerry is a Go-based TLS certificate automation tool for issuing certificates through ACME and delivering them to multiple cloud platforms.

The project is currently at the initial MVP stage. The first milestone establishes a stable configuration model and execution plan before real cloud credentials or production certificate operations are introduced.

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

See [`config.example.json`](config.example.json) for the current schema.

## Planned milestones

1. Integrate an ACME client and DNS-01 provider interface.
2. Add Tencent Cloud CDN/COS certificate deployment.
3. Add Alibaba Cloud CDN/OSS certificate deployment.
4. Add Qiniu CDN/Kodo certificate deployment.
5. Add certificate state, renewal scheduling, retries, locking, and notifications.
6. Add daemon mode and an optional Web console after the automation core is stable.

## Development

```bash
make verify
```

The module path currently uses `github.com/nonozone/TLSFerry` and can be changed before the repository is published if the final GitHub owner differs.

## License

TLSFerry is open source under the [Apache License 2.0](LICENSE). Contributions are welcome under the same license.

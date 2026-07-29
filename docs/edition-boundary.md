# TLSFerry edition boundary

TLSFerry CE and TLSFerry Cloud share protocols, not a deployment unit or private control-plane source tree.

## Community Edition owns

- ACME issuance, local certificate storage, renewal, and host schedulers.
- DNS and certificate deployment provider adapters.
- Cloud account discovery that is explicitly read-only until a domain is enrolled.
- CLI help, completion, configuration validation, and local credential references.
- Versioned public executor protocols, including `remote-dns-protocol.md`.

## Cloud owns privately

- User accounts, organizations, sessions, tenants, plans, quotas, and billing.
- Cloudflare Workers/Hono control-plane endpoints and the Vue console.
- D1 schemas and migrations, Workflows/Cron orchestration, and R2 archives.
- Authoritative validation-zone credentials and challenge-record lifecycle.
- Executor registration, job assignment, audit history, notification delivery, and operational administration.

Cloud-private implementations must live in a separate private repository. They must not be added under `cloud/`, `apps/cloud-console/`, `internal/account/`, `internal/billing/`, `internal/controlplane/`, `internal/tenant/`, or `migrations/` in this public repository. The CE test suite enforces those path boundaries.

## Allowed crossing points

An edition crossing point must be all of the following:

1. Publicly documented and versioned.
2. Implementable by another compatible service without private source access.
3. Scoped to the minimum job or hostname authority.
4. Free of Cloud database, billing, tenant, or console dependencies.
5. Covered by CE contract tests that do not require the hosted service.

The current crossing point is the remote DNS challenge protocol. Its bearer token is job-scoped, CE never receives authoritative DNS credentials, and remote response bodies are not trusted as log-safe text.

## Change review

Any change that introduces hosted account state, multi-tenant authorization, payment logic, authoritative DNS ownership, or control-plane persistence belongs in the private Cloud repository. Shared certificate behavior should be implemented in CE first and exposed through a narrow protocol only when the hosted service needs it.

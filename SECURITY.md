# Security policy

## Supported versions

Before the first stable tag, security fixes are applied to `main`. After stable releases begin, the latest release line and `main` receive security fixes; older release lines are unsupported unless a release note says otherwise.

## Reporting a vulnerability

Do not open a public issue containing an exploit, credential, private key, ACME challenge value, or affected account details. Use the repository's **Security → Report a vulnerability** flow to create a private GitHub Security Advisory report.

Include the affected version or commit, operating system, provider, reproduction steps, impact, and whether any credential or certificate material may have been exposed. Use synthetic credentials and domains whenever possible.

## Response expectations

Maintainers will validate the report, establish severity, prepare a fix and regression test, and coordinate disclosure. A public advisory or release note will avoid exposing live credentials, private keys, account identifiers, or still-usable challenge material.

## Security boundaries

- Cloud credentials belong in the operating-system credential manager or a service-scoped environment, never in committed configuration.
- Certificate and ACME private keys are local sensitive state and must not be attached to reports.
- TLSFerry Cloud control-plane code and authoritative DNS credentials are outside this public CE repository.
- Public edition crossing points must follow [the edition boundary](docs/edition-boundary.md) and their versioned protocol contracts.

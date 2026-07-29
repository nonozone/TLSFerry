# Contributing to TLSFerry CE

TLSFerry CE is Apache-2.0 licensed. Contributions should keep the command-line workflow explicit, provider-neutral where practical, and safe for unattended operation.

## Development workflow

1. Use Go 1.25.12 or newer in the 1.25 release line.
2. Add focused tests before changing behavior.
3. Run `make verify`.
4. For release-surface changes, run `make release-check` from a clean tracked worktree after committing.
5. Do not commit `config.json`, `.tlsferry/`, `.flowlatch/`, certificates, private keys, tokens, or generated `bin/` and `dist/` files.

## Edition boundary

The public repository owns ACME, provider adapters, local schedulers, discovery, explicit enrollment, and public protocols. Accounts, tenants, billing, hosted DNS credentials, databases, orchestration, audit storage, and the Cloud console belong in the separate private Cloud repository. Changes must satisfy [docs/edition-boundary.md](docs/edition-boundary.md) and its automated path checks.

## Provider changes

- Keep DNS issuance separate from certificate deployment.
- Resolve credentials through `env:` or `keychain:` references; never accept literal secrets in configuration.
- Require explicit execution flags for external mutations.
- Avoid including provider response bodies, authorization headers, challenge values, or private keys in errors and logs.
- Add contract tests with fake clients or local HTTP servers. Tests must not require a contributor's cloud account.

## Pull requests

Keep each pull request focused and explain operator-visible behavior, security implications, verification commands, and any platform or provider limitation. Changes to public protocols require matching documentation and compatibility tests.

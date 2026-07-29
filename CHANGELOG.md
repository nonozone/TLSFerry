# Changelog

All notable changes to TLSFerry CE will be documented here. The project follows semantic versioning once the first stable release is published.

## Unreleased

### Added

- ACME DNS-01 issuance with Cloudflare, DNSPod, Alibaba Cloud DNS, and the public TLSFerry Cloud challenge protocol.
- Certificate deployment to Tencent Cloud CDN/COS, Alibaba Cloud CDN, and Qiniu CDN.
- Read-only cloud CDN discovery and preview-first, one-domain explicit enrollment.
- Operating-system credential storage, shell completion, retries, overlap locking, and renewal planning.
- Native unattended scheduling through macOS launchd, Linux systemd user timers, and Windows Task Scheduler.
- Cross-platform GitHub Releases, checksums, pinned release tooling, vulnerability scanning, and CE/Cloud boundary enforcement.

### Security

- Private key and ACME account files use restricted permissions where the operating system exposes POSIX modes.
- Remote and provider response bodies are excluded from executor errors when they may contain reflected sensitive material.
- Reachable dependency and standard-library vulnerabilities are release-blocking through `govulncheck`.

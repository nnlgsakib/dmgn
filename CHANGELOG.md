# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- README.md with professional structure, badges, and table of contents
- CONTRIBUTING.md with contribution guidelines
- LICENSE file (MIT)

### Changed

- README.md badges for Go version, license, and status
- README.md table of contents for navigation
- README.md expanded security section with reporting guidelines
- README.md community section with discussion links
- README.md expanded acknowledgments

## [0.1.0] - 2025-04-09

### Added

- **Phase 1: Local Foundation**
  - Identity generation and storage (ed25519)
  - Local memory storage with BadgerDB
  - CLI commands (init, add, query, status)
  - Memory graph with links

- **Phase 2: Encryption & API**
  - Full AES-GCM-256 encryption
  - REST API with authentication
  - Identity backup and import/export
  - Memory retention policies

- **Phase 3: Networking Core**
  - libp2p host initialization
  - DHT and mDNS peer discovery
  - TCP transports
  - Basic protocol handlers
  - Peers CLI command
  - Live status detection

## [0.0.1] - 2025-01-15

### Added

- Initial project setup
- Basic memory storage prototype

---

## Version History

| Version | Date | Notes |
|---------|------|-------|
| 0.1.0 | 2025-04-09 | Three phases complete |
| 0.0.1 | 2025-01-15 | Initial prototype |

## Upgrading

When upgrading between versions, check Breaking Changes below:

- **0.0.x to 0.1.x**: Key derivation changed (HKDF), requires re-initialization

## Deprecation Notices

None at this time.

## Security Updates

For security vulnerabilities, see [SECURITY.md](SECURITY.md).
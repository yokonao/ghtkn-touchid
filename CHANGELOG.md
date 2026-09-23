# Changelog

## v0.2.0 - 2026-09-23

### Changed

- **Breaking:** `ghtkn-touchid-reset` is now `ghtkn-touchid reset`. The helper ships as a single binary, and `ghtkn-touchid` with no arguments still unlocks.
- The Keychain access list trusts only the `ghtkn-touchid` binary that ran the reset, so the binary no longer has to be installed next to a companion.

### Added

- An experimental Go port under `go/`, built with cobra and cgo. It is not part of the Homebrew formula.

### Upgrading

The stored passphrase keeps working; no reset is needed. macOS asks once for permission to use the Keychain item after the upgrade. Approve it, or run `ghtkn-touchid reset` to rewrite the access list for the new binary.

## v0.1.2 - 2026-09-06

### Added

- Homebrew formula, served from this repository as its own tap.
- `make test-integration GHTKN_VERSION=vX.Y.Z` to test against a specific ghtkn release. v0.3.4–v0.4.0 are supported.

## v0.1.1 - 2026-09-06

### Fixed

- The build creates its output directory itself.

## v0.1.0 - 2026-09-06

### Fixed

- The passphrase is stored in the login Keychain again, because the data protection Keychain needs an entitlement ad-hoc signed binaries cannot carry.
- The reset drains ghtkn's PTY output while waiting, so ghtkn no longer stalls on a full terminal queue.

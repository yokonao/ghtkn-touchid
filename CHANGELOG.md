# Changelog

## v0.3.3 - 2026-09-26

### Changed

- Touch ID and Keychain are called through [appleframeworks](https://github.com/yokonao/appleframeworks) instead of purego directly.
- Keychain access no longer uses the deprecated SecKeychain and SecAccess APIs. New items get macOS's default access list, which trusts only the binary that ran the reset, as before.

### Upgrading

The stored passphrase keeps working; no reset is needed.

## v0.3.2 - 2026-09-25

### Fixed

- v0.3.0 and v0.3.1 accidentally included the old Swift build output, which made the Go module about 141MB; both are retracted. The binaries on GitHub Releases were not affected.

## v0.3.1 - 2026-09-25

### Added

- Prebuilt darwin/amd64 and darwin/arm64 binaries on GitHub Releases, with build provenance attestations. See [docs/install.md](docs/install.md).

## v0.3.0 - 2026-09-25

### Changed

- **Breaking:** The helper is now written in Go, and the Swift implementation is gone. It uses the same Keychain items and command line.
- **Breaking:** Homebrew is no longer supported. Install with `go install github.com/yokonao/ghtkn-touchid/cmd/ghtkn-touchid@latest`, which needs Go 1.27.1 or later. The binary no longer needs cgo.
- `make test-integration GHTKN_VERSION=vX.Y.Z` is now `GHTKN_VERSION=vX.Y.Z go test -tags integration ./...`.

### Upgrading

Remove the Homebrew install with `brew uninstall ghtkn-touchid && brew untap yokonao/ghtkn-touchid`, then install with `go install`. The stored passphrase keeps working; no reset is needed. macOS asks once for permission to use the Keychain item from the new binary. Approve it, or run `ghtkn-touchid reset` to rewrite the access list for the new binary.

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

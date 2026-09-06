# ghtkn-touchid

`ghtkn-touchid` unlocks a local [ghtkn](https://github.com/suzuki-shunsuke/ghtkn) agent with a passphrase protected by Touch ID in macOS Keychain. It is an independent helper and is not part of the ghtkn project.

## Requirements

- macOS 13 or later with Touch ID
- Swift 5.9 or later
- ghtkn v0.4.0 configured with the agent backend, available in an absolute `PATH` entry

The helper implements the public newline-delimited JSON agent protocol v1 from [ghtkn-go-sdk v0.6.1](https://github.com/suzuki-shunsuke/ghtkn-go-sdk/blob/v0.6.1/ghtkn/backend/agent/protocol.go), and follows ghtkn's socket lookup order: `GHTKN_AGENT_SOCKET`, `XDG_RUNTIME_DIR`, `XDG_CACHE_HOME`, then `~/.cache/ghtkn/agent.sock`.

## Install

This repository is also its own Homebrew tap, so the formula is tapped from the repository URL:

```sh
brew tap yokonao/ghtkn-touchid https://github.com/yokonao/ghtkn-touchid
brew install yokonao/ghtkn-touchid/ghtkn-touchid
```

To build from a checkout instead, run `make build` and `make test`, then `./install.sh` to write both binaries to `~/.local/bin`. The installer is optional, but keep the two binaries together in one directory: the reset grants Keychain access to the pair it finds beside itself, so moving them afterward makes macOS ask for permission until the reset runs again.

`make test-integration` drives a real, isolated `ghtkn agent` through the actual unlock protocol (Keychain and Touch ID are not involved); run it after bumping the pinned ghtkn/SDK version, with `ghtkn` available in `PATH`.

## Unlock

```sh
ghtkn agent start &
ghtkn-touchid
```

Each unlock from the locked state requires Touch ID. If the agent is already unlocked, it does not access Keychain.

A fresh install has no passphrase to read yet, so run `ghtkn-touchid-reset` before the first unlock, and `ghtkn auth` after it.

## Reset

```sh
ghtkn-touchid-reset
```

The reset requires a terminal, Touch ID, and the exact confirmation `RESET`. It generates a 256-bit random passphrase, resets the ghtkn agent with it through a PTY, and stores it in a Keychain item whose access list names the two installed helper binaries.

The ghtkn reset stops the agent and deletes its encryption key and cached access and refresh tokens, so the configured GitHub Apps need `ghtkn auth` again.

## Troubleshooting

`ghtkn-touchid: the ghtkn passphrase is not stored in Keychain` — nothing has written the passphrase yet. Run `ghtkn-touchid-reset` once, as in the first run.

`A required entitlement isn't present.` — an older build stored the passphrase in the data protection Keychain, which needs an entitlement these helpers cannot carry. Reinstall from the current sources; the reset aborts before it touches the agent, so nothing is lost.

macOS asks for permission to use the Keychain item — the access list names the installed helper binaries, and reinstalling or upgrading replaces them. Approving the prompt keeps the item usable, and `ghtkn-touchid-reset` rewrites the access list for the new binaries.

## Security model

The passphrase is not accepted through arguments or environment variables and is not written to stdout, logs, or the clipboard. Requests and PTY buffers are overwritten after use. Agent errors are not relayed because an untrusted socket could reflect a passphrase.

The passphrase item lives in the login Keychain with an access list that trusts only the two helper binaries. macOS enforces that list; the biometric check is enforced by the helpers themselves.

This helper does not protect against:

- a compromised user session, kernel, or helper binary;
- a program the user authorizes at the macOS Keychain prompt;
- another process using the agent socket while the agent is unlocked;
- disclosure from ghtkn itself or from a compromised same-user agent;
- physical access after successful biometric authentication.

Reset resolves `ghtkn` from absolute entries in `PATH`; empty and relative entries are ignored. Keep the helper, the resolved ghtkn executable, and the socket directory writable only by the current user. Lock or stop the agent when it is not needed.

## License

MIT. ghtkn and ghtkn-go-sdk are separate MIT-licensed projects; their names identify compatibility only.

# ghtkn-touchid

`ghtkn-touchid` unlocks a local [ghtkn](https://github.com/suzuki-shunsuke/ghtkn) agent with a passphrase protected by Touch ID in macOS Keychain. It is an independent helper and is not part of the ghtkn project.

## Requirements

- macOS 13 or later with Touch ID and a device passcode
- Swift 5.9 or later
- ghtkn v0.4.0 available in an absolute `PATH` entry
- the ghtkn agent backend

The helper implements the public newline-delimited JSON agent protocol v1 from [ghtkn-go-sdk v0.6.1](https://github.com/suzuki-shunsuke/ghtkn-go-sdk/blob/v0.6.1/ghtkn/backend/agent/protocol.go). It follows ghtkn's documented socket lookup order: `GHTKN_AGENT_SOCKET`, `XDG_RUNTIME_DIR`, `XDG_CACHE_HOME`, then `~/.cache/ghtkn/agent.sock`.

## Build, test, and install

```sh
make build
make test
./install.sh
```

The installer is optional. It writes `ghtkn-touchid` and `ghtkn-touchid-reset` to `~/.local/bin`; the binaries can be installed elsewhere.

## First-time setup

Only `ghtkn-touchid-reset` writes the passphrase to Keychain, so run it once before the first unlock. Without it, `ghtkn-touchid` fails with `the ghtkn passphrase is not stored in Keychain`.

```sh
ghtkn-touchid-reset
ghtkn agent start &
ghtkn-touchid
ghtkn auth
```

The reset replaces the ghtkn agent key, so `ghtkn auth` is needed afterward to reauthenticate the configured GitHub Apps.

## Unlock

```sh
ghtkn agent start &
ghtkn-touchid
```

Each unlock from the locked state requires Touch ID. The helper enables refresh tokens with a seven-day unused-token TTL. If the agent is already unlocked, it does not access Keychain.

## Destructive reset

```sh
ghtkn-touchid-reset
```

The same command is used for the first-time setup and for rotating the passphrase later. It requires a terminal, Touch ID, and the exact confirmation `RESET`. It generates a 256-bit random passphrase, stages it in Keychain, and runs `ghtkn agent reset` through a PTY. It confirms the ghtkn reset, waits for terminal echo to be disabled, then sends the passphrase twice without depending on prompt text. The ghtkn reset stops the agent and deletes its encryption key and cached access and refresh tokens. Reauthenticate configured GitHub Apps with `ghtkn auth` afterward.

Keychain uses `pending`, `committed`, and active items. A failed ghtkn reset keeps the old active item and the pending passphrase. An interrupted commit keeps either the active or committed item so the unlock helper can try recovery candidates.

## Security model

The passphrase is not accepted through arguments or production environment variables and is not written to stdout, logs, or the clipboard. Requests and PTY buffers are overwritten after use. Agent errors are not relayed because an untrusted socket could reflect a passphrase. Keychain items use `kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly` with `biometryCurrentSet`; changing the enrolled biometric set invalidates access.

This helper does not protect against:

- a compromised user session, kernel, or helper binary;
- another process using the agent socket while the agent is unlocked;
- disclosure from ghtkn itself or from a compromised same-user agent;
- physical access after successful biometric authentication.

Reset resolves `ghtkn` from absolute entries in `PATH`; empty and relative entries are ignored. Keep the helper, the resolved ghtkn executable, and the socket directory writable only by the current user. Lock or stop the agent when it is not needed.

## License

MIT. ghtkn and ghtkn-go-sdk are separate MIT-licensed projects; their names identify compatibility only.

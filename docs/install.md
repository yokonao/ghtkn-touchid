# Install

ghtkn-touchid is a single binary for macOS. Install it into a directory in your `PATH` in one of these ways:

1. [GitHub Releases](#github-releases)
1. [mise](#mise)
1. [go install](#go-install)

The reset grants Keychain access to the binary that runs it, so an upgraded or moved binary makes macOS ask for permission until the reset runs again. A fresh install has no passphrase yet; run `ghtkn-touchid reset` once before the first unlock.

## GitHub Releases

Download the archive for your Mac from [GitHub Releases](https://github.com/yokonao/ghtkn-touchid/releases), then install the binary into `PATH`:

```sh
asset=ghtkn-touchid_darwin_arm64.tar.gz # ghtkn-touchid_darwin_amd64.tar.gz on Intel
gh release download -R yokonao/ghtkn-touchid -p "$asset" # the latest release; pass a tag for another
tar -xzf "$asset" ghtkn-touchid
mkdir -p ~/.local/bin
install -m 755 ghtkn-touchid ~/.local/bin/
```

The binaries are not notarized. `gh` and `curl` do not mark downloads as quarantined, but a browser does, and macOS then refuses to run the binary; remove the mark with `xattr -d com.apple.quarantine ghtkn-touchid`.

### Verify the archive

Each archive has a [build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations) from the release workflow. Verify it with the [GitHub CLI](https://cli.github.com/):

```sh
gh attestation verify "$asset" \
  -R yokonao/ghtkn-touchid \
  --signer-workflow yokonao/ghtkn-touchid/.github/workflows/release.yaml
```

## mise

With [mise](https://mise.jdx.dev/):

```sh
mise use -g github:yokonao/ghtkn-touchid
```

mise verifies the [build provenance attestation](#verify-the-archive) automatically.

## go install

With Go 1.27.1 or later:

```sh
go install github.com/yokonao/ghtkn-touchid/cmd/ghtkn-touchid@latest
```

The binary lands in `$(go env GOBIN)`, or `~/go/bin` when that is unset.

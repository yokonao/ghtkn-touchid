# Releasing

Add the release to `CHANGELOG.md` first; it has to be in the tagged tree.

Homebrew builds from the source tarball GitHub generates for a tag, so a release is a tag plus a formula bump. No GitHub release page is involved.

```sh
git tag -a vX.Y.Z -m vX.Y.Z
git push origin vX.Y.Z
curl -sL https://github.com/yokonao/ghtkn-touchid/archive/refs/tags/vX.Y.Z.tar.gz | shasum -a 256
```

Put that checksum and the new tag in `url` and `sha256` in `Formula/ghtkn-touchid.rb`, then push the formula to `main`; the tap serves the formula from `main`, so it takes effect on the next `brew update`.

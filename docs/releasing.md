# Releasing

Homebrew builds from the source tarball GitHub generates for a tag, so a release is a tag plus a formula bump. No GitHub release page is involved.

```sh
git tag -a v0.1.2 -m v0.1.2
git push origin v0.1.2
curl -sL https://github.com/yokonao/ghtkn-touchid/archive/refs/tags/v0.1.2.tar.gz | shasum -a 256
```

Put that checksum and the new tag in `url` and `sha256` in `Formula/ghtkn-touchid.rb`, then push the formula to `main`; the tap serves the formula from `main`, so it takes effect on the next `brew update`.

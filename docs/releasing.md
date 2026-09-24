# Releasing

Add the release to `CHANGELOG.md` first; it has to be in the tagged tree.

A release is a tag; `go install ...@latest` picks it up through the Go module proxy. No GitHub release page is involved.

```sh
git tag -a vX.Y.Z -m vX.Y.Z
git push origin vX.Y.Z
```

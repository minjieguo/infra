# pkg
golang common package

## Release

This repository uses Go module versions from Git tags.

Run the release script after committing your changes:

```bash
./scripts/release-push.sh
```

The script creates the next patch tag, such as `v0.0.1` or `v0.0.2`, then pushes the current branch and the new tag to `origin`.

Other bump types are also supported:

```bash
./scripts/release-push.sh minor
./scripts/release-push.sh major
```

Update this package in another Go project with:

```bash
go get github.com/LostRoy/pkg@latest
```

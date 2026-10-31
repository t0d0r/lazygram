# Continuous integration

Two GitHub Actions workflows, `.github/workflows/linux.yml` (name
`Linux`, ubuntu-latest) and `macos.yml` (name `macOS`, macos-latest),
run on every pull request and on every push to `master`. Each job checks
the repository out, installs the Go version from `go.mod`
(`actions/setup-go`) and golangci-lint (`golangci/golangci-lint-action`,
install only), then runs `make check` (gofmt, go vet, golangci-lint,
tests) and `make smoke` (the demo, non-interactive, one screen). The
jobs need no secrets and get read-only repository permissions; they
never touch Telegram.

The README shows the two workflow badges and the latest release badge,
after nchat's README. A red badge means `make check` or `make smoke`
fails on that platform at the head of `master`.

Releases stay a manual step (`make dist`, `make release`, see
release.md); CI does not publish anything.

Security note: the workflows run only the Makefile targets on code from
the repository, with `contents: read`; a pull request from a fork cannot
reach any secret because there are none, and the release tarballs are
never built or signed by CI.

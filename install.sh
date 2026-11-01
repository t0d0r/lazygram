#!/bin/sh
# lazygram installer. Run it as
#
#   curl -fsSL https://github.com/t0d0r/lazygram/releases/latest/download/install.sh | sh
#
# or download it, read it, and run it with sh. It installs the latest
# lazygram release (or LAZYGRAM_VERSION=vX.Y.Z) for this system into
# $LAZYGRAM_PREFIX/bin (default ~/.local/bin):
#
#   1. the latest version is read from where GitHub's releases/latest
#      redirects, without following it; only vX.Y.Z is accepted;
#   2. the release tarball and SHA256SUMS are downloaded over HTTPS only,
#      redirects included;
#   3. the tarball must match its line in SHA256SUMS, else nothing is
#      installed;
#   4. only the lazygram binary is extracted; it must report the version
#      it was downloaded as, then it is moved into place.
#
# No sudo, nothing written outside the prefix and a temporary directory
# that is removed at exit. POSIX sh; read it before running it.
set -eu

REPO="t0d0r/lazygram"
PREFIX="${LAZYGRAM_PREFIX:-$HOME/.local}"
BIN="$PREFIX/bin"

die() {
  printf 'install.sh: %s\n' "$*" >&2
  exit 1
}

# fetch downloads over HTTPS only; -f fails on HTTP errors, -L follows the
# release asset redirect, which must stay on HTTPS too.
fetch() {
  curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 "$@"
}

for tool in curl tar uname mktemp awk; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required"
done
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  die "sha256sum or shasum is required"
fi

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) die "unsupported system '$(uname -s)': releases exist for Linux and macOS" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) die "unsupported architecture '$(uname -m)': releases exist for amd64 and arm64" ;;
esac

VERSION="${LAZYGRAM_VERSION:-}"
if [ -z "$VERSION" ]; then
  # The latest release, from the redirect GitHub answers for releases/latest
  # (a HEAD request, not followed). No answer, or one that points anywhere
  # but this repository's release tags, stops here.
  LOCATION="$(curl -fsSI --proto '=https' --tlsv1.2 "https://github.com/$REPO/releases/latest" 2>/dev/null |
    tr -d '\r' | awk 'tolower($1) == "location:" { print $2 }' | tail -n 1)"
  case "$LOCATION" in
    "https://github.com/$REPO/releases/tag/"*) VERSION="${LOCATION##*/}" ;;
    "") die "cannot find the latest release: no answer from GitHub" ;;
    *) die "GitHub did not name a latest release of $REPO (got '$LOCATION')" ;;
  esac
fi
printf '%s\n' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
  die "a release version looks like v0.4.19, got '$VERSION'"

NAME="lazygram-$VERSION-$OS-$ARCH"
BASE="https://github.com/$REPO/releases/download/$VERSION"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
trap 'exit 130' INT TERM

printf 'lazygram %s for %s/%s\n' "$VERSION" "$OS" "$ARCH"
fetch -o "$TMP/$NAME.tar.gz" "$BASE/$NAME.tar.gz" || die "download failed: $BASE/$NAME.tar.gz"
fetch -o "$TMP/SHA256SUMS" "$BASE/SHA256SUMS" || die "download failed: $BASE/SHA256SUMS"

EXPECTED="$(awk -v f="$NAME.tar.gz" '$2 == f || $2 == "*" f { print $1; exit }' "$TMP/SHA256SUMS")"
[ -n "$EXPECTED" ] || die "$NAME.tar.gz is not listed in the release's SHA256SUMS"
ACTUAL="$(sha256 "$TMP/$NAME.tar.gz")"
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum mismatch for $NAME.tar.gz: refusing to install"
printf 'checksum ok  %s\n' "$ACTUAL"

tar -xzf "$TMP/$NAME.tar.gz" -C "$TMP" "$NAME/lazygram" || die "the archive holds no $NAME/lazygram"
[ -f "$TMP/$NAME/lazygram" ] && [ ! -L "$TMP/$NAME/lazygram" ] || die "the archive's lazygram is not a regular file"
chmod 0755 "$TMP/$NAME/lazygram"
GOT="$("$TMP/$NAME/lazygram" -version 2>/dev/null || true)"
[ "$GOT" = "lazygram $VERSION" ] || die "the binary reports '$GOT', not lazygram $VERSION: refusing to install"

mkdir -p "$BIN"
cp "$TMP/$NAME/lazygram" "$BIN/.lazygram-install.$$"
chmod 0755 "$BIN/.lazygram-install.$$"
mv -f "$BIN/.lazygram-install.$$" "$BIN/lazygram" # a rename: a running lazygram keeps its file
printf 'installed %s (%s)\n' "$BIN/lazygram" "$GOT"

case ":$PATH:" in
  *":$BIN:"*) ;;
  *) printf 'note: %s is not in your PATH\n' "$BIN" ;;
esac
printf 'next: lazygram login (QR code), or lazygram demo\n'

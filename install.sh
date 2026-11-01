#!/usr/bin/env bash
# lazygram installer: downloads one pinned release from GitHub, checks it
# against the release's SHA256SUMS and puts the binary in $PREFIX/bin
# (default ~/.local/bin). No root, no shell tricks, nothing else touched.
#
#   LAZYGRAM_VERSION=v0.4.18 LAZYGRAM_PREFIX=$HOME/.local bash install.sh
#
# Read this file before running it; it is short on purpose.
set -euo pipefail

VERSION="${LAZYGRAM_VERSION:-v0.4.18}"
PREFIX="${LAZYGRAM_PREFIX:-$HOME/.local}"
REPO="t0d0r/lazygram"
BASE="https://github.com/$REPO/releases/download/$VERSION"

die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) die "LAZYGRAM_VERSION must look like v0.4.18, got '$VERSION'" ;;
esac

for tool in curl tar; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required"
done
if command -v sha256sum >/dev/null 2>&1; then
  SUM="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SUM="shasum -a 256"
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

NAME="lazygram-$VERSION-$OS-$ARCH"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

printf 'lazygram %s for %s/%s\n' "$VERSION" "$OS" "$ARCH"
curl -fsSL --proto '=https' --tlsv1.2 -o "$TMP/$NAME.tar.gz" "$BASE/$NAME.tar.gz" \
  || die "download failed: $BASE/$NAME.tar.gz"
curl -fsSL --proto '=https' --tlsv1.2 -o "$TMP/SHA256SUMS" "$BASE/SHA256SUMS" \
  || die "download failed: $BASE/SHA256SUMS"

# The checksum line of this one file, checked by the tool at hand.
EXPECTED="$(grep -E " \*?$NAME\.tar\.gz\$" "$TMP/SHA256SUMS" | head -n 1 | cut -c1-64)"
[ -n "$EXPECTED" ] || die "$NAME.tar.gz is not listed in SHA256SUMS"
ACTUAL="$(cd "$TMP" && $SUM "$NAME.tar.gz" | cut -c1-64)"
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum mismatch for $NAME.tar.gz: refusing to install"
printf 'checksum ok  %s\n' "$EXPECTED"

tar -xzf "$TMP/$NAME.tar.gz" -C "$TMP" "$NAME/lazygram"
[ -f "$TMP/$NAME/lazygram" ] || die "the archive holds no lazygram binary"

mkdir -p "$PREFIX/bin"
install -m 0755 "$TMP/$NAME/lazygram" "$PREFIX/bin/lazygram"
printf 'installed %s\n' "$PREFIX/bin/lazygram"
"$PREFIX/bin/lazygram" -version

case ":$PATH:" in
  *":$PREFIX/bin:"*) ;;
  *) printf 'note: %s/bin is not in your PATH\n' "$PREFIX" ;;
esac
printf 'next: lazygram login (QR code), or lazygram demo\n'

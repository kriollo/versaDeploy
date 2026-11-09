#!/bin/sh
# Installs versa from the GitHub releases, verified against the release's sha256.
#
#   curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh
#
# VERSA_VERSION=v1.15.0 picks a release (default: the latest).
# VERSA_INSTALL_DIR=/opt/bin picks the folder (default: /usr/local/bin when writable
# or run as root, else ~/.local/bin).
set -eu

REPO="kriollo/versaDeploy"
VERSION="${VERSA_VERSION:-latest}"

die() {
	printf 'versa install: %s\n' "$*" >&2
	exit 1
}

case "$(uname -s)" in
Linux) OS=linux ;;
Darwin) OS=darwin ;;
*) die "unsupported OS $(uname -s); on Windows download versa_windows_amd64.exe from https://github.com/$REPO/releases" ;;
esac
case "$(uname -m)" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) die "no build for $(uname -m)" ;;
esac
command -v curl >/dev/null 2>&1 || die "curl is required"
if command -v sha256sum >/dev/null 2>&1; then
	SHA="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	SHA="shasum -a 256"
else
	die "sha256sum or shasum is required to verify the download"
fi

if [ "$VERSION" = latest ]; then
	# the latest release's URL ends in its tag (no API call, no rate limit)
	VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
	VERSION=${VERSION##*/}
fi
printf '%s' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+' || die "invalid version: $VERSION"

NAME="versa_${OS}_${ARCH}"
URL="https://github.com/$REPO/releases/download/$VERSION/$NAME"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading versa $VERSION ($OS/$ARCH)..."
curl -fsSL -o "$TMP/versa" "$URL" || die "download failed: $URL"
curl -fsSL -o "$TMP/versa.sha256" "$URL.sha256" || die "checksum download failed: $URL.sha256"
WANT=$(cut -d' ' -f1 <"$TMP/versa.sha256")
GOT=$($SHA "$TMP/versa" | cut -d' ' -f1)
[ -n "$WANT" ] && [ "$WANT" = "$GOT" ] || die "checksum mismatch (want $WANT, got $GOT): not installed"
chmod 755 "$TMP/versa"

DIR="${VERSA_INSTALL_DIR:-}"
if [ -z "$DIR" ]; then
	if [ "$(id -u)" = 0 ] || [ -w /usr/local/bin ]; then
		DIR=/usr/local/bin
	else
		DIR="$HOME/.local/bin"
	fi
fi
mkdir -p "$DIR"
mv "$TMP/versa" "$DIR/versa"
echo "Installed $DIR/versa ($("$DIR/versa" version 2>/dev/null | head -n 1))"
case ":$PATH:" in
*":$DIR:"*) ;;
*) echo "Add $DIR to your PATH, e.g.: echo 'export PATH=\"$DIR:\$PATH\"' >> ~/.profile" ;;
esac

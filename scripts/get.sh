#!/bin/sh
# Downloads the latest ask release and installs it for the current user.
#   curl -fsSL https://raw.githubusercontent.com/OrionWambert/ask-golang/main/scripts/get.sh | sh
#   BIN_DIR=/opt/bin sh get.sh
set -eu

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
base="https://github.com/OrionWambert/ask-golang/releases/latest/download"

case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) echo "Unsupported system: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

archive="ask_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/$archive" "$base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"

expected="$(awk -v name="$archive" '$2 == name { print $1 }' "$tmp/checksums.txt")"
if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$tmp/$archive" | awk '{ print $1 }')"
else
    actual="$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')"
fi
if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
    echo "Checksum mismatch for $archive" >&2
    exit 1
fi

tar -xzf "$tmp/$archive" -C "$tmp" ask
mkdir -p "$BIN_DIR"
install -m 755 "$tmp/ask" "$BIN_DIR/ask"
echo "Installed $("$BIN_DIR/ask" version) to $BIN_DIR/ask"

case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo "Add this line to your shell profile to use ask: export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

if ! command -v claude >/dev/null 2>&1; then
    echo "Warning: the claude binary is not on your PATH: install Claude Code before using ask." >&2
fi

#!/bin/sh
# Builds ask from source and installs it for the current user.
#   ./scripts/install.sh
#   BIN_DIR=/opt/bin ./scripts/install.sh
set -eu

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
root="$(cd "$(dirname "$0")/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
    echo "Go is required to build ask: https://go.dev/dl/" >&2
    exit 1
fi

cd "$root"
version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
target="$BIN_DIR/ask$(go env GOEXE)"

mkdir -p "$BIN_DIR"
go build -ldflags "-X oriowambert.com/ask-golang/cmd.version=$version" -o "$target" .
echo "Installed ask $version to $target"

case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo "Add this line to your shell profile to use ask: export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

if ! command -v claude >/dev/null 2>&1; then
    echo "Warning: the claude binary is not on your PATH: install Claude Code before using ask." >&2
fi

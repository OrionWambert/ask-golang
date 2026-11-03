#!/bin/sh
# Removes ask. Saved config and history are kept.
set -eu

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

for target in "$BIN_DIR/ask" "$BIN_DIR/ask.exe"; do
    if [ -f "$target" ]; then
        rm -f "$target"
        echo "Removed $target"
    fi
done

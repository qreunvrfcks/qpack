#!/bin/sh
# qpack installer: puts binary, .desktop and icon into ~/.local
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/qpack"
[ -f "$BIN" ] || { echo "qpack binary not found next to assets/ (run from extracted release dir)"; exit 1; }

mkdir -p "$HOME/.local/bin" "$HOME/.local/share/applications" "$HOME/.local/share/icons/hicolor/256x256/apps"
# purge previous install (binary, desktop, icons incl. other sizes)
rm -f "$HOME/.local/bin/qpack"
rm -f "$HOME/.local/share/applications/qpack.desktop"
rm -f "$HOME"/.local/share/icons/hicolor/*/apps/qpack.png "$HOME"/.local/share/icons/hicolor/*/apps/qpack.ico 2>/dev/null || true
cp "$BIN" "$HOME/.local/bin/qpack"
chmod +x "$HOME/.local/bin/qpack"
cp "$ROOT/assets/qpack.desktop" "$HOME/.local/share/applications/qpack.desktop"
cp "$ROOT/assets/icon.png" "$HOME/.local/share/icons/hicolor/256x256/apps/qpack.png"
update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true
gtk-update-icon-cache -f "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
echo "installed: ~/.local/bin/qpack"

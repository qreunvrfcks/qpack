#!/bin/sh
# qpack .run payload entrypoint (executed by makeself after self-extraction).
# Working dir = extracted payload: qpack binary + assets/.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
BIN="$HERE/qpack"
[ -f "$BIN" ] || { echo "qpack binary missing in payload"; exit 1; }

mkdir -p "$HOME/.local/bin" "$HOME/.local/share/applications" "$HOME/.local/share/icons/hicolor/256x256/apps"
# purge previous install (binary, desktop, icons incl. other sizes)
rm -f "$HOME/.local/bin/qpack"
rm -f "$HOME/.local/share/applications/qpack.desktop"
rm -f "$HOME"/.local/share/icons/hicolor/*/apps/qpack.png "$HOME"/.local/share/icons/hicolor/*/apps/qpack.ico 2>/dev/null || true
cp "$BIN" "$HOME/.local/bin/qpack"
chmod +x "$HOME/.local/bin/qpack"
cp "$HERE/assets/qpack.desktop" "$HOME/.local/share/applications/qpack.desktop"
cp "$HERE/assets/icon.png" "$HOME/.local/share/icons/hicolor/256x256/apps/qpack.png"
update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true
gtk-update-icon-cache -f "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
echo "qpack installed to ~/.local/bin/qpack"

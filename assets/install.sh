#!/bin/sh
# qpack installer: puts binary, .desktop and icon into ~/.local
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/qpack"
[ -f "$BIN" ] || { echo "qpack binary not found next to assets/ (run from extracted release dir)"; exit 1; }

ICONDIR="$HOME/.local/share/icons/hicolor"
mkdir -p "$HOME/.local/bin" "$HOME/.local/share/applications" "$ICONDIR/256x256/apps"
# purge previous install (binary, desktop, icons incl. other sizes)
rm -f "$HOME/.local/bin/qpack"
rm -f "$HOME/.local/share/applications/qpack.desktop"
rm -f "$ICONDIR"/*/apps/qpack.png "$ICONDIR"/*/apps/qpack.ico 2>/dev/null || true
cp "$BIN" "$HOME/.local/bin/qpack"
chmod +x "$HOME/.local/bin/qpack"
cp "$ROOT/assets/qpack.desktop" "$HOME/.local/share/applications/qpack.desktop"
# icon must match its 256x256 slot; resize if ImageMagick exists
if command -v magick >/dev/null 2>&1; then
  magick "$ROOT/assets/icon.png" -resize 256x256 "$ICONDIR/256x256/apps/qpack.png"
elif command -v convert >/dev/null 2>&1; then
  convert "$ROOT/assets/icon.png" -resize 256x256 "$ICONDIR/256x256/apps/qpack.png"
else
  cp "$ROOT/assets/icon.png" "$ICONDIR/256x256/apps/qpack.png"
fi
# minimal theme index so icon loaders accept ~/.local hicolor
if [ ! -f "$ICONDIR/index.theme" ]; then
  printf '[Icon Theme]\nName=Hicolor\nComment=Fallback icon theme\nDirectories=256x256/apps\n\n[256x256/apps]\nSize=256\nContext=Applications\nType=Threshold\n' > "$ICONDIR/index.theme"
fi
update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true
gtk-update-icon-cache -f "$ICONDIR" 2>/dev/null || true
if command -v kbuildsycoca6 >/dev/null 2>&1; then
  kbuildsycoca6 --noincremental 2>/dev/null || true
elif command -v kbuildsycoca5 >/dev/null 2>&1; then
  kbuildsycoca5 --noincremental 2>/dev/null || true
fi
echo "installed: ~/.local/bin/qpack"

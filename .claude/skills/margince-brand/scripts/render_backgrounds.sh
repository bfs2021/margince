#!/usr/bin/env bash
# Renders assets/backgrounds.html into the images the .pptx and .docx helpers
# use. Needs a Chromium binary (CHROME, default "chromium") and ImageMagick.
set -euo pipefail

assets="$(cd "$(dirname "$0")/../assets" && pwd)"
chrome="${CHROME:-chromium}"
work="$(mktemp -d)"
trap 'rm -r "$work"' EXIT

# name, CSS width, CSS height, output file
pieces=(
  "hero 1280 720 slide-hero.jpg"
  "paper 1280 720 slide-paper.jpg"
  "band 1240 64 slide-band.png"
  "panel 400 680 slide-panel.png"
  "docband 733 249 doc-band.png"
)

for piece in "${pieces[@]}"; do
  read -r name width height out <<<"$piece"
  "$chrome" --headless --no-sandbox --disable-gpu --hide-scrollbars \
    --force-device-scale-factor=2 --default-background-color=00000000 \
    --window-size="$width,$((height + 200))" --screenshot="$work/$name.png" \
    "file://$assets/backgrounds.html#$name" 2>/dev/null
  # Headless Chromium's viewport is shorter than its window, so the window
  # is taller than the piece and the crop takes the piece alone.
  crop="$((width * 2))x$((height * 2))+0+0"
  case "$out" in
    *.jpg) convert "$work/$name.png" -crop "$crop" +repage -background '#f1f5f2' -flatten -quality 88 "$assets/$out" ;;
    *) convert "$work/$name.png" -crop "$crop" +repage "$assets/$out" ;;
  esac
  echo "$out"
done

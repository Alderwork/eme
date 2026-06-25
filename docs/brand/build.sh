#!/usr/bin/env bash
# Regenerate eme's vector logos + favicon set from the Fraunces source.
# Deterministic and offline-after-fetch: pulls the exact Google Fonts instance
# (opsz 144, wght 600, WONK 1), bakes SOFT 60, outlines glyphs to SVG paths,
# then rasterizes the favicon set + multi-res .ico.
#
# Requires: python3, curl, rsvg-convert, ImageMagick (magick). No system installs —
# fontTools + brotli go into a throwaway venv under .build/.
set -euo pipefail
cd "$(dirname "$0")"
BUILD=.build
UA='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36'
CSS='https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght,SOFT,WONK@144,600,60,1&display=swap'

mkdir -p "$BUILD" logo favicon
python3 -m venv "$BUILD/venv"
"$BUILD/venv/bin/pip" install --quiet "fonttools>=4.0" brotli

curl -sS -A "$UA" "$CSS" -o "$BUILD/fraunces.css"
LATIN_URL=$(grep -A6 '/\* latin \*/' "$BUILD/fraunces.css" | grep -oE 'https://[^)]+\.woff2' | head -1)
curl -sS -A "$UA" "$LATIN_URL" -o "$BUILD/fraunces-latin.woff2"
"$BUILD/venv/bin/python" -c "from fontTools.ttLib.woff2 import decompress; decompress('$BUILD/fraunces-latin.woff2','$BUILD/fraunces-latin.ttf')"
# Google bakes opsz/wght/WONK; bake the remaining SOFT=60 to fully flatten.
"$BUILD/venv/bin/fonttools" varLib.instancer "$BUILD/fraunces-latin.ttf" SOFT=60 -o "$BUILD/fraunces-static.ttf"

# Outline wordmark + monogram + favicon sources (writes *.svg into CWD).
( cd "$BUILD" && cp ../gen_svg.py . && ./venv/bin/python gen_svg.py )
mv "$BUILD"/eme-wordmark.svg "$BUILD"/eme-wordmark-amber.svg "$BUILD"/eme-mark.svg "$BUILD"/eme-mark-amber.svg logo/
cp "$BUILD"/favicon.svg "$BUILD"/favicon-maskable.svg favicon/

# Rasterize favicon set + multi-res .ico.
for s in 16 32 48 180 192 512; do rsvg-convert -w $s -h $s favicon/favicon.svg -o favicon/icon-$s.png; done
rsvg-convert -w 512 -h 512 favicon/favicon-maskable.svg -o favicon/maskable-512.png
cp favicon/icon-180.png favicon/apple-touch-icon.png
magick favicon/icon-16.png favicon/icon-32.png favicon/icon-48.png favicon/favicon.ico

# Render hero + OG cards (light + dark) via headless Chrome at 2x.
CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
if [ -x "$CHROME" ]; then
  shot(){ "$CHROME" --headless=new --disable-gpu --hide-scrollbars \
    --force-device-scale-factor=2 --window-size="$2" \
    --default-background-color=00000000 --screenshot="$3" "file://$PWD/$1" >/dev/null 2>&1; }
  shot hero.html        1200,420 hero.png
  shot hero-dark.html   1200,420 hero-dark.png
  shot og-card.html     1200,630 og-card.png
  shot og-card-dark.html 1200,630 og-card-dark.png
  echo "rendered hero/og (light + dark)"
else
  echo "skip hero/og render: Chrome not found (set CHROME=/path/to/chrome)"
fi

echo "brand assets regenerated: logo/ favicon/ hero* og-card*"

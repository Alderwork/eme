# eme brand kit

Vector logos, favicons, and design tokens for **eme**. The single source of truth for
the brand *system* (color, type, voice, usage rules) is [`../../DESIGN.md`](../../DESIGN.md);
this folder holds the produced assets and the script that regenerates them.

## Contents

```
logo/
  eme-wordmark.svg        # "eme" wordmark, charcoal ink — for light backgrounds
  eme-wordmark-amber.svg  # amber ink — the documented inversion, for charcoal/dark
  eme-mark.svg            # "e" monogram, charcoal ink
  eme-mark-amber.svg      # "e" monogram, amber ink
favicon/
  favicon.svg             # "e" on the amber field (source)
  favicon.ico             # multi-res 16/32/48
  icon-16/32/48/180/192/512.png
  apple-touch-icon.png    # 180×180
  favicon-maskable.svg / maskable-512.png  # extra safe-zone padding for PWA masks
  site.webmanifest
tokens.json / tokens.css  # design tokens, generated from DESIGN.md
hero.* / og-card.*        # render sources + PNGs (light + -dark amber-on-charcoal)
favicon.html              # original favicon render source
gen_svg.py / build.sh     # regeneration pipeline
```

Both a light (charcoal-on-amber) and a dark (`-dark`, amber-on-charcoal) variant of the
hero banner and OG card are produced.

The logos are **true vector outlines** (glyph paths, no `<text>`, no embedded font), so they
render identically everywhere with zero font dependency.

## Usage

- **Wordmark on amber** (`#E8A21A`) or any light surface → `logo/eme-wordmark.svg`.
- **Wordmark on charcoal** (`#121317`) or dark surface → `logo/eme-wordmark-amber.svg`.
- The fill is the only difference; both are otherwise identical paths.
- Clear space ≥ the cap-height of the `e`; minimum wordmark width 96px, favicon `e` ≥ 16px
  (see DESIGN.md → Brand). Don't recolor, outline, skew, or add effects.

## Regenerate

```bash
./build.sh
```

Pulls the exact Fraunces instance (`opsz 144, wght 600, SOFT 60, WONK 1`) from Google Fonts,
outlines the glyphs to SVG paths, and rasterizes the favicon set + `.ico`. Needs `python3`,
`curl`, `rsvg-convert`, and ImageMagick (`magick`); `fontTools` + `brotli` are installed into
a throwaway `.build/venv` (no system changes).

## Fonts & licensing

[Fraunces](https://github.com/googlefonts/fraunces) is licensed under the SIL Open Font
License 1.1. We ship **outlines** (the logos are paths, not the font), so no font file is
redistributed here. If you embed Fraunces directly (e.g. self-hosting it on a site), include
its OFL license alongside the font file.

#!/usr/bin/env python3
"""Outline the eme wordmark + monogram from the static Fraunces instance to SVG paths.

Emits dependency-free vector logos (no <text>, no embedded font) plus favicon sources.
"""
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.transformPen import TransformPen

FONT = "fraunces-static.ttf"
INK = "#121317"      # charcoal-900
AMBER = "#E8A21A"    # brand-field amber (gold-500)
TRACK = -12          # letter-spacing -1px @166px in 2000upm units
PAD = 48

f = TTFont(FONT)
gs = f.getGlyphSet()
cmap = f.getBestCmap()


def layout(text):
    """Return (placed paths with x offsets, tight bounds) in font units (y-up)."""
    placed = []
    bounds = BoundsPen(gs)
    x = 0
    for i, ch in enumerate(text):
        g = gs[cmap[ord(ch)]]
        spen = SVGPathPen(gs)
        g.draw(spen)
        placed.append((spen.getCommands(), x))
        g.draw(TransformPen(bounds, (1, 0, 0, 1, x, 0)))
        x += g.width + (TRACK if i < len(text) - 1 else 0)
    return placed, bounds.bounds


def wordmark(text, out, fill=INK):
    placed, (xMin, yMin, xMax, yMax) = layout(text)
    W, H = (xMax - xMin) + 2 * PAD, (yMax - yMin) + 2 * PAD
    tx, ty = -xMin + PAD, yMax + PAD
    paths = "\n    ".join(f'<path transform="translate({ox} 0)" d="{d}"/>' for d, ox in placed)
    with open(out, "w") as fh:
        fh.write(
            f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W:.0f} {H:.0f}" role="img" aria-label="{text}">\n'
            f'  <g transform="translate({tx:.1f} {ty:.1f}) scale(1 -1)" fill="{fill}">\n    {paths}\n  </g>\n</svg>\n'
        )
    print(f"{out}: {W:.0f}x{H:.0f}")


def favicon(out, tile=512, frac=0.62, fill=INK, bg=AMBER, radius=0):
    """'e' monogram optically centered on a tile. frac = glyph height / tile."""
    placed, (xMin, yMin, xMax, yMax) = layout("e")
    gw, gh = xMax - xMin, yMax - yMin
    scale = (tile * frac) / gh
    sw, sh = gw * scale, gh * scale
    ox = (tile - sw) / 2 - xMin * scale
    oy = (tile + sh) / 2 + yMin * scale  # baseline placement, flip handled below
    d = placed[0][0]
    rect = (f'<rect width="{tile}" height="{tile}" rx="{radius}" fill="{bg}"/>\n  ' if bg else "")
    with open(out, "w") as fh:
        fh.write(
            f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {tile} {tile}" role="img" aria-label="eme">\n  '
            f'{rect}<g transform="translate({ox:.1f} {oy:.1f}) scale({scale:.5f} {-scale:.5f})" fill="{fill}">\n    <path d="{d}"/>\n  </g>\n</svg>\n'
        )
    print(f"{out}: {tile}px tile, glyph {frac:.0%}")


# logos — transparent, fill-swappable
wordmark("eme", "eme-wordmark.svg", fill=INK)            # default (ink on light)
wordmark("eme", "eme-wordmark-amber.svg", fill=AMBER)    # inversion (amber on charcoal)
wordmark("e", "eme-mark.svg", fill=INK)
wordmark("e", "eme-mark-amber.svg", fill=AMBER)
# favicons — e on amber tile
favicon("favicon.svg", tile=512, frac=0.62)                       # standard
favicon("favicon-maskable.svg", tile=512, frac=0.48)              # extra safe-zone padding

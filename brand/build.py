#!/usr/bin/env python3
"""
Builds the Nook logo.

Same idea as the Orcrist mark: the first letter carries the six palette colours
as stacked layers, each set back from the one in front along a single direction
(up and to the left, at 200 degrees); the remaining letters are the violet of
the front layer. Letters are emitted as outlines, not <text>, so the logo
renders the same everywhere.

Run from anywhere:  python3 brand/build.py
Needs fontTools and Liberation Sans Bold installed (fc-match must find it).
The PNG is rendered from the SVG by render.py (Playwright + Chromium).
"""

import math
import subprocess
from pathlib import Path

from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.ttLib import TTFont

ROOT = Path(__file__).resolve().parent

# One colour per palette, back to front. Violet leads, as in Orcrist.
LAYERS = ["#fbba16", "#9f3011", "#a0151b", "#1e4380", "#00492c", "#52389c"]
VIOLET, WHITE = "#52389c", "#ffffff"
STEP = 0.086   # set-back between layers, as a share of the cap height
ANGLE = 200    # direction of the set-back, in degrees

FONT_FILE = subprocess.check_output(
    ["fc-match", "-f", "%{file}", "Liberation Sans:style=Bold"]
).decode()
FONT = TTFont(FONT_FILE)
GLYPHS = FONT.getGlyphSet()
CMAP = FONT.getBestCmap()
CAP = FONT["OS/2"].sCapHeight


def glyph(ch):
    gn = CMAP[ord(ch)]
    pen = SVGPathPen(GLYPHS)
    GLYPHS[gn].draw(pen)
    bounds = BoundsPen(GLYPHS)
    GLYPHS[gn].draw(bounds)
    return pen.getCommands(), FONT["hmtx"][gn][0], bounds.bounds


def wordmark(cap_px=200, bg=None, tracking=0.0, bolden=0.035) -> str:
    k = cap_px / CAP
    pad = cap_px * 0.30
    rad = math.radians(ANGLE)
    s = STEP * cap_px
    dx, dy = math.cos(rad) * s, math.sin(rad) * s
    n = len(LAYERS)
    tail = -dx * (n - 1)          # how far the back layer reaches left
    top = -dy * (n - 1)           # ... and above the cap line (SVG y grows downwards)
    ox, oy = pad + tail, pad + top

    # faux-bold: shrink by delta, then stroke by delta (see Orcrist's build.py)
    delta = bolden * cap_px
    kb = k * (cap_px - delta) / cap_px
    sw = delta / kb

    def layer(ch, colour, x, y):
        d = glyph(ch)[0]
        return (
            f'<path d="{d}" fill="{colour}" stroke="{colour}" stroke-width="{sw:.1f}" '
            f'stroke-linejoin="round" transform="translate({x:.2f} {y:.2f}) '
            f'scale({kb:.5f} -{kb:.5f})"/>'
        )

    base_y = oy + cap_px - delta / 2
    out = []
    for i, colour in enumerate(LAYERS):           # back to front
        back = n - 1 - i
        out.append(layer("N", colour, ox + dx * back, base_y + dy * back))
    pen = glyph("N")[1] + tracking * CAP
    last_right = 0
    rest = []
    for ch in "OOK":
        d, adv, bb = glyph(ch)
        rest.append(f'<path d="{d}" transform="translate({pen:.1f} 0)"/>')
        last_right = pen + bb[2]
        pen += adv + tracking * CAP
    width = round(ox + last_right * kb + pad)
    height = round(oy + cap_px + pad * 0.55)
    joined = "\n    ".join(rest)
    bgrect = f'  <rect width="{width}" height="{height}" fill="{bg}"/>\n' if bg else ""
    layers = "\n  ".join(out)
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}" width="{width}" height="{height}" role="img" aria-label="Nook">
  <title>Nook</title>
{bgrect}  {layers}
  <g fill="{VIOLET}" stroke="{VIOLET}" stroke-width="{sw:.1f}" stroke-linejoin="round" transform="translate({ox:.2f} {base_y:.2f}) scale({kb:.5f} -{kb:.5f})">
    {joined}
  </g>
</svg>
"""


if __name__ == "__main__":
    (ROOT / "nook-logo.svg").write_text(wordmark(bg=WHITE))
    (ROOT / "nook-lockup.svg").write_text(wordmark(bg=None))
    print("wrote the SVGs")

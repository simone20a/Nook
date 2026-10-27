#!/usr/bin/env python3
"""Renders brand/nook-logo.svg to PNG with Playwright + Chromium."""
import re
from pathlib import Path

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent


def render(svg_name, png_name, scale=1):
    svg = (ROOT / svg_name).read_text()
    w, h = (int(x) for x in re.search(r'viewBox="0 0 (\d+) (\d+)"', svg).groups())
    with sync_playwright() as p:
        b = p.chromium.launch()
        pg = b.new_page(viewport={"width": w + 40, "height": h + 40}, device_scale_factor=scale)
        pg.set_content(f'<body style="margin:0;background:transparent">{svg}</body>')
        pg.locator("svg").screenshot(path=str(ROOT / png_name), omit_background=True)
        b.close()


if __name__ == "__main__":
    render("nook-logo.svg", "nook-logo.png")

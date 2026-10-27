"""產生 brand/hawkr 的 SVG。

字標是 Nunito（SIL OFL 1.1）ExtraBold（wght 800）以 HarfBuzz 排字（含 kerning）後轉成外框，不依賴字型。
需要 fonttools 與 uharfbuzz，以及 Nunito 的 variable font（google/fonts 的 ofl/nunito/Nunito[wght].ttf）：

    python3 build.py <Nunito[wght].ttf>
"""

import io
import os
import sys

import uharfbuzz as hb
from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

OUT = os.path.dirname(os.path.abspath(__file__))
VER, AMB, INK, LIGHT, WHITE = "#E4572E", "#F2A541", "#1F2933", "#F5F7FA", "#FFFFFF"

# 圖形：遮雨棚（四個扇形、兩色條紋）加兩根柱子與檯面組成 H。座標以 120×120 為格。
MARK = (
    '<defs><clipPath id="awning"><path d="M28 24 H92 L102 46 a10.5 10.5 0 0 1 -21 0 a10.5 10.5 0 0 1 -21 0 '
    'a10.5 10.5 0 0 1 -21 0 a10.5 10.5 0 0 1 -21 0 Z"/></clipPath></defs>'
    '<g clip-path="url(#awning)"><rect width="120" height="70" fill="{a1}"/>'
    '<rect x="39" width="21" height="70" fill="{a2}"/><rect x="81" width="21" height="70" fill="{a2}"/></g>'
    '<g fill="none" stroke="{ink}" stroke-width="14" stroke-linecap="round">'
    '<path d="M36 67 V101"/><path d="M84 67 V101"/><path d="M36 83 H84"/></g>'
)

# 小尺寸（16–48px）專用：不畫條紋，遮雨棚三個扇形，柱子加粗。座標以 32×32 為格。
FAVICON = (
    f'<rect width="32" height="32" rx="7" fill="{VER}"/>'
    f'<path fill="{WHITE}" d="M8.5 5.5 H23.5 L27 12 a3.667 3.667 0 0 1 -7.333 0 a3.667 3.667 0 0 1 -7.333 0 '
    'a3.667 3.667 0 0 1 -7.333 0 Z"/>'
    f'<g fill="none" stroke="{WHITE}" stroke-width="3.6" stroke-linecap="round">'
    '<path d="M10.5 17.5 V26"/><path d="M21.5 17.5 V26"/><path d="M10.5 21.5 H21.5"/></g>'
)


def mark(ink, a1=VER, a2=AMB):
    return MARK.format(a1=a1, a2=a2, ink=ink)


def wordmark_path(font_path, text="Hawkr", wght=800):
    f = instantiateVariableFont(TTFont(font_path), {"wght": wght})
    buf = io.BytesIO()
    f.save(buf)
    font = hb.Font(hb.Face(buf.getvalue()))
    b = hb.Buffer()
    b.add_str(text)
    b.guess_segment_properties()
    hb.shape(font, b, {"kern": True, "liga": False})
    gs, order = f.getGlyphSet(), f.getGlyphOrder()
    pen, bounds, x = SVGPathPen(gs), BoundsPen(gs), 0
    for info, pos in zip(b.glyph_infos, b.glyph_positions):
        t = (1, 0, 0, -1, x + pos.x_offset, -pos.y_offset)
        gs[order[info.codepoint]].draw(TransformPen(pen, t))
        gs[order[info.codepoint]].draw(TransformPen(bounds, t))
        x += pos.x_advance
    return pen.getCommands(), bounds.bounds


def write(name, view_box, body):
    with open(os.path.join(OUT, name), "w") as fh:
        fh.write(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{view_box}" role="img" aria-label="Hawkr">{body}</svg>\n')


def main(font_path):
    d, (x0, y0, x1, y1) = wordmark_path(font_path)
    word = lambda ink: f'<path fill="{ink}" d="{d}"/>'
    pad = 20
    word_box = f"{x0 - pad:g} {y0 - pad:g} {x1 - x0 + 2 * pad:g} {y1 - y0 + 2 * pad:g}"
    # 橫式組合：字標的底線對齊柱子底部（y=101），大寫高約為圖形 H 的高度。
    s, tx, ty = 0.112, 140, 101
    lock = lambda ink: f'<g>{mark(ink)}</g><g transform="translate({tx} {ty}) scale({s})">{word(ink)}</g>'
    lock_w = round(tx + x1 * s + 8)
    write("hawkr-mark.svg", "0 0 120 120", mark(INK))
    write("hawkr-mark-dark.svg", "0 0 120 120", mark(LIGHT))
    write("hawkr-wordmark.svg", word_box, word(INK))
    write("hawkr-wordmark-dark.svg", word_box, word(LIGHT))
    write("hawkr-lockup.svg", f"0 0 {lock_w} 120", lock(INK))
    write("hawkr-lockup-dark.svg", f"0 0 {lock_w} 120", lock(LIGHT))
    write("hawkr-app-icon.svg", "0 0 1024 1024",
          f'<rect width="1024" height="1024" rx="224" fill="{VER}"/>'
          f'<g transform="translate(152 140) scale(6)">{mark(WHITE, WHITE, AMB)}</g>')
    write("hawkr-favicon.svg", "0 0 32 32", FAVICON)


if __name__ == "__main__":
    main(sys.argv[1])

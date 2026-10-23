"""Margince-branded PowerPoint slides on python-pptx.

The layout mirrors templates/deck.html: positions are CSS pixels on its
1280×720 slide. Gradients come from images in assets/ (see
render_backgrounds.sh); text and shapes stay editable on top of them.
Import the helpers, or run the file to write a sample deck:

    python3 margince_pptx.py sample.pptx
"""

import math
import sys
from pathlib import Path

from lxml import etree
from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import MSO_ANCHOR, PP_ALIGN
from pptx.oxml.ns import qn
from pptx.util import Emu, Pt

ASSETS = Path(__file__).resolve().parent.parent / "assets"

EMERALD = RGBColor(0x0B, 0x7A, 0x53)
EMERALD_LIFT = RGBColor(0x16, 0xA0, 0x6C)
EMERALD_TEXT = RGBColor(0x0A, 0x6F, 0x4B)
EMERALD_BG = RGBColor(0xE8, 0xF3, 0xEE)
MINT = RGBColor(0x2B, 0xB6, 0x73)
INK = RGBColor(0x10, 0x1A, 0x15)
INK2 = RGBColor(0x33, 0x40, 0x3A)
INK3 = RGBColor(0x66, 0x73, 0x6C)
EDGE = RGBColor(0xE3, 0xE7, 0xE4)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

# PowerPoint swaps in a system face when these are not installed on the viewing machine.
HEADING_FONT = "Outfit"
BODY_FONT = "Geist"
LABEL_FONT = "Geist Mono"

GUTTER = 84


def px(n):
    """One CSS pixel of the 1280px-wide HTML slide, on a 13.333in slide."""
    return Emu(int(n * 9525))


def css_pt(n):
    return Pt(n * 0.75)


def new_deck():
    prs = Presentation()
    prs.slide_width = px(1280)
    prs.slide_height = px(720)
    return prs


def _flat(shape):
    # The default theme style adds a drop shadow that LibreOffice draws even
    # when the shape's own effect list is empty, so the style reference goes.
    style = shape._element.find(qn("p:style"))
    if style is not None:
        shape._element.remove(style)
    return shape


def _fill(shape, color, alpha=None):
    shape.fill.solid()
    shape.fill.fore_color.rgb = color
    if alpha is not None:
        srgb = shape.fill._xPr.find(qn("a:solidFill")).find(qn("a:srgbClr"))
        etree.SubElement(srgb, qn("a:alpha")).set("val", str(int(alpha * 100000)))


def _shape(slide, kind, x, y, w, h, color, *, alpha=None, radius=None, edge=None):
    shape = _flat(slide.shapes.add_shape(kind, px(x), px(y), px(w), px(h)))
    _fill(shape, color, alpha)
    if edge is None:
        shape.line.fill.background()
    else:
        shape.line.color.rgb = edge
        shape.line.width = Pt(0.75)
    if radius is not None:
        shape.adjustments[0] = min(0.5, radius / min(w, h))
    return shape


def _soft_shadow(shape):
    effects = etree.SubElement(shape._element.spPr, qn("a:effectLst"))
    shadow = etree.SubElement(effects, qn("a:outerShdw"), blurRad=str(px(36)), dist=str(px(10)),
                              dir="5400000", algn="t", rotWithShape="0")
    color = etree.SubElement(shadow, qn("a:srgbClr"), val="101A15")
    etree.SubElement(color, qn("a:alpha"), val="10000")


def _picture(slide, name, x, y, w=None, h=None):
    return slide.shapes.add_picture(str(ASSETS / name), px(x), px(y),
                                    px(w) if w else None, px(h) if h else None)


def _text(slide, x, y, w, h, text, *, font=BODY_FONT, size=21, color=INK2, bold=False,
          align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP, tracking=None, line=None):
    """size is in CSS pixels, like the HTML template; tracking is in hundredths of a point."""
    box = slide.shapes.add_textbox(px(x), px(y), px(w), px(h))
    frame = box.text_frame
    frame.word_wrap = True
    frame.vertical_anchor = anchor
    frame.margin_left = frame.margin_right = frame.margin_top = frame.margin_bottom = 0
    para = frame.paragraphs[0]
    para.alignment = align
    if line:
        para.line_spacing = line
    run = para.add_run()
    run.text = text
    run.font.name = font
    run.font.size = css_pt(size)
    run.font.bold = bold
    run.font.color.rgb = color
    if tracking is not None:
        run.font._element.set("spc", str(tracking))
    return box


def _lines(text, size, width):
    # A bold Outfit glyph averages just under half an em, so this estimates how many
    # lines a heading wraps to; text boxes cannot report their own height.
    return max(1, math.ceil(len(text) * size * 0.48 / width))


def _slide(prs, ground):
    slide = prs.slides.add_slide(prs.slide_layouts[6])
    _picture(slide, ground, 0, 0, 1280, 720)
    return slide


def _chip(slide, x, y, text, *, on_dark=False):
    width = 34 + len(text) * 7.6
    if on_dark:
        _shape(slide, MSO_SHAPE.ROUNDED_RECTANGLE, x, y, width, 30, WHITE, alpha=0.12, radius=15)
    else:
        _shape(slide, MSO_SHAPE.ROUNDED_RECTANGLE, x, y, width, 30, EMERALD_BG, radius=15)
    _shape(slide, MSO_SHAPE.OVAL, x + 13, y + 11.5, 7, 7, MINT if on_dark else EMERALD)
    _text(slide, x + 27, y, width - 27, 30, text, size=14, color=WHITE if on_dark else EMERALD_TEXT,
          anchor=MSO_ANCHOR.MIDDLE)


def _band(slide, label, page):
    _picture(slide, "slide-band.png", 20, 20, 1240, 64)
    _picture(slide, "lockup-white.png", 46, 38, h=28)
    if page:
        _shape(slide, MSO_SHAPE.ROUNDED_RECTANGLE, 1162, 37, 76, 30, WHITE, alpha=0.16, radius=15)
        _text(slide, 1162, 37, 76, 30, page, font=LABEL_FONT, size=12, color=WHITE,
              align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    if label:
        _text(slide, 640, 20, 508, 64, label, size=14, color=WHITE, align=PP_ALIGN.RIGHT,
              anchor=MSO_ANCHOR.MIDDLE)


def title_slide(prs, title, subtitle=None, byline=None, eyebrow=None):
    slide = _slide(prs, "slide-hero.jpg")
    _picture(slide, "lockup-white.png", 88, 64, h=40)
    _hero_text(slide, eyebrow, title, subtitle, byline)
    return slide


def _hero_text(slide, eyebrow, title, subtitle, byline):
    bottom = 648
    if byline:
        _text(slide, 88, bottom - 18, 900, 24, byline, size=15, color=EMERALD_BG)
        bottom -= 62
    if subtitle:
        _text(slide, 88, bottom - 34, 900, 40, subtitle, size=24, color=EMERALD_BG)
        bottom -= 58
    height = _lines(title, 88, 1100) * 92
    _text(slide, 88, bottom - height, 1100, height, title, font=HEADING_FONT, size=88, color=WHITE,
          bold=True, anchor=MSO_ANCHOR.BOTTOM, tracking=-200, line=0.95)
    if eyebrow:
        _chip(slide, 88, bottom - height - 50, eyebrow, on_dark=True)


def section_slide(prs, number, title, line=None):
    slide = _slide(prs, "slide-paper.jpg")
    _picture(slide, "slide-panel.png", 20, 20, 400, 680)
    _picture(slide, "logomark-white.png", 60, 60, h=46)
    _text(slide, 516, 180, 700, 150, f"{number:02d}", font=HEADING_FONT, size=140, color=EMERALD,
          bold=True, anchor=MSO_ANCHOR.BOTTOM, tracking=-400)
    height = _lines(title, 60, 700) * 64
    _text(slide, 516, 344, 700, height, title, font=HEADING_FONT, size=60, color=INK, bold=True,
          tracking=-100, line=0.95)
    if line:
        _text(slide, 516, 344 + height + 16, 600, 60, line, size=22, color=INK3)
    return slide


def content_slide(prs, title, *, eyebrow=None, lead=None, points=None, label=None, page=None):
    """A banded slide. Custom shapes go from body_top(slide) down; the head ends there."""
    slide = _slide(prs, "slide-paper.jpg")
    _band(slide, label, page)
    y = 144
    if eyebrow:
        _chip(slide, GUTTER, y, eyebrow)
        y += 52
    height = _lines(title, 54, 1000) * 60
    _text(slide, GUTTER, y, 1000, height, title, font=HEADING_FONT, size=54, color=INK, bold=True,
          tracking=-120, line=0.95)
    y += height + 22
    if lead:
        _text(slide, GUTTER, y, 780, 36, lead, size=22, color=INK3)
        y += 52
    if points:
        y = _points(slide, y + 8, points)
    slide._margince_body_top = y
    return slide


def body_top(slide):
    return slide._margince_body_top + 32


def _points(slide, y, points):
    for i, point in enumerate(points, start=1):
        _shape(slide, MSO_SHAPE.RECTANGLE, GUTTER, y, 1000, 1, EDGE)
        _text(slide, GUTTER, y + 20, 60, 30, f"{i:02d}", font=LABEL_FONT, size=15, color=EMERALD)
        _text(slide, GUTTER + 64, y + 14, 936, 40, point, size=21)
        y += 64
    return y


def card(slide, x, y, w, h, heading, body):
    box = _shape(slide, MSO_SHAPE.ROUNDED_RECTANGLE, x, y, w, h, WHITE, radius=22, edge=EDGE)
    _soft_shadow(box)
    tile = _shape(slide, MSO_SHAPE.ROUNDED_RECTANGLE, x + 30, y + 28, 40, 40, EMERALD, radius=12)
    tile.fill.gradient()
    tile.fill.gradient_angle = 45
    stops = tile.fill.gradient_stops
    stops[0].color.rgb, stops[1].color.rgb = EMERALD_LIFT, EMERALD
    _text(slide, x + 30, y + 90, w - 60, 36, heading, font=HEADING_FONT, size=26, color=INK, bold=True,
          tracking=-50)
    _text(slide, x + 30, y + 132, w - 60, h - 150, body, size=19, color=INK3)


def stat(slide, x, y, w, value, label):
    box = _shape(slide, MSO_SHAPE.ROUNDED_RECTANGLE, x, y, w, 170, WHITE, radius=22, edge=EDGE)
    _soft_shadow(box)
    _text(slide, x + 30, y + 26, w - 60, 84, value, font=HEADING_FONT, size=76, color=EMERALD, bold=True,
          tracking=-200)
    _text(slide, x + 30, y + 118, w - 60, 30, label, size=18, color=INK3)


def closing_slide(prs, headline, line=None, eyebrow="Next step"):
    slide = _slide(prs, "slide-hero.jpg")
    _picture(slide, "lockup-white.png", 88, 64, h=40)
    _hero_text(slide, eyebrow, headline, line, None)
    return slide


def _sample(path):
    prs = new_deck()
    title_slide(prs, "Deck title in one line", "One sentence that says what the reader gets from this deck.",
                "Author name · 23 October 2026", eyebrow="Eyebrow · topic")
    section_slide(prs, 1, "Section title", "One line on what this section covers.")
    content_slide(prs, "A heading that states one idea.", eyebrow="Eyebrow label",
                  lead="A lead sentence that frames the points below.",
                  points=["First point, one sentence of detail.", "Second point, one sentence of detail.",
                          "Third point, one sentence of detail."],
                  label="Deck title", page="03 / 06")
    slide = content_slide(prs, "Two options, side by side.", eyebrow="Comparison", label="Deck title",
                          page="04 / 06")
    for i, name in enumerate(("Option A", "Option B")):
        card(slide, GUTTER + i * 568, body_top(slide), 544, 200, name, "What it is, what it costs, what it gives.")
    slide = content_slide(prs, "Three numbers the reader should keep.", eyebrow="In figures", label="Deck title",
                          page="05 / 06")
    figures = [("42", "What this figure counts"), ("3.5×", "What this figure counts"),
               ("98%", "What this figure counts")]
    for i, (value, label) in enumerate(figures):
        stat(slide, GUTTER + i * 378, body_top(slide), 354, value, label)
    closing_slide(prs, "Request a conversation", "Contact line or next step")
    prs.save(path)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: margince_pptx.py OUT.pptx")
    _sample(sys.argv[1])

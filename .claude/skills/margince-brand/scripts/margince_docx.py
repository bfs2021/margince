"""Margince-branded Word documents on python-docx.

The layout mirrors templates/document.html. Page one opens under a rounded
emerald mesh card (assets/doc-band.png, floated behind the text) with the
white lockup, the document kind, the title and a meta line. Import the
helpers, or run the file to write a sample:

    python3 margince_docx.py sample.docx
"""

import sys
from pathlib import Path

from docx import Document
from docx.enum.table import WD_ROW_HEIGHT_RULE, WD_TABLE_ALIGNMENT
from docx.enum.text import WD_ALIGN_PARAGRAPH, WD_TAB_ALIGNMENT
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Inches, Mm, Pt, RGBColor

ASSETS = Path(__file__).resolve().parent.parent / "assets"

EMERALD = RGBColor(0x0B, 0x7A, 0x53)
EMERALD_TEXT = RGBColor(0x0A, 0x6F, 0x4B)
EMERALD_BG = "E8F3EE"
MINT = RGBColor(0x8F, 0xE3, 0xB8)
PALE = RGBColor(0xC9, 0xE4, 0xD7)
WARN_BG = "FFF6E6"
WARN_DOT = RGBColor(0xF5, 0xA5, 0x24)
WARN_TEXT = RGBColor(0x8A, 0x53, 0x00)
INK = RGBColor(0x10, 0x1A, 0x15)
INK2 = RGBColor(0x33, 0x40, 0x3A)
INK3 = RGBColor(0x66, 0x73, 0x6C)
HEAD_FILL = "F6F8F7"
LINE = "E3E7E4"
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

# Word swaps in a system face when these are not installed on the viewing machine.
HEADING_FONT = "Outfit"
BODY_FONT = "Geist"
LABEL_FONT = "Geist Mono"

PAGE_W = Mm(210)
GUTTER = Mm(20)
TEXT_W = PAGE_W - 2 * GUTTER
CARD_INSET = Mm(8)
CARD_H = Mm(66)


def _shade(cell, fill):
    shd = OxmlElement("w:shd")
    shd.set(qn("w:val"), "clear")
    shd.set(qn("w:color"), "auto")
    shd.set(qn("w:fill"), fill)
    cell._tc.get_or_add_tcPr().append(shd)


def _cell_margins(cell, top=0, bottom=0, left=0, right=0):
    margins = OxmlElement("w:tcMar")
    for side, value in (("top", top), ("bottom", bottom), ("start", left), ("end", right)):
        node = OxmlElement(f"w:{side}")
        node.set(qn("w:w"), str(int(value) // 635))  # 635 EMU to the twip
        node.set(qn("w:type"), "dxa")
        margins.append(node)
    cell._tc.get_or_add_tcPr().append(margins)


def _borders(table, edges):
    """edges maps a side (top, left, bottom, right, insideH, insideV) to (hex, eighths of a point)."""
    borders = OxmlElement("w:tblBorders")
    for side in ("top", "left", "bottom", "right", "insideH", "insideV"):
        node = OxmlElement(f"w:{side}")
        spec = edges.get(side)
        if spec:
            node.set(qn("w:val"), "single")
            node.set(qn("w:color"), spec[0])
            node.set(qn("w:sz"), str(spec[1]))
        else:
            node.set(qn("w:val"), "nil")
        borders.append(node)
    table._tbl.tblPr.append(borders)


def _run(paragraph, text, *, font=BODY_FONT, size=10.5, color=INK2, bold=False):
    run = paragraph.add_run(text)
    run.font.name = font
    run.font.size = Pt(size)
    run.font.bold = bold
    run.font.color.rgb = color
    return run


def _spacing(paragraph, before=0, after=0):
    paragraph.paragraph_format.space_before = Pt(before)
    paragraph.paragraph_format.space_after = Pt(after)


def _float_behind(run, x, y):
    """Turn the run's inline picture into one anchored to the page, behind the text."""
    inline = run._r.find(".//" + qn("wp:inline"))
    anchor = OxmlElement("wp:anchor")
    for key, value in (("distT", "0"), ("distB", "0"), ("distL", "0"), ("distR", "0"), ("simplePos", "0"),
                       ("relativeHeight", "0"), ("behindDoc", "1"), ("locked", "1"), ("layoutInCell", "0"),
                       ("allowOverlap", "1")):
        anchor.set(key, value)
    simple = OxmlElement("wp:simplePos")
    simple.set("x", "0")
    simple.set("y", "0")
    anchor.append(simple)
    for axis, offset in (("H", x), ("V", y)):
        position = OxmlElement(f"wp:position{axis}")
        position.set("relativeFrom", "page")
        value = OxmlElement("wp:posOffset")
        value.text = str(int(offset))
        position.append(value)
        anchor.append(position)
    anchor.append(inline.find(qn("wp:extent")))
    effect = OxmlElement("wp:effectExtent")
    for side in "ltrb":
        effect.set(side, "0")
    anchor.append(effect)
    anchor.append(OxmlElement("wp:wrapNone"))
    for tag in ("wp:docPr", "wp:cNvGraphicFramePr", "a:graphic"):
        anchor.append(inline.find(qn(tag)))
    inline.getparent().replace(inline, anchor)


def _page_number(paragraph):
    for kind, text in (("begin", None), (None, "PAGE"), ("end", None)):
        run = paragraph.add_run()
        run.font.name = LABEL_FONT
        run.font.size = Pt(8)
        run.font.color.rgb = INK3
        if kind:
            node = OxmlElement("w:fldChar")
            node.set(qn("w:fldCharType"), kind)
        else:
            node = OxmlElement("w:instrText")
            node.set(qn("xml:space"), "preserve")
            node.text = text
        run._r.append(node)


def new_document(title, *, kind=None, meta=None):
    doc = Document()
    section = doc.sections[0]
    section.page_width, section.page_height = PAGE_W, Mm(297)
    section.left_margin = section.right_margin = GUTTER
    section.top_margin, section.bottom_margin = Mm(18), Mm(18)
    section.header_distance, section.footer_distance = Mm(0), Mm(8)
    section.different_first_page_header_footer = True

    normal = doc.styles["Normal"]
    normal.font.name = BODY_FONT
    normal.font.size = Pt(10.5)
    normal.font.color.rgb = INK2
    normal.paragraph_format.space_after = Pt(6)
    normal.paragraph_format.line_spacing = 1.25

    _first_page_card(section.first_page_header, title, kind, meta)
    for footer in (section.first_page_footer, section.footer):
        para = footer.paragraphs[0]
        tabs = para.paragraph_format.tab_stops
        # The Footer style's centre tab would catch the page number half way across.
        tabs.add_tab_stop(Inches(3.25), WD_TAB_ALIGNMENT.CLEAR)
        tabs.add_tab_stop(TEXT_W, WD_TAB_ALIGNMENT.RIGHT)
        _run(para, "Margince\t", size=8, color=INK3)
        _page_number(para)
    doc._margince_sections = 0
    return doc


def _first_page_card(header, title, kind, meta):
    table = header.add_table(rows=1, cols=1, width=TEXT_W)
    table.alignment = WD_TABLE_ALIGNMENT.LEFT
    _borders(table, {})
    row = table.rows[0]
    row.height, row.height_rule = CARD_INSET + CARD_H, WD_ROW_HEIGHT_RULE.EXACTLY
    cell = row.cells[0]
    _cell_margins(cell, top=CARD_INSET + Mm(10))
    logo = cell.paragraphs[0]
    logo.add_run().add_picture(str(ASSETS / "lockup-white.png"), height=Mm(5.5))
    _spacing(logo, after=30)
    if kind:
        para = cell.add_paragraph()
        _run(para, "●  ", size=7, color=MINT)
        _run(para, kind, size=8.5, color=PALE)
        _spacing(para, after=4)
    para = cell.add_paragraph()
    para.paragraph_format.line_spacing = 1.0
    _run(para, title, font=HEADING_FONT, size=28, color=WHITE, bold=True)
    _spacing(para, after=6)
    if meta:
        _run(cell.add_paragraph(), meta, size=9.5, color=PALE)

    # Word rejects a header whose last block is a table, so the header's own
    # paragraph stays last; it holds the card image and the gap to the body.
    closing = header.paragraphs[0]
    closing._p.addprevious(table._tbl)
    run = closing.add_run()
    run.add_picture(str(ASSETS / "doc-band.png"), width=PAGE_W - 2 * CARD_INSET)
    _float_behind(run, CARD_INSET, CARD_INSET)
    _spacing(closing, after=18)


def lead(doc, text):
    para = doc.add_paragraph()
    para.paragraph_format.line_spacing = 1.2
    _run(para, text, size=13, color=INK3)
    _spacing(para, after=6)
    return para


def heading(doc, text, level=2):
    """A level-2 heading carries its section number, in emerald, before the text."""
    para = doc.add_paragraph()
    if level == 2:
        doc._margince_sections += 1
        _run(para, f"{doc._margince_sections:02d}   ", font=LABEL_FONT, size=9, color=EMERALD)
        _run(para, text, font=HEADING_FONT, size=17, color=INK, bold=True)
        _spacing(para, before=22, after=6)
    else:
        _run(para, text, font=HEADING_FONT, size=12, color=INK, bold=True)
        _spacing(para, before=12, after=3)
    para.paragraph_format.keep_with_next = True
    return para


def paragraph(doc, text):
    para = doc.add_paragraph()
    _run(para, text)
    return para


def bullets(doc, points):
    for point in points:
        para = doc.add_paragraph(style="List Bullet")
        _run(para, point)
        _spacing(para, after=3)


def callout(doc, label, text, *, warn=False):
    table = doc.add_table(rows=1, cols=1)
    _borders(table, {})
    cell = table.rows[0].cells[0]
    _shade(cell, WARN_BG if warn else EMERALD_BG)
    _cell_margins(cell, top=Mm(3.5), bottom=Mm(3.5), left=Mm(5), right=Mm(5))
    para = cell.paragraphs[0]
    _run(para, "●  ", size=8, color=WARN_DOT if warn else EMERALD)
    _run(para, f"{label} ", color=WARN_TEXT if warn else EMERALD_TEXT, bold=True)
    _run(para, text)
    _spacing(para)
    _spacing(doc.add_paragraph(), after=2)
    return table


def table(doc, header, rows, *, numeric=()):
    """numeric names the column indexes that hold figures; they align right."""
    grid = doc.add_table(rows=1, cols=len(header))
    edge = (LINE, 4)
    _borders(grid, {"top": edge, "left": edge, "bottom": edge, "right": edge, "insideH": edge})
    for i, name in enumerate(header):
        cell = grid.rows[0].cells[i]
        _shade(cell, HEAD_FILL)
        _run(cell.paragraphs[0], name, size=8.5, color=INK3)
    for row in rows:
        cells = grid.add_row().cells
        for i, value in enumerate(row):
            _run(cells[i].paragraphs[0], str(value), size=9.5)
    for row in grid.rows:
        for i, cell in enumerate(row.cells):
            _cell_margins(cell, top=Mm(2), bottom=Mm(2), left=Mm(3.5), right=Mm(3.5))
            _spacing(cell.paragraphs[0])
            if i in numeric:
                cell.paragraphs[0].alignment = WD_ALIGN_PARAGRAPH.RIGHT
    _spacing(doc.add_paragraph(), after=4)
    return grid


def _sample(path):
    doc = new_document("Document title", kind="Document type", meta="Author name · 23 October 2026")
    lead(doc, "One or two sentences that say what this document decides or reports.")
    heading(doc, "First section")
    paragraph(doc, "Body text in Geist. Keep paragraphs short and lead with the fact.")
    bullets(doc, ["A point the reader needs.", "A second point the reader needs."])
    callout(doc, "Key point.", "The one thing to remember from this section.")
    heading(doc, "Second section")
    table(doc, ["Item", "Owner", "Amount"], [["First row", "Team", "1,200"], ["Second row", "Team", "980"]],
          numeric=(2,))
    heading(doc, "A sub-section", level=3)
    paragraph(doc, "Sub-sections use level 3 and sit tight under their section.")
    callout(doc, "Open question.", "Something that still needs a decision.", warn=True)
    doc.add_page_break()
    heading(doc, "Third section")
    paragraph(doc, "Later pages open straight on the text, with the footer below.")
    doc.save(path)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: margince_docx.py OUT.docx")
    _sample(sys.argv[1])

---
name: margince-brand
description: Style presentations, slide decks and documents (reports, one-pagers, memos, proposals, briefs) in the Margince brand, with the logo on an emerald band at the top. Use whenever the user asks for a deck, slides, a presentation or a document "in Margince style", "on brand", "with our logo", or for Margince, Gradion or a Margince customer. Covers HTML (as an artifact or printed to PDF), PowerPoint (.pptx) and Word (.docx).
---

# Margince brand for decks and documents

Every page or slide this skill makes carries the Margince lockup (the "M" mark and the word "Margince") in white
on an emerald band at the top. The colours, fonts and mark come from the product: `DESIGN.md` sections 3 and 4, and
`frontend/src/design-system/logomark.tsx`. When those change, update this skill in the same pull request.

## Pick the format

| The user asks for | Start from | Output |
|---|---|---|
| A deck, slides or a presentation, no format named | `templates/deck.html` | An HTML artifact, or a PDF |
| A document, report, memo or one-pager, no format named | `templates/document.html` | An HTML artifact, or a PDF |
| A PowerPoint or `.pptx` file | `scripts/margince_pptx.py` | `.pptx` |
| A Word or `.docx` file | `scripts/margince_docx.py` | `.docx` |
| A PDF | the matching HTML template | PDF printed by headless Chromium |

All paths are relative to this skill's folder, `.claude/skills/margince-brand/`.

## Build an HTML deck or document

1. Copy the template into the scratchpad. Never edit the template in place.
2. Replace the placeholder text. Keep the band, the lockup and the class names.
3. For a deck, copy the `<section class="slide">` of the slide type you need: title, section, points, two
   columns, figures, closing. Number the footers in order.
4. Set the `<title>` to the deck or document name.
5. To publish it as an artifact, publish the HTML file. To make a PDF, print it as below.

The templates load Outfit, Geist and Geist Mono from Google Fonts and inline the mark as SVG, so one file is the
whole deliverable. The deck prints one 1280×720 slide per page. The document prints on A4 with page numbers.

```bash
chromium --headless --no-sandbox --virtual-time-budget=8000 --no-pdf-header-footer \
  --print-to-pdf=out.pdf file:///abs/path/to/deck.html
```

`--virtual-time-budget` gives the web fonts time to load; without it the PDF falls back to system fonts.

## Build a .pptx or .docx

Both scripts need `python-pptx` or `python-docx`. Run either one with an output path to get a sample that shows
every helper:

```bash
python3 .claude/skills/margince-brand/scripts/margince_pptx.py sample.pptx
python3 .claude/skills/margince-brand/scripts/margince_docx.py sample.docx
```

To build a real file, import the module from the `scripts/` folder and call its helpers:

- `margince_pptx`: `new_deck`, `title_slide`, `section_slide`, `content_slide` (band, eyebrow, title, lead,
  bullets), `card`, `stat`, `closing_slide`. Place custom shapes on a content slide from `body_top()` down.
- `margince_docx`: `new_document` (emerald band on page one, thin band on later pages, page numbers), `lead`,
  `heading`, `paragraph`, `bullets`, `callout` (with `warn=True` for an open point), `table` (pass `numeric` for
  columns of figures).

The helpers set the fonts by name. A machine without Outfit and Geist shows a system face, and the layout still
holds. Tell the user so when they send the file to others.

## Check before you hand it over

Render the result to images and look at every page: HTML or PDF through `pdftoppm`, `.pptx` and `.docx` through
`soffice --headless --convert-to pdf` and then `pdftoppm`. Look for text that runs out of its box, a slide that
spills onto a second page, and a lockup that is missing or stretched.

## Brand rules

### Colour

| Role | Value | Use |
|---|---|---|
| Emerald | `#0b7a53` | The band, title and closing slides, bullets, rules, one highlight per slide |
| Emerald ink | `#0a6f4b` | Eyebrows, links, table heads |
| Emerald tint | `#e8f3ee` | Callouts, table head fill, a selected row |
| Paper | `#f1f5f2` | Slide ground |
| Ink | `#101a15` / `#33403a` / `#66736c` | Headings / body / labels and meta |
| Indigo | `#5b61d6` | Only for content about the AI agent's work |
| States | `#17c964` / `#f5a524` / `#ff383c` | Success / warning / danger, as a soft tint behind a word |

Emerald is the one brand colour. Grey carries everything else, and no colour is used only to decorate.

### Type

- Headings: **Outfit** 700, letter spacing −0.02em.
- Body: **Geist** 400; 500 for a label beside an icon.
- Eyebrows and small labels: **Geist Mono** 500, uppercase, tracked +0.08em.
- Figures in a table or a stat: tabular numbers, aligned right.

### Logo

- On emerald, use the white lockup. On paper or white, use the colour lockup: emerald mark, ink word.
- Never recolour the mark, stretch it, set it on a photo, or add a shadow.
- Keep clear space around it of half the mark's height.
- Files in `assets/`: `lockup-white.png`, `lockup-color.png`, `logomark-white.png`, `logomark-emerald.png`, and
  the mark as `logomark-white.svg` and `logomark-emerald.svg`.

### Layout

- Every content slide and the first page of a document open under the emerald band, with the lockup at the left
  and the deck or document name at the right.
- Title and closing slides are a full emerald field; a section slide has an emerald strip on its left third.
- One idea per slide: an eyebrow, a heading that states the idea in one sentence, then at most five points.
- Leave empty ground empty. Split a full slide in two rather than shrink its type.

## Copy rules

The same claims hold as on the public website:

- Margince is "source-available (BUSL-1.1)". Never write "open source".
- Write "GDPR-ready", never "GDPR-compliant" or "certified".
- No figure without a source the user gave, no testimonials, and no attacks on other vendors.
- The call to action is "Request a conversation". Never "Book a demo", "Buy now" or a waitlist.
- German copy uses "du", never "Sie".

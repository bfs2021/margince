---
name: margince-brand
description: Style presentations, slide decks and documents (reports, one-pagers, memos, proposals, briefs) in the Margince brand, with the logo on an emerald band at the top. Use whenever the user asks for a deck, slides, a presentation or a document "in Margince style", "on brand", "with our logo", or for Margince, Gradion or a Margince customer. Covers HTML (as an artifact or printed to PDF), PowerPoint (.pptx) and Word (.docx).
---

# Margince brand for decks and documents

Every page or slide this skill makes carries the Margince lockup (the "M" mark and the word "Margince") in white
on an emerald band at the top. The look is the product's own: a paper ground lit by soft emerald and indigo glows,
a rounded gradient band set in from the edge, pill labels, white cards with soft shadows, and large headings. The colours, fonts and mark come from the product: `DESIGN.md` sections 3 and 4, and
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
   cards, figures, closing. Number the page pills in the band in order.
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
  numbered points, page pill), `card`, `stat`, `closing_slide`. Positions are CSS pixels of the 1280×720 HTML
  slide. Place custom shapes on a content slide from `body_top(slide)` down.
- `margince_docx`: `new_document` (the emerald card on page one, page numbers), `lead`, `heading` (numbers each
  section), `paragraph`, `bullets`, `callout` (with `warn=True` for an open point), `table` (pass `numeric` for
  columns of figures).

Office files cannot draw the gradients, so the helpers place images from `assets/` under the editable text:
`slide-hero.jpg`, `slide-paper.jpg`, `slide-band.png`, `slide-panel.png` and `doc-band.png`. They are drawn from
`assets/backgrounds.html`. After a change to a gradient there or in a template, run
`scripts/render_backgrounds.sh` (set `CHROME` to the Chromium binary) to draw them again.

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
| Emerald | `#0b7a53` | The band, point numbers, figures, one highlight per slide |
| Emerald lift | `#16a06c` | The light end of the band's gradient and of a card's icon tile |
| Deep emerald | `#075a3c` to `#06311f` | The mesh behind title, closing and section slides |
| Emerald ink | `#0a6f4b` | Eyebrow pills, links |
| Emerald tint | `#e8f3ee` | Eyebrow pill fill, callouts |
| Paper | `#f1f5f2` | Slide ground, with an emerald glow at the top left and an indigo glow at the top right |
| Ink | `#101a15` / `#33403a` / `#66736c` | Headings / body / labels and meta |
| Indigo | `#5b61d6` | A faint glow in the mesh and on paper; as a colour of its own, only for the AI agent's work |
| States | `#17c964` / `#f5a524` / `#ff383c` | Success / warning / danger, as a soft tint behind a word |

Emerald is the one brand colour. Grey carries everything else, and no colour is used only to decorate.

### Type

- Headings: **Outfit** 700, set large and tight: −0.025em on slide headings, −0.03em on title slides.
- Body: **Geist** 400; 500 for a label beside an icon or an eyebrow pill.
- Eyebrows are pills in sentence case, with a small dot before the word.
- **Geist Mono** 500 only for numbers that count: point numbers, section numbers, the page pill.
- Figures in a table or a stat: tabular numbers, aligned right.

### Logo

- On emerald, use the white lockup. On paper or white, use the colour lockup: emerald mark, ink word.
- Never recolour the mark, stretch it, set it on a photo, or add a shadow.
- Keep clear space around it of half the mark's height.
- Logo files in `assets/`: `lockup-white.png`, `lockup-color.png`, `logomark-white.png`, `logomark-emerald.png`,
  and the mark as `logomark-white.svg` and `logomark-emerald.svg`.

### Layout

- Every content slide opens under the band: a rounded emerald gradient bar set 20px in from the edge, with the
  lockup at the left and the deck name and a page pill at the right.
- A document's first page opens under a rounded emerald mesh card that holds the lockup, kind, title and meta.
- Title and closing slides are a full deep-emerald mesh with a faint grid fading in from the top right. A section
  slide has a rounded mesh panel on its left third.
- Cards are white, with a 22px radius, a hairline edge and a soft shadow. No hard rules, borders or bars.
- One idea per slide: an eyebrow, a heading that states the idea in one sentence, then at most five points.
- Leave empty ground empty. Split a full slide in two rather than shrink its type.

## Copy rules

The same claims hold as on the public website:

- Margince is "source-available (BUSL-1.1)". Never write "open source".
- Write "GDPR-ready", never "GDPR-compliant" or "certified".
- No figure without a source the user gave, no testimonials, and no attacks on other vendors.
- The call to action is "Request a conversation". Never "Book a demo", "Buy now" or a waitlist.
- German copy uses "du", never "Sie".

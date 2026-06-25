# OK Folio — Brand & Design System Brief

> **Product priority:** OK Folio is **aggregator-first**; the curation emphasis in this doc is the *secondary* layer. Canonical for V1 product priority: `product-prd.md` + `design-notes.md`.

This is the canonical brand brief for OK Folio. It guides design, UI, and public-facing copy.

## Identity

- **Product name:** OK Folio
- **Tagline:** A beautiful folio for visual discoveries.
- **Public site:** folio.oklabs.uk
- **Category:** Personal, open-source, self-hosted beauty system.

**Positioning.** OK Folio is a personal, self-hosted beauty system for collecting,
curating, and revisiting visual discoveries from many streams. It is image-first,
quiet, refined, and built for a person with taste who wants to gather systematically
chosen pieces of beauty from different sources.

## Brand philosophy

- **OK** is a hidden founder mark, not the main metaphor.
- **Folio** is the central metaphor: a curator's working folio — not a scraper,
  downloader, or generic photo app.
- The product is about **beauty, curation, calmness, and refined personal organization.**
- AI may be used extensively inside the product (categorization, metadata, similarity,
  suggested folios, visual research), but **AI is never the brand center.** Do not present
  OK Folio as an AI product.
- Open-source and self-hosted, but it should feel **elegant, editorial, and gallery-grade.**
- Not tied to any single source. Early streams should never define the brand.

**Avoid** in public brand copy: scraper, downloader, hoarder, surveillance, "AI-powered",
archive-first utility, and the legacy names (PhotoPrismSight, OK Sight Ex, PicEx, extractor).

## Core metaphor

A hybrid of a **frame** and a **page**:
- **Frame:** gallery, artwork, viewing, attention.
- **Page:** folio, editorial layout, curated collection, thoughtful record.

Do **not** use a camera or lens as the main symbol — it narrows the product toward
photography only.

## Tone

Calm, refined, personal, editorial, quiet confidence.

## Visual style

- Editorial serif / art-book mood for brand moments; neutral professional UI for daily work.
- Light editorial default theme. **Adaptive light/dark required.** Dark mode should feel
  like a gallery viewing mode, not a cyber/hacker dashboard.
- Clean, elegant palette — let images carry most of the color.
- Restrained accent color only for important states (favorite, selection, brand mark).
- No decorative gradient blobs, neon AI styling, loud tech dashboards, or marketing hero art.

**Suggested palette direction:** gallery white / warm paper · ink black · soft graphite ·
fine warm-gray lines · a rare muted cinnabar / deep gallery red accent (hearts, selected
state, brand mark).

## Typography

- An editorial brand face (serif / art-book) for brand moments.
- A neutral, professional UI face for everyday product surfaces.

## Information architecture

Top navigation: **Gallery · Folios · Inbox · Streams · Settings**

- **Gallery** is the first screen. **Streams are backstage/admin — never the center.**

Gallery modes: **Magazine** (default — editorial rhythm with featured pieces) ·
**Library** (regular grid for scanning/managing many pieces) ·
**Wall** (quiet, immersive, museum-wall viewing).

## Key surfaces

- **Piece detail** — a hybrid of a museum label and an editable metadata panel: show the
  piece beautifully first, then allow quiet editing of title, source, author/artist, date,
  notes, EXIF/file metadata (when available), folio membership, favorite state. Provenance
  is visible in detail, not noisy on every gallery card.
- **Add Piece** — a curated import flow (not a bulk dump): file/local import with title,
  source, author/artist, date, notes. The system can later enrich EXIF, dimensions, color,
  similar pieces, and suggested folios.
- **Inbox** — the review workspace. Calm actions: Keep / Dismiss / Add to Folio. Avoid
  aggressive productivity language.
- **Folios** — curated groups. Covers auto-selected by default, with room for manual
  override. The system may suggest auto-folios, but never brand that as "AI."

## Requested design deliverables

1. Brand identity direction.
2. Logo/symbol concepts based on frame + page.
3. Wordmark direction for "OK Folio".
4. Light and dark color tokens.
5. Typography recommendations (editorial brand face + neutral UI face).
6. Layout system for Gallery, Folios, Inbox, Streams, Settings.
7. Component examples: top navigation; gallery card / piece tile; magazine featured area;
   library grid; wall mode; piece detail; add-piece flow; folio card; inbox review row/card;
   stream admin card; favorite heart state.
8. README / public brand preview section.

## Design priorities

1. Beauty first.
2. Quiet curation second.
3. Provenance and metadata third.
4. Stream/admin management fourth.
5. Hidden intelligence in the background — never a loud AI pitch.

## Success criteria

- Feels like an elegant self-hosted visual folio, not a scraper dashboard.
- The first screen is useful as Gallery, not a landing page.
- Can hold photos, paintings, illustrations, scans, screenshots, and other visual pieces.
- Personal and calm, yet mature enough to be a public open-source product.

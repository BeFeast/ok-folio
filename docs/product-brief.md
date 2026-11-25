# OK Folio — Product Brief

> **Product priority:** OK Folio is **aggregator-first**; the curation emphasis in this doc is the *secondary* layer. Canonical for V1 product priority: `product-prd.md` + `design-notes.md`.

## One line

A beautiful folio for visual discoveries — a personal, self-hosted, open-source beauty
system for collecting, curating, and revisiting visual pieces from many streams.

## What it is / is not

**Is:** image-first, curated, calm, self-hosted, provenance-aware. A curator's working
folio.

**Is not:** a scraper, a downloader, an archive utility, a surveillance tool, or an
"AI product." Not a landing page — the Gallery is immediately useful.

## Information architecture

Top navigation: **Gallery · Folios · Inbox · Streams · Settings**

- **Gallery** — the default, first screen. Modes: **Magazine** (default), **Library**, **Wall**.
- **Folios** — curated groups of pieces; auto-selected covers, manual override later.
- **Inbox** — review workspace for new/unreviewed pieces: Keep / Dismiss / Add to Folio.
- **Streams** — backstage/admin for incoming sources and connectors. Not the center.
- **Settings** — preferences, theme (adaptive light/dark), account/instance config.

## Core terms

| UI term | Internal/API term | Meaning |
|---|---|---|
| Piece | Item | A single visual object. |
| Folio | — | A curated group/collection. |
| Gallery | — | The main viewing surface. |
| Inbox | — | Incoming/unreviewed pieces. |
| Streams | Ingest / Channels | Sources and connectors (backstage). |
| Add Piece | — | Manual curated import. |
| Favorite | — | Heart. |

## Piece detail

Museum-label-first, then a quiet editable metadata panel:
title · source · author/artist · date · notes · EXIF/file metadata (when available) ·
folio membership · favorite state. Provenance shown in detail, not on every card.

## Add Piece flow

Curated, not bulk. File/local import with title, source, author/artist, date, notes.
Later enrichment: EXIF, dimensions, color descriptors, similar pieces, suggested folios.

## Inbox workflow

New pieces may appear in the Gallery as unreviewed, but the Inbox is the review space.
Calm suggested actions: Keep · Dismiss · Add to Folio.

## Architecture direction

- Monorepo: backend/API plus frontend in one repository.
- Provider/source connectors live under a **Streams / Channels / Ingest** architecture;
  "Ingest" is acceptable as an internal technical term. "Extractor" is not the public metaphor.
- Provider-specific logic stays isolated per connector.
- Tests are fixture-backed and safe from production data.
- AI/enrichment is internal and quiet — never the brand center.

## Current state

The repository is evolving from a legacy media-extractor seed. The Go backend and Vite/React
dashboard are the starting point; the brand and UI are being designed toward the IA above.
See [migration-from-ok-sight-ex.md](migration-from-ok-sight-ex.md).

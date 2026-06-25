# OK Folio

**A beautiful folio for visual discoveries.**

OK Folio is a personal open-source beauty system for collecting, curating, and
revisiting visual discoveries from many streams. It is self-hosted, image-first,
and quiet by design: a private gallery where beautiful pieces can be gathered,
reviewed, organized into folios, and enriched with provenance and notes.

It is not a scraper, a downloader, or an admin tool. It is a curator's working
folio — calm, refined, and built for a person with taste who wants to gather
systematically chosen pieces of beauty from different sources.

## What it is

- **Image-first.** The Gallery is the first screen, not a dashboard or a landing page.
- **Curated.** Pieces are gathered into Folios, triaged in the Inbox, and given
  provenance and notes.
- **Self-hosted & open-source.** Your collection stays on your own machine.
- **Quiet intelligence.** Categorization, metadata, similarity, and suggested folios
  may be assisted in the background — but OK Folio is not marketed as an "AI product."
- **Many sources.** Streams bring pieces in from different providers. No single source
  defines the product.

## Language

| Term | Meaning |
|---|---|
| **Piece** | A single visual object — photo, painting, illustration, scan, screenshot. |
| **Folio** | A curated group of pieces. |
| **Gallery** | The main viewing surface. Modes: Magazine (default), Library, Wall. |
| **Inbox** | Where new, unreviewed pieces are triaged (Keep / Dismiss / Add to Folio). |
| **Streams** | Backstage area for incoming sources and connectors. |
| **Add Piece** | Manual, curated import with title, source, author, date, and notes. |

Top navigation: **Gallery · Folios · Inbox · Streams · Settings**

See the brand and product docs:
[brand system](docs/brand-system.md) ·
[product brief](docs/product-brief.md) ·
[glossary](docs/brand-glossary.md) ·
[migration note](docs/migration-from-ok-sight-ex.md)

## Status

Early. The brand and product design system are being established. The current codebase
is a Go backend plus a Vite/React dashboard, evolving from a legacy media-extractor seed
toward the OK Folio product. The live legacy runtime is intentionally untouched until a
separate, explicit deployment plan is approved.

## Development

```bash
go test ./...
cd dashboard && npm ci && npm run build
```

Or run the repository verifier:

```bash
./scripts/product-verifier.sh
```

## License

MIT — see [LICENSE](LICENSE).

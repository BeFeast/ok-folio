# OK Folio - Design Notes (aggregator-first realignment)

The OK Folio brand and product design system (`brand-system.md`, `product-brief.md`) was
authored with a **curation-first** emphasis. The product is **aggregator-first** (see
[product-prd.md](product-prd.md)). The brand **aesthetic is kept**; this note records the
**information-priority** adjustments so UI work optimizes the right primary flow.

## Keep (aesthetic)

- Palette: gallery white / warm paper, ink black, soft graphite, restrained cinnabar accent.
- Editorial serif for brand moments; neutral UI face for daily surfaces.
- Frame + page brand mark. Adaptive light/dark (dark = gallery viewing mode).
- IA: **Gallery · Folios · Inbox · Streams · Settings**. Modes: Magazine · Library · Wall.

## Change (priority, for aggregator-first)

1. **Default Gallery mode = Library** (dense grid for thousands), not Magazine. Magazine and
   Wall are secondary lenses, not the landing view.
2. **Add a facet / filter rail**: source, category, artist, favorites + search. Essential at
   scale; absent in the first design pass.
3. **Inbox = exceptions queue**, not a manual triage of every arrival. Default is auto-keep +
   dedupe; the Inbox surfaces duplicates / ambiguous only. Badge reflects exceptions, which
   should usually be small.
4. **Streams is the engine - make it first-class**: per-source counts, last sync, health.
   It is where aggregation is observed, not a hidden admin corner.
5. **Demote "Add Piece"** from a hero action to a secondary one. Manual add is the minor path
   in an aggregator.
6. **Copy at scale**: shift from "gathered / kept with intention" (small, curated set) to
   discovery-and-flow language over a large auto-aggregated collection. Show real counts.
7. **Piece cards** carry subtle provenance (source facet); detail view shows full provenance.

## Source design

Delivered by Claude Design (HTML artifacts + screenshots), brand-faithful. These notes are
the delta to apply when the design is implemented or iterated; the aesthetic direction
stands.

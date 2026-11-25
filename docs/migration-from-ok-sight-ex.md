# Migration Note — From the Legacy Seed to OK Folio

OK Folio is the new product identity. It grew out of a legacy media-extractor seed and is
being repositioned from an extractor/admin tool into a personal, image-first beauty system.

## Legacy names (reference only)

The following names are **legacy** and must not appear in public OK Folio product surfaces
(brand, README, UI copy, marketing):

- `OK Sight Ex` — previous product name.
- `ok-sight-ex` — previous repository slug.
- `PhotoPrismSight` — earlier project name of the seed.
- `photoprism-extractor` — previous Go module name (now `ok-folio`).
- `PicEx`, `extractor`, `sight.photo` — legacy/source references.

`sight.photo` was only the first historical stream and does not define the product. Treat all
of the above as implementation/reference lineage, not product identity.

## What was renamed (first public-facing pass)

- Product name → **OK Folio**; tagline → **A beautiful folio for visual discoveries.**
- Go module `photoprism-extractor` → `ok-folio` (and all import paths).
- Dashboard title, header, tagline, and footer → OK Folio.
- Navigation: the operations/admin view is now **Streams**.
- README replaced with the OK Folio brand README; added brand-system, product-brief, and
  glossary docs.

## Intentionally left for later (low-risk first pass)

- Some internal technical names remain (e.g. the `cmd/extractor` binary directory, the
  `webgallery` connector package, and assorted `extractor`/`photo` identifiers in code).
  These are internal and were kept to avoid unnecessary churn; "Ingest" is the preferred
  internal term going forward.
- The product information architecture (Gallery · Folios · Inbox · Streams · Settings),
  the Piece/Folio model, and the Gallery modes are the **design target** — the current
  dashboard is the starting point to evolve, not the finished product.

## Runtime safety

The legacy live runtime must remain untouched until a separate, explicit deployment plan is
approved. No deploys, no live runtime changes, no secret values in the repository.

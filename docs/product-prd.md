# OK Folio - Product PRD (V1)

## Product

OK Folio is a **self-hosted, single-user visual aggregator**. Connectors automatically
gather media from sources; the system organizes it by source / category / artist; and it
all lives in a beautiful custom gallery. Curation (folios, manual add, inbox triage) is a
**secondary layer**, not the core act.

V1 starts on an **existing web-gallery collection** (the first connector) and adds a
**Telegram forward-to-bot** connector as the second source.

## User and scale

- **Single user**, self-hosted on a private network. No auth, no multi-tenant.
- Collection scale: **thousands and growing** (auto-aggregated). Design for volume, not a
  hand-picked few.

## Core loop

1. **Streams** (connectors) gather new media on a schedule or as it is forwarded.
2. New arrivals are **auto-kept and de-duplicated**; the **Inbox** only surfaces
   duplicates / ambiguous items for a decision.
3. Kept pieces enter the **Gallery**, organized by source / category / artist facets.
4. The user **browses / searches / filters** at scale; optionally curates into **Folios**.

## V1 scope

- **Gallery** over the adopted collection. Modes: **Library (default, dense grid)**
  primary; **Magazine** (editorial/featured) and **Wall** (immersive) secondary.
- **Facets / filters**: source, category, artist, favorites; search; basic dedupe.
- **Inbox**: exceptions queue - duplicates and ambiguous arrivals only (default is
  auto-keep + dedupe).
- **Streams**: connector management/status - per-source counts, last sync, health.
- **Connectors**:
  - **Web-gallery** (existing parser as compatibility baseline).
  - **Telegram (forward-to-bot)**: the user forwards media from Telegram into their own
    OK Folio bot (they are its admin); the connector ingests forwarded message media via
    the Bot API, preserving original-source metadata where present. Forward-going only.
- **Piece detail**: image-first + metadata/provenance (source, artist, date, category) +
  favorite.
- **Folios + manual Add Piece**: secondary curation layer, minimal in V1.

## Out of V1 (later)

- AI organization / similarity / suggested folios (V1 organizes by existing metadata only).
- Additional connectors (e.g. a Telegram user-session connector for channel backfill;
  other providers after compliance + credential review).
- Multi-user / sharing.
- Advanced curation (bulk ops, saved smart filters) beyond the basics.

## Architecture

- **Custom OK Folio gallery**: Go backend + Vite/React frontend, typed API contracts;
  connectors isolated behind the provider interface (`DiscoveredMedia`, dedupe keys,
  typed errors). See [architecture-decision.md](architecture-decision.md) lineage in the
  project history.
- **Adopt existing catalog**: read the existing originals/DB **in place, as-is** (no import
  into a new schema in V1). Any destructive migration is a separate, reviewed plan.
- **Runtime**: deployment platform owns lifecycle; secrets in the deployment secret store;
  deploy is separate and approval-gated.

## Design

Aggregator-first realignment of the OK Folio design system. See
[design-notes.md](design-notes.md). The brand aesthetic (warm paper, editorial serif,
restrained accent, frame+page mark, Gallery/Folios/Inbox/Streams/Settings, Magazine/
Library/Wall) is kept; the **information priority** shifts to browsing/triaging an
auto-aggregated collection at scale.

## Ingestion model: auto-keep + dedupe

- New media from any connector is **auto-kept** by default.
- A **dedupe** step drops exact/near duplicates; the dedupe key is connector-specific
  (e.g. a stable source id + media id), never a local file path.
- The **Inbox** holds only the exceptions: duplicates to confirm and ambiguous items.

## Telegram (forward-to-bot) details

- Bot API only. The user forwards media into their own bot; the connector reads forwarded
  `message` media via `getUpdates` (or webhook) and resolves files by `file_id`.
- Original source is taken from `forward_origin` / `forward_from_chat` when present and used
  for provenance and the dedupe key.
- The **bot token lives in the deployment secret store only** - never in git, issues, or
  logs. Rotate the token via BotFather if it is ever exposed.

## Data safety / non-negotiables

- Production data (originals, DB, thumbnails, provider cookies, runtime config) is
  protected; tests are fixture-backed; workers never deploy; lifecycle goes through the
  deployment platform; secrets stay in the secret store, never in git.

## Verification

```bash
go test ./...
cd dashboard && npm ci && npm run build
# or
./scripts/product-verifier.sh
```

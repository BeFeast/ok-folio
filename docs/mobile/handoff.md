# Handoff: OK Folio — Mobile (Responsive Web + PWA)

## Overview
OK Folio is a personal, self-hosted, image-first gallery for collecting and curating
visual pieces from many sources (app: **folio.oklabs.uk**). This handoff covers the
**phone-first responsive web + installable PWA** experience — the same product as
desktop, adapted to small screens. It must feel like a real installed app: home-screen
icon, standalone chrome (no browser bar), themed splash, app-like navigation and gestures.

Tagline: *A beautiful folio for visual discoveries.* Mood: calm, gallery/museum,
generous whitespace, image-first, restrained motion.

**Voice constraints (do not use these words anywhere in UI copy):** scraper, downloader,
hoarder, surveillance, "AI-first", or legacy names (PhotoPrism, OK Sight Ex, PicEx).

## About the Design Files
The file in this bundle (`OK Folio - Mobile.dc.html`) is a **design reference created in
HTML** — a prototype showing intended look, layout, and behavior. It is **not production
code to copy directly.** It is a single pannable "canvas" document containing ~30
annotated phone/tablet frames laid out side by side.

Your task is to **recreate these designs in the target codebase's existing environment**
(React, Vue, Svelte, SwiftUI, native, etc.) using its established components, patterns,
and libraries. If no front-end environment exists yet, choose the most appropriate
framework for an installable PWA and implement there. Reuse the existing OK Folio brand &
design system — do not invent a new visual language.

> `support.js` is only the runtime that renders the HTML reference. **Ignore it for
> implementation** — it is not part of the product.

## Fidelity
**High-fidelity (hifi).** Final colors, typography, spacing, and interaction intent are
specified. Recreate the UI faithfully using the codebase's libraries. The one exception:
artwork tiles in the mock are abstract CSS-gradient placeholders standing in for real
images — wire them to actual piece thumbnails/full images.

---

## Design Tokens

### Color — Light (default)
| Token | Hex | Use |
|---|---|---|
| `accent` | `#7C2420` | Primary actions, active tab, selection, badges, links |
| `paper` (bg) | `#F3EFE7` | App background, sheets |
| `paper-raised` | `#FBF8F1` | Cards, inputs, list rows, sheets-on-paper |
| `ink` | `#1C1A16` | Primary text, titles |
| `ink-70` | `rgba(28,26,22,.7)` | strong secondary |
| `ink-55` | `rgba(28,26,22,.55)` | body secondary text |
| `ink-50/45` | `rgba(28,26,22,.5/.45)` | tertiary / metadata / inactive labels |
| `hairline` | `rgba(40,30,20,.08–.12)` | borders, dividers |
| `chip-border` | `rgba(40,30,20,.2)` | unselected chip outline |
| `tile-skeleton` | `#E7E1D4` | skeleton/placeholder tiles, segmented-control track |
| `danger` | `#C0392B` | destructive (Delete, "Source removed" pill, dismiss) |
| `warning` | `#8a6d1a` on `rgba(180,140,30,.16)` | "Missing image" pill |

### Color — Dark
| Token | Hex | Use |
|---|---|---|
| `accent` | `#C75D49` | Primary actions, active tab, selection, badges (warm terracotta) |
| `bg` | `#16130E` | App background (near-black, warm) |
| `surface` | `#161310` | sheets/cards |
| `surface-raised` | `rgba(255,255,255,.06–.07)` | segmented track, raised fills |
| `text` | `#ECE6DA` | primary text |
| `text-50` | `rgba(236,230,218,.5)` | secondary/metadata |
| `hairline` | `rgba(236,230,218,.08–.12)` | borders/dividers |

On accent buttons in dark mode, label/icon color is the dark bg `#16130E` (not white).

### Typography
- **Display / titles:** `Newsreader` (serif). Weights 300/400/500/600. Used for screen
  titles, piece titles, folio names, empty-state headings, museum labels.
- **UI / labels / metadata:** `Instrument Sans` (sans). Weights 400/500/600/700. Used for
  all controls, body, captions, pills, nav labels.

Type scale observed in the mock (px):
| Role | Font / weight / size / line-height |
|---|---|
| Screen title | Newsreader 500 · 26 |
| Screen title (compact, w/ mode bar) | Newsreader 500 · 24 |
| FolioDetail / viewer big title | Newsreader 400 · 28 / 26 |
| Piece title (grid caption) | Newsreader 500 · 13–15 |
| Empty-state heading | Newsreader 500 · 21–22 |
| Body / field text | Instrument Sans 400 · 14–15 |
| Metadata / caption | Instrument Sans 400 · 11–12 |
| Nav label | Instrument Sans 500/600 · 9.5–10 |
| Section label (ALL-CAPS) | Instrument Sans 600 · 11, letter-spacing .05–.06em, ink-45 |
| Button label | Instrument Sans 600 · 14–15 |
| Chip / pill | Instrument Sans 500 · 12–13 |

### Spacing & Geometry
- Screen horizontal padding: **20px** (16px on dense 3-up grids / pickers).
- Grid gaps: 14–16px (2-up), 8px (3-up), 2–3px (Wall, edge-to-edge).
- Radii: **piece tiles 3px**, cover stack 3px, cards/rows 12–14px, inputs 11px,
  buttons 12–13px, pills/chips 16–18px (full), bottom sheet top corners 24px,
  phone frame 46px (device), tablet frame 34px.
- Primary button height **52px**; secondary/icon controls 44–48px.
- Accent button shadow: `0 8px 20px rgba(124,36,32,.3)`.
- Bottom sheet shadow: `0 -18px 40px rgba(0,0,0,.25)`; scrim `rgba(20,14,10,.5)`.
- Toast shadow: `0 14px 34px rgba(0,0,0,.3)`.

### Safe areas & device
- Design width **390pt**; verify **360–430**.
- Respect notch / home-indicator insets on every full-bleed screen
  (status bar ~50px top; home indicator ~20–26px bottom; the tab bar hugs it).
- **Tap targets ≥ 44pt.** No hover-only affordances.

---

## Navigation Model
- **Bottom tab bar** (fixed): Gallery · Folios · Inbox · Streams · Settings.
  - Translucent paper, `backdrop-filter: blur(18px)`, 1px top hairline, sits above the
    home indicator.
  - Active tab = accent icon **and** label; inactive = ink-50 icon + label.
  - **Inbox** shows an unread **count badge** (accent fill, 1.5px bg-colored ring,
    700 weight numeral) at top-right of its icon.
- **Compact top bar:** screen title (Newsreader) on the left; on the right, three actions:
  **search** (magnifier), **light/dark toggle** (half-moon), and the accent **Add piece**
  (+, the only filled control). Icon buttons ~40px circular.
- **Search:** tapping the magnifier expands a full-width rounded field (focus ring
  `0 0 0 4px rgba(124,36,32,.08)`, accent border) with a **Cancel** text button; keyboard
  rises. Shows RECENT rows (clock icon + term) then a live "MATCHING PIECES · N" 3-up
  thumbnail grid. Same field is the **author type-ahead** used by Gallery filters.
- **Tablet:** bottom tabs **promote to a left rail** (~84px) with the app icon on top;
  content grid opens up (see Tablet Variant). Sheets become centered popovers.

---

## Screens / Views

### 1. Gallery (core grid)
Top bar = title "Gallery" + search/add icons. Below the title a **segmented control**
(`Magazine | Library | Wall`, selected segment = accent fill on a `#E7E1D4` track) and a
right-aligned **Filters** button (outlined, sliders icon). Infinite scroll with momentum;
tiles fade in at the end. Tapping any piece → Piece Viewer (opens from the tapped tile).

Responsive columns: **1 on small phones, 2 on large/landscape**; Wall adds a 3rd.

Three modes:
- **Magazine** — editorial varied layout: a full-width hero (196px) with an overlaid
  caption (gradient scrim bottom; Newsreader title + Instrument Sans artist/date in white), then a
  mixed row (one tall tile beside two stacked tiles), then a wide tile. For slow browsing.
- **Library** — uniform 2-col grid; each cell = image (124px) + **museum label below**
  (Newsreader title + Instrument Sans "Artist · Year"). For scanning by title/author.
- **Wall** — edge-to-edge salon hang: `grid-template-columns: 1fr 1fr 1fr`, `gap: 3px`,
  `grid-auto-rows: 84px`, some tiles `grid-row: span 2`. No gutters, no labels.

States:
- **Loading** — title shows, grid is shimmer skeleton tiles in final aspect ratios; a
  centered "Loading your folio…" with spinner. App shell paints instantly from cache.
- **Empty (first run)** — centered BrandMark (outlined), Newsreader "Nothing here yet",
  body copy, accent **"Add your first piece"** button. Warm, never scolding.
- **Filtered-empty** — active filters remain as removable chips under the title; centered
  magnifier-with-minus glyph, "No pieces match", "Try loosening one", outlined
  **Clear filters**.

### 2. Filters (bottom sheet)
Rises over a dim scrim from the bottom; 24px top radius, drag handle. Contents:
- Title "Filters" (Newsreader) + "Reset" text button.
- **Favorites only** row (heart icon + label) with an accent toggle (on).
- **ARTIST** type-ahead input (magnifier + "Type to find an artist…").
- **MEDIUM** chips (Painting/Photography/Drawing/Print/Sculpture): selected = accent
  fill/white text; unselected = outline.
- Sticky accent **"Show N pieces"** button — count is live as filters change.

### 3. Piece Viewer (immersive, touch-native)
Full-screen artwork on near-black (`#0c0a08`). Chrome floats on blurred pills
(`rgba(20,14,10,.4)`, blur):
- Top: **close ×** (left), **counter** "12 / 248" (center), **favorite heart** (right,
  filled accent when favorited).
- Bottom **peek**: gradient scrim with Newsreader title, Instrument Sans "Artist · Year", a
  "Swipe up for details" hint, and an up-chevron.
- Chrome **auto-hides** after a beat (tap to bring back).

**Gestures:** swipe **◀ / ▶** → prev/next piece · swipe **▼** → dismiss (rubber-bands the
artwork down with a gentle scale, then drops back to the exact grid tile it came from) ·
swipe **▲** → open info sheet. Honor **reduced-motion** (fall back to a quiet fade).

**Info bottom sheet** (hidden by default): drag-up reveals a sheet (surface `#161310`,
24px top radius) with the **museum label** — big Newsreader title, accent artist name, then
a key/value list: **Date, Medium, Source, Dimensions, File** (e.g. "3024 × 2796 · 4.2 MB").
**KEYWORDS** as chips. Pinned footer actions: accent **"Add to folio"**, a **share**
button, a **favorite** button.

### 4. Folios
- **Cover grid** — title "Folios" + accent Add (+). 2-col grid of **cover tiles**: a
  top piece image over a peeking second image (offset `inset:8px 0 0 6px`, opacity .55)
  to read as a stack; below = folio name (Newsreader) + "N pieces". A dashed **"New folio"**
  tile (accent dashed border, + icon) closes the grid.
- **Create / rename / delete** — long-press (or ⋯) a folio → action sheet listing the
  folio header, **Rename**, **Change cover**, **Delete folio** (danger red), plus a
  separate **Cancel** card. The selected folio shows an accent selection ring behind the
  sheet. Delete confirms. The "+" opens the same sheet to name a new folio.
- **FolioDetail** — back chevron + ⋯ menu; big Newsreader folio name + "N pieces · updated …".
  Tighter **3-up** grid (108px tiles, 8px gap). A floating accent **"Add pieces"** pill
  over a bottom paper-fade.
- **Add pieces (multi-select picker)** — full-screen sheet "Add to <Folio>" with
  **Cancel** / **Done** header. 3-up thumbnail grid; tap to select → accent ring
  (`box-shadow: 0 0 0 3px accent`) + accent tint overlay + filled check badge; unselected
  shows an empty white ring. Sticky footer accent button **"Add N pieces"** (live count).

### 5. Inbox (exceptions queue)
Title "Inbox" + "N to review". **Text-first rows** (paper-raised cards): a status **pill**
(e.g. "Source removed" = danger; "Possible duplicate" = accent-tint; "Missing image" =
warning), Newsreader title, Instrument Sans artist, the **reason in plain words**, and a **source/
action link** (arrow-out icon + "View source" / "Compare" / "Retry import"). A
**thumbnail** (54px) when the matched piece has an image, else a placeholder glyph tile.
A **×** dismiss button at the row's top-right.
- **Swipe-to-dismiss** — swipe a row left to reveal a danger **Dismiss** action behind it
  (× + label); the × does the same. Dismissed items leave a brief **undo** toast.
- **All caught up (empty)** — accent-tint circle with a check, Newsreader "All caught up",
  body copy. No tab badge when empty.

### 6. Add piece (full-screen sheet)
Rises with 24px top radius, drag handle, **Cancel / Save** header, "Add a piece" title.
- **Source row:** three options — **Library** (selected style: accent icon), **Camera**,
  **URL** — as 74px rounded buttons.
- **Preview** of the chosen image (150px).
- **Form** (paper-raised inputs, ALL-CAPS labels): **Title**, **Artist** + **Date** (on
  one row), **Source**, (and **Notes**). 
- Sticky accent **"Add to folio"** button.
- On submit the sheet **closes instantly** and import runs in the background.

### 7. Progress toast (non-blocking)
After Add: a **bottom-center** toast (above the tab bar), ink `#1C1A16` card, spinner
(accent), "Importing "<title>"" + "Adding to your folio in the background…".
Auto-dismisses. Toasts/feedback always render bottom-center, above the tab bar.

### 8. Streams
Title "Streams" + subtitle "Sources that fill your folio" + accent Add. Single-column
stack of source cards: a 44px source thumbnail, Newsreader source name, Instrument Sans
"Type · synced … · N pieces" (API / RSS / Manual), and an **on/off toggle** (accent when
on, neutral when off). Calm, list-like, imagery subdued.

### 9. Settings (single-column, grouped)
Title "Settings". Groups with ALL-CAPS labels:
- **APPEARANCE** — segmented **Light | Dark | Auto** (drives the whole app); a
  "Default gallery mode" disclosure row.
- **STORAGE & SYNC** — "Offline cache (size)", "Sync over cellular" toggle, "Server
  address (folio.oklabs.uk)".
- App footer: app icon + "OK Folio · Version 2.4 · Installed".
Quiet rows, hairline dividers, switches.

### 10. Offline
Status bar shows struck-through signal/wifi. A paper-raised **offline banner** under the
title (struck wifi icon + "You're offline" + "Showing your saved pieces — new imports
resume when you reconnect."). Cached tiles render normally; **not-yet-cached** tiles hold
a quiet placeholder glyph on `#E7E1D4`. The **app-shell loads with no network** (PWA).

### Tablet Variant (Gallery & Folios)
Same product, more room. Bottom tabs → **left rail** (~84px: app icon top, then the 5
destinations with active = accent). Gallery opens to a **4-column** grid (Library mode
shown, 150px tiles, 22×20px gaps) with the mode segmented control + Filters in the header.
Sheets become centered popovers; the viewer keeps its full-bleed gesture behavior.

---

## Interactions & Behavior (summary)
- **Navigation:** tab bar switches top-level destinations; titles change per screen.
- **Search:** expand/collapse; recents + live results; doubles as artist type-ahead.
- **Filters:** bottom sheet; live result count; active filters persist as removable chips.
- **Viewer:** swipe ◀▶ paginate, ▼ dismiss-to-tile (scale + rubber-band), ▲ info sheet;
  chrome auto-hide; favorite toggle.
- **Folios:** long-press → rename/change-cover/delete; multi-select picker with live count.
- **Inbox:** swipe-left or × to dismiss (with undo); per-row contextual action links.
- **Add piece:** instant close + background import + non-blocking toast.
- **Motion:** subtle, brand-consistent (sheet rise, fade, gentle scale). **Honor
  `prefers-reduced-motion`** everywhere — degrade to fades.
- **States everywhere:** loading, empty, error, **offline** (cached app-shell + skeleton).

## State Management (per screen, framework-agnostic)
- Global: `theme` (light/dark/auto), `online`/offline status, unread Inbox count, current
  tab.
- Gallery: `mode` (magazine/library/wall), `filters` ({favoritesOnly, artist, mediums[]}),
  paginated `pieces` + infinite-scroll cursor, loading/empty/filtered-empty flags.
- Viewer: `currentIndex`, `total`, `favorited`, `infoSheetExpanded`, chrome-visible.
- Folios: `folios[]`, `selectedFolio`, FolioDetail `pieces`, picker `selectedIds` (count).
- Add piece: source choice, preview asset, form fields, in-flight import jobs (for toast).
- Streams: `streams[]` with enabled toggles + sync metadata.

## Assets
- **Icons:** simple line/solid icons (grid, stacked-frames, inbox tray, waves, sliders,
  search, +, half-moon, heart, share, chevrons, check, trash, pencil, link-out, camera,
  spinner). In the mock these are inline SVG — replace with the codebase's icon set,
  matching stroke weight ~1.7–2 and the rounded line caps shown.
- **App icon / BrandMark:** **two overlapping rounded picture frames** (a back frame
  outline + a front filled frame outlined in paper). Provide **maskable** (safe-zone
  padding, used full-bleed on accent) and **standard** forms; scales legibly to 16px (front
  frame alone at the smallest size). Theme-color meta = `#F3EFE7` (light) so OS chrome
  tints to paper; dark splash/identity uses `#16130E` + `#C75D49`.
- **Splash:** paper `#F3EFE7`, centered accent BrandMark + "OK Folio" (Newsreader) + small
  "A folio for visual discoveries".
- **Artwork:** the gradient tiles are **placeholders** — wire to real piece images.

## PWA notes
- Installable, **standalone** display (no browser bar). Manifest: name "OK Folio", theme
  color `#F3EFE7` (and dark variant), maskable + standard icons, the themed splash.
- Service worker caches the **app-shell** so it loads offline; cached piece images render,
  uncached show placeholders; background import resumes on reconnect.

## Files
- `OK Folio - Mobile.dc.html` — the full design reference (all screens + tablet + a
  component kit: tab bar, bottom sheet, filter chips, cover tile, inbox row, toast), light
  & dark, as an annotated canvas.
- `support.js` — runtime for the HTML reference only; **not** part of the product, ignore
  for implementation.

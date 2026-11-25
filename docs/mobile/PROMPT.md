CLAUDE CODE — IMPLEMENTATION PROMPT: OK Folio Mobile (Responsive Web + PWA)

The full spec is in README.md (same folder) and the visual reference is
`ok-folio-mobile.dc.html` (an annotated HTML mock — recreate it in this repo's stack; do
not copy its runtime `support.js`).

========================================================================
READ THIS FIRST — THE ONE RULE THAT WAS BROKEN LAST TIME
========================================================================
OK Folio is an IMAGE-FIRST gallery. Its slogan: "the Gallery is the first screen, not a
dashboard or a landing page." The previous implementation violated this — it opened with a
giant serif headline ("Your gathered pieces"), an italic count subtitle ("43,650 pieces,
kept with intention"), and three stacked control rows, so the first artwork appeared only
~60% down the screen. That is the #1 thing to fix and never repeat.

HARD CONTRACT for the Gallery and every grid screen (Folios, FolioDetail, add-pieces):
- The artwork IS the interface. Real image tiles must be visible on first paint, no scroll.
- Chrome above the first image row ≤ ~22% of viewport height (~180px on a 390×844 phone).
  First image row begins by ~25% down; ≥1.5 image rows visible initially.
- NO marketing hero. The heading is ONE small line: "Gallery" (serif, 24–26px). No big
  headline, no italic subtitle, no oversized piece-count banner.
- ONE "View" popup, opened from a top-bar icon, holds the mode switch AND all filters. The
  Gallery screen has NO permanent control row — just title + top-bar icons, then images.
  Inside the popup, Favorites is the FIRST control, then Layout (Magazine/Library/Wall),
  then artist, then medium.
- NO desktop-isms: mobile has no cursor. Never render "hover to preview" or any
  hover-dependent hint. Tap opens the piece.
- Edge-to-edge imagery: full content width (≤20px side padding; Wall = 0), grid bleeds
  under the translucent tab bar. Images dominate; UI recedes.
If you're adding vertical space above the images, delete it. Less chrome, more art.

(See README.md for tokens, navigation, the 10 screens + tablet variant, and PWA notes.)

========================================================================
ACCEPTANCE CHECK BEFORE YOU CALL IT DONE
========================================================================
Open the running Gallery at 390×844 and confirm: (a) real image tiles are visible without
scrolling, (b) chrome above the first image row is ≤ ~180px, (c) there is no headline,
subtitle, or count banner, (d) the mode switch + filters open from the top-bar view icon
(no permanent control row), (e) no "hover" copy anywhere. If any fail, the screen is not
done.

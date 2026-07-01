# OK Folio Design QA Screenshots

The design QA pass renders the OK Folio product surfaces with controlled Playwright fixtures. It uses the distilled Claude Design v0.1.0 package as the contract source and does not require live catalog, Folio, provider, or runtime data.

## Command

```bash
cd dashboard
npm ci
npx playwright install chromium
npm run qa:design
```

The test starts Vite on `127.0.0.1:4173`, mocks `/api/v1/**`, and captures both light and dark screenshots for:

- Gallery
- Folios
- Folio Detail
- Viewer
- Inbox
- Streams
- Settings
- Add Piece

## Outputs

Screenshots are written under:

```text
dashboard/test-results/design-qa/<viewport>/<theme>/<surface>.png
```

Current viewport projects:

- `mobile-390x844`
- `desktop-1366x900`

The generated screenshot and Playwright report directories are intentionally gitignored. Future Claude Design passes should update the fixture data or assertions in `dashboard/tests/design-qa.*` and compare regenerated outputs locally.

## Assertions

The pass fails on common render regressions:

- blank or sparse first paint
- missing controlled Folio/Gallery/Inbox fixture data
- broken image responses/placeholders
- horizontal overflow
- center-occluded headings and primary controls

It is a render QA gate, not a pixel-baseline test. When design-derived surfaces change intentionally, regenerate screenshots with the same command and use the output paths above for review evidence.

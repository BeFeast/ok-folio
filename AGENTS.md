# AGENTS.md - OK Folio Repository

This is the implementation repository for OK Folio, a personal, self-hosted, image-first gallery for collecting and curating visual pieces from many sources.

## Core Rules

- Keep code, comments, commits, PR titles, PR bodies, and issues in English.
- Do not commit secrets, `.env` files, provider cookies, database dumps, downloaded media, thumbnails, logs, build artifacts, `node_modules`, or generated bundles.
- Treat the legacy extractor code as seed material. New product identity is OK Folio / `ok-folio`.
- Immich is out of scope for this repository.
- Workers must keep PRs small and issue-scoped.

## Stack

- Backend: Go.
- Legacy dashboard: Vite + React.
- Runtime currently includes an extractor service, a gallery surface, and a database. Gallery architecture is still an open product decision: PhotoPrism may stay, be wrapped, or be replaced.

## Verification

Run the available checks before opening a PR:

```bash
go test ./...
cd dashboard && npm ci && npm run build
```

If a check cannot run, explain why in the PR body. Do not skip tests silently.

## Configuration And Secrets

- Use `config.example.yaml` as the documented shape.
- Real runtime config and secrets belong in the deployment secret store, not in git.
- Do not copy values from live runtime files into this repository.

## Runtime And Deploy

Workers do not deploy directly. Stack lifecycle is controlled by the deployment platform. Direct lifecycle commands are break-glass only.

Live verification must check the relevant LAN routes and real product behavior, not just process health.

## Review guidelines

- Treat committed secrets, `.env` data, provider cookies, database dumps, downloaded media, thumbnails, logs, build artifacts, `node_modules`, generated bundles, Immich changes, and direct deployment changes as blocking findings.
- Treat skipped CI, skipped product-verifier output, or unverified behavior changes as blocking findings unless the PR explains an issue-specific verifier failure.
- Flag changes that exceed the assigned issue scope or weaken CI gates or the configured review gate.

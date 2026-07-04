# Legacy-Retirement Preflight

`ok-folio-preflight` is a read-only verifier that gives an operator or PM one
place to gather evidence before retiring the remaining Wave 6 legacy services
(the stopped/startable PhotoPrism + MariaDB fallback and the retired legacy
extractor). It replaces the scattered manual checks that every handoff otherwise
has to rediscover.

## What it is (and is not)

The verifier is **strictly read-only**. It inspects committed repository state —
rendered Dockhand templates, config templates, and source — and prints
`PASS` / `WARN` / `FAIL` / `PENDING` evidence. It never:

- runs `docker compose up/down/restart` or any Dockhand mutation,
- runs `systemctl` or Maestro control-plane mutations,
- opens the legacy database, or
- requires any secret value for its offline checks (templates carry `${VAR}`
  placeholders, not values, and no evidence line prints a secret).

A guardrail test (`TestVerifierCannotSpawnProcesses`) proves the verifier code
imports no `os/exec`/`syscall`, so it is structurally incapable of mutating
lifecycle.

## Running it

```bash
# Offline (default): safe to run locally against the rendered templates.
make retirement-preflight
# or:
go run ./cmd/ok-folio-preflight
```

Exit code is `0` unless an **offline** check `FAIL`s. `PENDING` and `WARN` never
fail the run, so in-flight cutover phases (C3/C4) do not block development.

### Optional live probe

A single, clearly-separated read-only `GET` against a running app's
connector-status endpoint (`/api/v1/streams/connectors/status`) confirms the
running app surfaces the `webgallery:<id>` per-source id and Telegram freshness:

```bash
make retirement-preflight LIVE_URL=http://<app-host>:8080
# or:
go run ./cmd/ok-folio-preflight --live-connectors-url http://<app-host>:8080
```

The live probe is never required for the offline pass; an unreachable app is
reported as `WARN`, not `FAIL`.

## Checks

| Check | Meaning | Blocking? |
| --- | --- | --- |
| `normal-stack-decoupled` | The normal `compose.yaml` boots without required `LEGACY_*` env, an external legacy network, or writable legacy storage. | `FAIL` blocks |
| `photoprism-indexing-gated` | `photoprism.enabled`/`auto_index` default to `false` in the rendered and example config; the admin index route is a gated escape hatch. | `FAIL` blocks |
| `maintenance-commands-decoupled` | Ongoing maintenance commands live in `ok-folio-admin` (Phase C3). Absent → `PENDING`. | `PENDING` until C3 |
| `derivative-fallback-measured` | Legacy PhotoPrism storage is genuinely optional (no longer a required `${PHOTOPRISM_STORAGE_HOST_PATH:?}` substitution — a read-only mount alone is *not* optional) **and** its fallback is measured in the derivative path (Phase C4). Still required for boot or not yet measured → `PENDING`. | `PENDING` until C4 |
| `connector-state-surfaces` | Connector status surfaces per-source ids (`webgallery:1`) and Telegram freshness from durable `connector_state`, not the legacy extractor. | `FAIL` blocks |
| `legacy-extractor-retirement-documented` | Retirement is documented as expected stopped/startable (no lifecycle mutation is performed). | `WARN` if undocumented |

`PENDING` is deliberate: phases C3 and C4 land independently, and the verifier
reports their absence as pending rather than failing hard, so it stays useful
throughout the cutover window.

## Interpreting the result

- `READY` — all offline checks passed.
- `READY-WITH-PENDING` — no offline check failed; one or more checks are waiting
  on an in-flight phase (C3/C4). Cite the pending ids in the handoff.
- `NOT READY` — a retirement assumption failed; do not retire until it is `PASS`.

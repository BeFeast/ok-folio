# Dedicated OK Folio Dockhand Stack Runbook

This is the public-safe operator runbook for the dedicated OK Folio data stack.
The completed vault copy must contain concrete dataset names, host paths, ports,
LAN names, Infisical paths, and Dockhand details. Do not commit the completed
vault copy or any rendered `.env` file.

## Repository Artifacts

- Compose template: `deploy/dockhand/ok-folio/compose.yaml`
- Valkey config template: `deploy/dockhand/ok-folio/valkey.conf.template`
- Postgres initdb: `deploy/dockhand/ok-folio/initdb/010-vector-extensions.sh`
- Static template check: `scripts/check-ok-folio-stack-template.sh`
- Rendered legacy mount assertion: `scripts/assert-rendered-legacy-mounts-ro.sh`

The compose template intentionally contains no concrete IPs, host paths, secret
values, or live ports. Dockhand receives a staged stack directory rendered on the
deployment host.

## Vault Values To Record

Record these concrete values in the vault runbook before first deploy:

| Item | Vault value |
| --- | --- |
| Postgres ZFS child dataset | `<ok-folio-postgres-dataset>` |
| Postgres PGDATA host path | `<ok-folio-pgdata-host-path>` |
| Valkey sibling host directory | `<ok-folio-valkey-host-path>` |
| Rendered Valkey config host path | `<ok-folio-valkey-config-host-path>` |
| Config file host path | `<ok-folio-config-host-path>` |
| Originals host path | `<photo-originals-host-path>` |
| Daily host path | `<photo-daily-host-path>` |
| PhotoPrism storage host path | `<photoprism-storage-host-path>` |
| External legacy Docker network name | `<legacy-network-name>` |
| Verified-free app ops port | `<app-port>` |
| Verified-free Postgres ops port | `<postgres-port>` |
| Verified-free Valkey ops port | `<valkey-port>` |
| Staging hostname | `<staging-hostname>` |
| NPM LAN target | `<npm-lan-target>` |
| Cloudflared tunnel/config path | `<cloudflared-config-path>` |
| Dockhand base URL | `<dockhand-base-url>` |
| Dockhand auth method/header | `<dockhand-auth-contract>` |
| Dockhand create-stack route/body | `<dockhand-create-stack-contract>` |
| Dockhand deploy/up route/body | `<dockhand-deploy-contract>` |

The three ops ports must be a contiguous verified-free block for app, Postgres,
and Valkey. Re-check the block immediately before deployment:

```bash
ss -ltn "( sport = :<app-port> or sport = :<postgres-port> or sport = :<valkey-port> )"
```

The command must print no listeners. If any port is occupied, pick a new block
and update only the vault runbook and rendered `.env`.

## ZFS And Data Directories

Create a dedicated child dataset for Postgres under the plain Databases parent,
then create a sibling directory for Valkey:

```bash
zfs create <ok-folio-postgres-dataset>
zfs set recordsize=16K compression=lz4 atime=off <ok-folio-postgres-dataset>
install -d -o 999 -g 999 -m 0700 <ok-folio-pgdata-host-path>
install -d -o 999 -g 1000 -m 0700 <ok-folio-valkey-host-path>
zfs get recordsize,compression,atime <ok-folio-postgres-dataset>
stat -c '%u:%g %a %n' <ok-folio-pgdata-host-path> <ok-folio-valkey-host-path>
```

Expected: Postgres reports `16K`, `lz4`, `off`; PGDATA is owned by uid `999`;
Valkey data is owned by the current `valkey/valkey:8-alpine` service identity
(`999:1000`) so the entrypoint can continue on the non-root server path.

## Environment Contract

Use discrete `DB_*` keys as the single source of truth. Do not render
`DATABASE_URL` unless deliberately using the emergency escape hatch, and never
render both DSN styles at the same time.

Complete rendered `.env` key set:

```dotenv
OK_FOLIO_IMAGE_REGISTRY=<registry-host>
OK_FOLIO_IMAGE_SHA=<immutable-commit-sha>
OK_FOLIO_APP_PORT=<verified-free-app-port>
OK_FOLIO_POSTGRES_PORT=<verified-free-postgres-port>
OK_FOLIO_VALKEY_PORT=<verified-free-valkey-port>
OK_FOLIO_PGDATA_HOST_PATH=<postgres-pgdata-host-path>
OK_FOLIO_VALKEY_HOST_PATH=<valkey-host-path>
OK_FOLIO_VALKEY_CONFIG_HOST_PATH=<rendered-valkey-conf-host-path>
OK_FOLIO_CONFIG_HOST_PATH=<config-yaml-host-path>
POSTGRES_SHARED_BUFFERS=<host-budget-placeholder>
POSTGRES_EFFECTIVE_CACHE_SIZE=<host-budget-placeholder>
POSTGRES_ADMIN_USER=<ok-folio-postgres-bootstrap-user>
POSTGRES_ADMIN_PASSWORD=<ok-folio-postgres-bootstrap-password>
DB_HOST=postgres
DB_PORT=5432
DB_USER=<ok-folio-postgres-app-user>
DB_PASSWORD=<ok-folio-postgres-app-password>
DB_NAME=<ok-folio-postgres-dbname>
DB_SSLMODE=disable
VALKEY_HOST=valkey
VALKEY_PORT=6379
VALKEY_PASSWORD=<ok-folio-valkey-password>
VALKEY_MAXMEMORY=<host-budget-placeholder>
VALKEY_MAXMEMORY_POLICY=<host-budget-placeholder>
LEGACY_DB_HOST=<legacy-mariadb-container-name>
LEGACY_DB_USER=<legacy-read-user>
LEGACY_DB_PASSWORD=<legacy-read-password>
LEGACY_DOCKER_NETWORK=<external-legacy-network-name>
PHOTO_ORIGINALS_HOST_PATH=<photo-originals-host-path>
PHOTO_DAILY_HOST_PATH=<photo-daily-host-path>
PHOTOPRISM_STORAGE_HOST_PATH=<photoprism-storage-host-path>
TELEGRAM_BOT_TOKEN=<telegram-bot-token>
TELEGRAM_USERNAME=<telegram-username>
REGISTRY_URL=<registry-host>
REGISTRY_USERNAME=<push-scoped-ci-user>
REGISTRY_PASSWORD=<push-scoped-ci-password>
```

Add any net-new values under the OK Folio Infisical path as path references, not
literal values copied into git, issue comments, or PR bodies.

## Stack Services

Postgres uses `ghcr.io/tensorchord/vchord-postgres:pg18-v1.1.1`, sets
`PGDATA=/var/lib/postgresql/18/docker`, and mounts the ZFS-backed PGDATA path.
The server command disables ZFS-unfriendly WAL preallocation behavior and keeps
`shared_buffers` and `effective_cache_size` tunable until the host RAM budget is
final.

The initdb script creates `vector` as superuser and creates `vchord` when the
image exposes it. Initdb also creates and grants the least-privilege app role
from `DB_USER`/`DB_PASSWORD`. The OK Folio app role must never run
`CREATE EXTENSION`; app migrations only use the `vector` type when initdb
already made it available.

Valkey uses the alpine image and starts as `valkey-server` with a rendered
`valkey.conf` so the image entrypoint can switch to the non-root Valkey user.
The rendered config requires `VALKEY_PASSWORD`, enables appendonly persistence,
and keeps `maxmemory` and eviction policy as host-budget placeholders until
sizing is decided. Do not pass the Valkey password as a long-running process
argument.

The app image is always pinned as `ok-folio:<immutable-commit-sha>`. It joins
the private stack network and the external legacy Docker network. The app talks
to Postgres and Valkey by service name; published ports are for operations only.

All legacy mounts are kernel-enforced read-only:

- originals: `/photoprism/originals:ro`
- daily export: `/photoprism/_daily:ro`
- PhotoPrism storage/thumb tier: `/photoprism/storage:ro`

## Render And Assert

Render on the deployment host from Infisical and the vault path references. Do
not commit the rendered `.env`, rendered Valkey config, or rendered compose.
Stage the assertion script with the stack so this workflow does not depend on
the staged directory's location relative to a repository checkout.

```bash
cd <staged-ok-folio-stack-dir>
install -m 0755 <repo-checkout>/scripts/assert-rendered-legacy-mounts-ro.sh ./assert-rendered-legacy-mounts-ro.sh
infisical export --path <ok-folio-infisical-path> --format dotenv > .env
set -a
. ./.env
set +a
envsubst '$VALKEY_PASSWORD $VALKEY_MAXMEMORY $VALKEY_MAXMEMORY_POLICY' \
  < valkey.conf.template > "$OK_FOLIO_VALKEY_CONFIG_HOST_PATH"
chown 999:1000 "$OK_FOLIO_VALKEY_CONFIG_HOST_PATH"
chmod 0400 "$OK_FOLIO_VALKEY_CONFIG_HOST_PATH"
envsubst '$PHOTO_ORIGINALS_HOST_PATH $PHOTO_DAILY_HOST_PATH $PHOTOPRISM_STORAGE_HOST_PATH' \
  < compose.yaml > compose.mounts.rendered.yaml
./assert-rendered-legacy-mounts-ro.sh compose.mounts.rendered.yaml
```

The assertion must fail deployment if any legacy path is mounted without a
trailing `:ro`.

## Dockhand Deploy Contract

Docker lifecycle must go through Dockhand only. Do not run `docker compose up`
manually for this stack.

Before deployment work starts, complete one of these in the vault runbook:

- API path: capture the exact Dockhand create-stack route, deploy/up route,
  request bodies, auth headers, and whether stacks are filesystem-discovered or
  require registration.
- UI path: record the operator authorization that the Dockhand UI/button is the
  lifecycle action for this stack.

Then deploy in this order:

1. Create and verify ZFS/Valkey directories.
2. Add and render secrets from Infisical.
3. Build, smoke, and push the immutable `ok-folio:<sha>` image.
4. Stage the stack directory and render `.env` with the pinned sha.
5. Run the rendered legacy mount assertion.
6. Deploy via the verified Dockhand route or authorized UI action.
7. Verify service healthchecks are green.
8. Verify Postgres extensions:

```bash
psql "postgresql://<admin>@<lan-host>:<postgres-port>/<db>" \
  -c "select extname from pg_extension where extname in ('vector','vchord') order by 1;"
```

9. Verify AutoMigrate created the owned schema:

```bash
psql "postgresql://<app-user>@<lan-host>:<postgres-port>/<db>" \
  -c "\dt" \
  -c "\d downloaded_photos" \
  -c "select exists (select 1 from information_schema.columns where table_name = 'downloaded_photos' and column_name = 'embedding');"
```

10. Verify the app is reachable on the LAN ops port.

## UID 1000 Originals Read Smoke

The app runs as uid `1000`; Postgres PGDATA is uid `999`; PhotoPrism originals
may be owned by another uid. Prove uid `1000` can read real image bytes through
the read-only originals mount:

```bash
dockhand exec <ok-folio-app-container> -- id
dockhand exec --user 1000:1000 <ok-folio-app-container> -- \
  sh -c 'sample="$(find /photoprism/originals -type f | head -n 1)"; test -n "$sample"; stat "$sample"; head -c 16 "$sample" >/tmp/original-smoke.bytes; test -s /tmp/original-smoke.bytes'
```

If Dockhand does not expose exec, fetch a real photo through the app API and
assert the response body is non-empty. Record the sampled path ownership and the
smoke result in the vault runbook.

## NPM And Cloudflared

Create an NPM proxy host for `<staging-hostname>` to the app LAN target with the
wildcard certificate. Add SSE-safe advanced nginx settings:

```nginx
proxy_buffering off;
proxy_cache off;
proxy_read_timeout 1h;
proxy_send_timeout 1h;
proxy_http_version 1.1;
proxy_set_header Connection "";
```

Add a per-hostname cloudflared ingress rule for `<staging-hostname>` pointing to
NPM. Do not rely on a catch-all rule. Validate before reload:

```bash
cloudflared tunnel ingress validate --config <cloudflared-config-path>
```

Verify SSE through the public URL:

```bash
curl -N -H 'Accept: text/event-stream' https://<staging-hostname>/api/events
```

If the edge buffers despite these settings, document the LAN fallback URL and
the failed public-edge evidence in the vault runbook.

## Worker Boundary

This repository change can commit only the public-safe template and checks.
Concrete ports, datasets, secrets, Dockhand routes, deployment, healthchecks,
SSE verification, and real originals-byte smoke require deployment-host access
and must be completed by the operator before closing the issue.

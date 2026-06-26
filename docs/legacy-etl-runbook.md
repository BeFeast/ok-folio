# Legacy ETL Backfill And Incremental Sync

OK Folio imports only the two owned legacy tables, `downloaded_photos` and
`extraction_runs`. The legacy database is read-only to ETL and is never opened
through GORM or a live MySQL client in the app. Extraction is an operator-run
`mariadb-dump` stream; loading connects only to OK Folio Postgres and consumes
SQL from stdin.

## Preconditions

Run these checks through the host's container-exec path because the legacy DB
port is not published.

```sql
SHOW TABLE STATUS FROM `<legacy_database>` WHERE Name IN ('downloaded_photos','extraction_runs');
SHOW GRANTS FOR CURRENT_USER();
```

Both tables must report `ENGINE=InnoDB`. The ETL user must be
operator-provisioned out of band with table-level `SELECT` on exactly
`downloaded_photos` and `extraction_runs`, plus only normal `USAGE`. It must not
have global grants, database-wide grants, `GRANT OPTION`, root credentials, or
access to PhotoPrism tables. The password comes from the OK Folio Infisical path
and is passed through an environment variable or MariaDB defaults file, never as
a CLI argument.

The legacy source timezone is a human decision before loading. Pass the verified
IANA zone to `--legacy-timezone`; the loader sets the Postgres transaction time
zone so legacy `datetime(3)` values are interpreted as the correct instant.

## Backfill

The dump command must include:

```text
--single-transaction --skip-lock-tables --no-create-info --no-tablespaces --compact
```

It must not include `--master-data`, `--source-data`, or `--lock-all-tables`
because those can trigger `FLUSH TABLES WITH READ LOCK` and stall PhotoPrism.
Binlog coordinates are not used; watermarks live in OK Folio Postgres.

Use `cmd/ok-folio-etl` to print the exact checks and safe dump arguments:

```bash
go run ./cmd/ok-folio-etl print-legacy-checks --legacy-database "$LEGACY_DB_NAME"
```

Pipe the safe dump into the loader:

```bash
mariadb-dump --single-transaction --skip-lock-tables --no-create-info --no-tablespaces --compact \
  "$LEGACY_DB_NAME" downloaded_photos extraction_runs \
| go run ./cmd/ok-folio-etl load-dump --config /config/config.yaml --legacy-timezone "$LEGACY_SOURCE_TZ" --setval
```

The loader inserts real legacy IDs verbatim, stamps `provider='sight.photo'` on
new `downloaded_photos` rows, sets `url_hash` explicitly, preserves absolute
`file_path` and `artist`, and runs `setval` for both ID sequences when `--setval`
is provided.

## Upsert Contracts

`downloaded_photos` conflicts on `url_hash`. On conflict, ETL updates only
legacy-sourced mutable fields:

```text
source_page, title, artist, file_name, upload_date, file_path, file_size,
status, error_message, downloaded_at
```

It never updates OK Folio-owned or insert-derived fields in the conflict SET
list:

```text
favorite, content_hash, perceptual_hash, embedding, provider, category
```

`extraction_runs` conflicts on `id` and updates only:

```text
end_time, status, pages_processed, photos_found, photos_downloaded,
photos_skipped, photos_failed, error_message
```

Re-running an overlapping dump is expected to be idempotent.

## Incremental Sync

Incremental extracts should overlap with the previous watermark:

```sql
WHERE id >= :last_id
```

For `extraction_runs`, include the start-time guard:

```sql
WHERE id >= :last_run_id OR start_time > :last
```

Because `mariadb-dump --where` applies one predicate to every table named in
the command, run incremental extracts as table-specific dumps and pipe both into
the loader. The helper can print each safe command shape:

```bash
go run ./cmd/ok-folio-etl print-legacy-checks \
  --legacy-database "$LEGACY_DB_NAME" \
  --tables downloaded_photos \
  --where "id >= $LAST_PHOTO_ID"

go run ./cmd/ok-folio-etl print-legacy-checks \
  --legacy-database "$LEGACY_DB_NAME" \
  --tables extraction_runs \
  --where "id >= $LAST_RUN_ID OR start_time > '$LAST_RUN_START'"
```

The loader derives new watermarks from the rows actually loaded in the Postgres
transaction and advances `etl_watermark` only after commit. It never queries the
legacy source for a live `MAX(id)`.

Run the incremental job about every six hours until cutover. The recurring
runner location remains a human/deployment choice: host cron or a Dockhand
one-off are both acceptable as long as the command path and secret injection keep
the same constraints.

## Reconcile And Cutover Gate

Run a full reconcile at least daily during the migration window. Reconcile uses
the same full two-table dump and upsert contracts so drifted mutable fields that
do not bump a watermark, including `status` (`pending` included),
`error_message`, and `file_size`, converge again.

A final full reconcile immediately before cutover is a hard gate. The cutover
checklist must also record row counts for both owned tables at snapshot time,
the known-row `downloaded_at` before/after comparison to the second, and an app
smoke test that the gallery reads the catalog and resolves `file_path` against
the mounted originals.

## Content Hash Pass

Content hashing is decoupled from DB extraction:

```bash
go run ./cmd/ok-folio-etl hash-content --config /config/config.yaml --originals-root /originals-ro --limit 500
```

The pass reads file bytes from the read-only originals mount for rows where
`content_hash IS NULL` and writes the raw 32-byte sha256 to OK Folio Postgres.
It performs no legacy database or file writes and can be re-run safely.

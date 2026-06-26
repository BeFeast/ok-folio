#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/deploy/dockhand/ok-folio/compose.yaml"
initdb_file="$repo_root/deploy/dockhand/ok-folio/initdb/010-vector-extensions.sh"
valkey_template="$repo_root/deploy/dockhand/ok-folio/valkey.conf.template"

fail() {
  echo "ok-folio stack template check failed: $*" >&2
  exit 1
}

require_grep() {
  local pattern="$1"
  local file="$2"
  local message="$3"
  if ! grep -Eq -- "$pattern" "$file"; then
    fail "$message"
  fi
}

[ -f "$compose_file" ] || fail "missing $compose_file"
[ -f "$initdb_file" ] || fail "missing $initdb_file"
[ -f "$valkey_template" ] || fail "missing $valkey_template"

require_grep 'ghcr\.io/tensorchord/vchord-postgres:pg18-v[0-9]+\.[0-9]+\.[0-9]+' "$compose_file" "postgres must use a pinned VectorChord Postgres 18 image"
require_grep 'PGDATA: /var/lib/postgresql/18/docker' "$compose_file" "postgres must set the Postgres 18 PGDATA path"
require_grep 'full_page_writes=off' "$compose_file" "postgres must set full_page_writes=off"
require_grep 'wal_init_zero=off' "$compose_file" "postgres must set wal_init_zero=off"
require_grep 'wal_recycle=off' "$compose_file" "postgres must set wal_recycle=off"
require_grep 'shared_buffers=\$\{POSTGRES_SHARED_BUFFERS:-' "$compose_file" "postgres shared_buffers must be tunable"
require_grep 'effective_cache_size=\$\{POSTGRES_EFFECTIVE_CACHE_SIZE:-' "$compose_file" "postgres effective_cache_size must be tunable"
require_grep 'pg_isready' "$compose_file" "postgres must have a pg_isready healthcheck"

require_grep 'CREATE EXTENSION IF NOT EXISTS vector' "$initdb_file" "initdb must create the vector extension"
require_grep 'CREATE EXTENSION IF NOT EXISTS vchord CASCADE' "$initdb_file" "initdb must create vchord when available"
require_grep 'POSTGRES_USER: \$\{POSTGRES_ADMIN_USER:\?' "$compose_file" "postgres bootstrap user must be separate from the app role"
require_grep 'DB_USER: \$\{DB_USER:\?' "$compose_file" "postgres initdb must receive the app role name"
require_grep 'CREATE ROLE %I LOGIN PASSWORD %L' "$initdb_file" "initdb must create the least-privilege app role"
require_grep 'GRANT USAGE, CREATE ON SCHEMA public TO :"app_user"' "$initdb_file" "initdb must grant schema access to the app role"

if ! command -v rg >/dev/null 2>&1; then
  fail "ripgrep is required to verify application code does not run CREATE EXTENSION"
fi

extension_calls_file="$(mktemp)"
extension_errors_file="$(mktemp)"
trap 'rm -f "$extension_calls_file" "$extension_errors_file"' EXIT

set +e
rg -n --glob '!**/*_test.go' 'Exec\(.*CREATE[[:space:]]+EXTENSION|Raw\(.*CREATE[[:space:]]+EXTENSION' "$repo_root/internal" "$repo_root/cmd" >"$extension_calls_file" 2>"$extension_errors_file"
rg_status=$?
set -e

if [ "$rg_status" -gt 1 ]; then
  cat "$extension_errors_file" >&2
  fail "ripgrep failed while checking for application CREATE EXTENSION calls"
fi

if [ -s "$extension_calls_file" ]; then
  cat "$extension_calls_file" >&2
  fail "application code must not run CREATE EXTENSION"
fi

require_grep 'valkey/valkey:8-alpine' "$compose_file" "valkey must use the alpine Valkey image"
require_grep 'valkey-server' "$compose_file" "valkey must start through the image entrypoint server path"
require_grep '/usr/local/etc/valkey/valkey.conf' "$compose_file" "valkey must use a rendered config file"
require_grep 'OK_FOLIO_VALKEY_CONFIG_HOST_PATH.*:/usr/local/etc/valkey/valkey.conf:ro' "$compose_file" "valkey config must be mounted read-only"
require_grep 'requirepass \$\{VALKEY_PASSWORD\}' "$valkey_template" "valkey config template must require a password"
require_grep 'appendonly yes' "$valkey_template" "valkey config template must enable appendonly persistence"
require_grep 'valkey-cli.*ping' "$compose_file" "valkey must have a ping healthcheck"

require_grep 'ok-folio:\$\{OK_FOLIO_IMAGE_SHA:\?' "$compose_file" "app image must be pinned by OK_FOLIO_IMAGE_SHA"
require_grep 'condition: service_healthy' "$compose_file" "app must depend on healthy services"
require_grep 'external: true' "$compose_file" "legacy network must be external"

if grep -Eq 'DATABASE_URL' "$compose_file"; then
  fail "compose must not render DATABASE_URL alongside discrete DB_* settings"
fi

legacy_mount_patterns=(
  'PHOTO_ORIGINALS_HOST_PATH.*:/photoprism/originals:ro'
  'PHOTO_DAILY_HOST_PATH.*:/photoprism/_daily:ro'
  'PHOTOPRISM_STORAGE_HOST_PATH.*:/photoprism/storage:ro'
)
for pattern in "${legacy_mount_patterns[@]}"; do
  require_grep "$pattern" "$compose_file" "legacy mount must end in :ro: $pattern"
done

while IFS= read -r line; do
  if [[ ! "$line" =~ :ro[[:space:]]*$ ]]; then
    fail "found a legacy mount without a trailing :ro: $line"
  fi
done < <(grep -E '\$\{(PHOTO_ORIGINALS_HOST_PATH|PHOTO_DAILY_HOST_PATH|PHOTOPRISM_STORAGE_HOST_PATH)' "$compose_file")

if grep -Eq '([0-9]{1,3}\.){3}[0-9]{1,3}|/mnt/|/tank/|/pool/|/var/lib/docker|/home/' "$compose_file"; then
  fail "compose template must not contain concrete IPs or host paths"
fi

echo "ok-folio stack template check passed"

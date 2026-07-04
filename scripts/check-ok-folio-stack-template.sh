#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/deploy/dockhand/ok-folio/compose.yaml"
legacy_override_file="$repo_root/deploy/dockhand/ok-folio/compose.legacy.yaml"
legacy_storage_override_file="$repo_root/deploy/dockhand/ok-folio/compose.legacy-storage.yaml"
initdb_file="$repo_root/deploy/dockhand/ok-folio/initdb/010-vector-extensions.sh"
valkey_template="$repo_root/deploy/dockhand/ok-folio/valkey.conf.template"
config_template="$repo_root/deploy/dockhand/ok-folio/config.yaml.template"

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
[ -f "$legacy_override_file" ] || fail "missing $legacy_override_file"
[ -f "$legacy_storage_override_file" ] || fail "missing $legacy_storage_override_file"
[ -f "$initdb_file" ] || fail "missing $initdb_file"
[ -f "$valkey_template" ] || fail "missing $valkey_template"
[ -f "$config_template" ] || fail "missing $config_template"

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
require_grep 'OK_FOLIO_DERIVATIVES_HOST_PATH.*:/derivatives[[:space:]]*$' "$compose_file" "app must mount the writable derivatives cache at /derivatives"
require_grep 'OK_FOLIO_CONFIG_HOST_PATH.*:/config/config.yaml:ro' "$compose_file" "app config must be mounted read-only"

# The normal runtime must boot without legacy DB env or the external legacy
# Docker network. No LEGACY_* variable (LEGACY_DB_*, LEGACY_DOCKER_NETWORK, or any
# future/misspelled legacy-prefixed knob) may be a mandatory (?) substitution in
# the base compose, and the base compose must not attach the app to an external
# network. Match the LEGACY_ prefix generally rather than an allowlist so a new
# required legacy var cannot silently re-tie normal app boot to the legacy stack.
if grep -Eq -- '\$\{LEGACY_[A-Z0-9_]*:?\?' "$compose_file"; then
  grep -Eno -- '\$\{LEGACY_[A-Z0-9_]*:?\?' "$compose_file" >&2
  fail "no LEGACY_* variable may be a required (?) variable in the normal runtime compose (legacy DB env and the legacy network are an ETL/admin override, not an app boot requirement)"
fi
if grep -Eq -- 'external:[[:space:]]*true' "$compose_file"; then
  fail "normal runtime compose must not require an external network; keep legacy connectivity in compose.legacy.yaml"
fi

# Legacy connectivity is isolated behind the explicit ETL/admin override. It must
# stay external (never a stack-managed network) and keep the read-only legacy DB
# credentials scoped to that override only.
require_grep 'external:[[:space:]]*true' "$legacy_override_file" "legacy admin override must keep the legacy network external"
require_grep 'name:[[:space:]]*\$\{LEGACY_DOCKER_NETWORK:\?' "$legacy_override_file" "legacy admin override must name the external legacy network from LEGACY_DOCKER_NETWORK"

require_grep 'base_url:[[:space:]]*"https://sight\.photo/photos/category/15/"' "$config_template" "runtime config template must scrape the sight.photo category listing"
require_grep 'category_id:[[:space:]]*15' "$config_template" "runtime config template must set the sight.photo category id"
require_grep '^logging:' "$config_template" "runtime config template must include logging settings"
require_grep 'level:[[:space:]]*"info"' "$config_template" "runtime config template must enable info logging"
require_grep '^download:' "$config_template" "runtime config template must include download settings"
require_grep 'concurrent_limit:[[:space:]]*[1-9][0-9]*' "$config_template" "runtime config template must set a non-zero download concurrency"
require_grep 'timeout:[[:space:]]*[1-9][0-9]*s' "$config_template" "runtime config template must set a download timeout"
require_grep 'delay_between:[[:space:]]*[1-9][0-9]*s' "$config_template" "runtime config template must set a download delay"
require_grep 'rate_limit_backoff:[[:space:]]*[1-9][0-9]*s' "$config_template" "runtime config template must set a rate-limit backoff"
require_grep 'user_agent:[[:space:]]*"OK-Folio/' "$config_template" "runtime config template must set an OK Folio user agent"
require_grep 'chat_id:[[:space:]]*"\$\{TELEGRAM_CHAT_ID\}"' "$config_template" "runtime config template must scope Telegram ingestion to a rendered chat id"

if grep -Eq 'DATABASE_URL' "$compose_file"; then
  fail "compose must not render DATABASE_URL alongside discrete DB_* settings"
fi

require_grep 'PHOTO_ORIGINALS_HOST_PATH.*:/photoprism/originals[[:space:]]*$' "$compose_file" "originals mount must be writable"
require_grep 'PHOTO_DAILY_HOST_PATH.*:/photoprism/_daily[[:space:]]*$' "$compose_file" "daily mount must be writable"

# The legacy PhotoPrism storage/thumb fallback is optional. Normal runtime must
# boot and serve thumbnails without it, so the base compose must neither require
# the PHOTOPRISM_STORAGE_HOST_PATH variable nor mount /photoprism/storage. The
# mount lives only in the opt-in compose.legacy-storage.yaml override.
if grep -Eq -- '\$\{PHOTOPRISM_STORAGE_HOST_PATH:\?' "$compose_file"; then
  fail "normal runtime compose must not require PHOTOPRISM_STORAGE_HOST_PATH; keep the legacy storage fallback in the opt-in compose.legacy-storage.yaml override"
fi
if grep -Eq -- '^[[:space:]]*-[[:space:]].*:/photoprism/storage' "$compose_file"; then
  fail "normal runtime compose must not mount /photoprism/storage; the legacy storage fallback is opt-in via compose.legacy-storage.yaml"
fi

# The opt-in legacy storage override mounts the fallback read-only and points the
# app at it so the measured legacy_storage thumbnail tier is only active while the
# override is applied.
require_grep 'PHOTOPRISM_STORAGE_HOST_PATH.*:/photoprism/storage:ro' "$legacy_storage_override_file" "legacy storage override must mount /photoprism/storage read-only"
require_grep 'OK_FOLIO_LEGACY_THUMB_DIR:[[:space:]]*/photoprism/storage' "$legacy_storage_override_file" "legacy storage override must point OK_FOLIO_LEGACY_THUMB_DIR at the read-only mount"

# Any /photoprism/storage mount in any compose file must be kernel-enforced
# read-only. This keeps rejecting a writable legacy storage mount even though the
# base runtime no longer mounts it at all.
for template in "$compose_file" "$legacy_override_file" "$legacy_storage_override_file"; do
  writable_storage="$(grep -E -- '^[[:space:]]*-[[:space:]].*:/photoprism/storage' "$template" | grep -Ev -- ':/photoprism/storage:ro([[:space:]]*$|[[:space:]]+#)' || true)"
  if [ -n "$writable_storage" ]; then
    echo "$writable_storage" >&2
    fail "legacy storage mount must be read-only (:ro) in $template"
  fi
done

for template in "$compose_file" "$legacy_override_file" "$legacy_storage_override_file"; do
  if grep -Eq '([0-9]{1,3}\.){3}[0-9]{1,3}|/mnt/|/tank/|/pool/|/var/lib/docker|/home/' "$template"; then
    fail "compose template must not contain concrete IPs or host paths: $template"
  fi
done

echo "ok-folio stack template check passed"

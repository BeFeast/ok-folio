#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <rendered-compose.yaml>" >&2
  exit 2
fi

rendered_compose="$1"
[ -f "$rendered_compose" ] || {
  echo "rendered compose file not found: $rendered_compose" >&2
  exit 2
}

fail() {
  echo "legacy mount read-only assertion failed: $*" >&2
  exit 1
}

targets=(
  '/photoprism/originals'
  '/photoprism/_daily'
  '/photoprism/storage'
)

for target in "${targets[@]}"; do
  if grep -Eq -- "^[[:space:]]*-[[:space:]]*:${target}:ro([[:space:]]*$|[[:space:]]+#)" "$rendered_compose"; then
    fail "$target has an empty host source"
  fi
  bad_mounts="$(grep -F -- "$target" "$rendered_compose" | grep -Ev -- ":${target}:ro([[:space:]]*$|[[:space:]]+#)" || true)"
  if [ -n "$bad_mounts" ]; then
    echo "$bad_mounts" >&2
    fail "$target is mounted without a trailing :ro"
  fi
done

echo "rendered legacy mount read-only assertion passed"

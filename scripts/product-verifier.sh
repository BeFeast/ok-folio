#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [ -f dashboard/package.json ]; then
  (cd dashboard && npm ci && npm run build)
  rm -rf internal/dashboard/dist
  cp -R dashboard/dist internal/dashboard/dist
fi

go test ./...

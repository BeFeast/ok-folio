# Registry Image Release Runbook

OK Folio registry images are released manually from GitHub Actions. The registry has tag deletion disabled, so immutable commit tags must only be pushed after the image has passed a local smoke test.

## GitHub Actions Workflow

- Workflow: `.github/workflows/release-image.yml`
- Trigger: `workflow_dispatch` only
- Image repository: `${REGISTRY_URL}/ok-folio`
- Tags pushed from each run:
  - `${GITHUB_SHA}` for the immutable deploy pin
  - `dev` for the latest development image

The workflow builds the repository `Dockerfile` with Docker Buildx, loads the smoke image into the runner Docker daemon, runs the container locally, checks that `http://127.0.0.1:18080/health` reports `status: healthy` and `database: connected`, checks that `${GITHUB_SHA}` does not already exist in the registry, and only then tags and pushes the same image content as `${GITHUB_SHA}` and `dev`.

Do not rerun the workflow for a commit after changing the Docker build context outside that commit. The workflow refuses to push when the `${GITHUB_SHA}` tag already exists because commit tags must never point at different content.

## Required Secrets

Create these repository Actions secrets in GitHub:

- `REGISTRY_URL`: registry host, without a repository name or path component
- `REGISTRY_USERNAME`: push-scoped registry user
- `REGISTRY_PASSWORD`: push-scoped registry password or token

Only GitHub Actions stores the runtime secret values used by the workflow. Do not commit secret values, `.env` files, downloaded registry credentials, or copied vault output.

## Push Credential Scope

Use a registry account or token scoped to push OK Folio images and read the same repository when the registry supports repository-level permissions. It must not be an administrator or delete-capable account.

Credential inventory:

- Vault/Infisical path: `/ok-folio/github-actions/registry-push`
- GitHub secret names: `REGISTRY_URL`, `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`
- Rotation: rotate the registry token/password in the vault first, update the GitHub repository secrets, then run the workflow manually for a known commit and confirm `/ok-folio:<sha>` appears in the registry tag list.

## Manual Release Procedure

1. Open the `Release image` workflow in GitHub Actions.
2. Run it manually from the target branch or commit.
3. Confirm the smoke step prints a healthy `/health` response before the login and push steps.
4. Confirm the registry tag list contains the new commit SHA tag and the updated `dev` tag.
5. Give the dedicated-stack change the immutable `${GITHUB_SHA}` tag. Do not deploy from `dev`.
6. Use `docs/dedicated-dockhand-stack-runbook.md` for the Dockhand-only stack deploy contract. Do not deploy with manual `docker compose up`.

## Builder LXC Fallback

If GitHub Actions cannot reach the registry, use the same tag scheme from a trusted builder LXC that has Docker Buildx, `curl`, `jq`, and push-scoped registry credentials:

```bash
set -euo pipefail

git fetch origin
git checkout <commit-sha>
registry="<registry-host>"
while [ "${registry%/}" != "$registry" ]; do
  registry="${registry%/}"
done
if [ -z "$registry" ] || [ "$registry" != "${registry%%/*}" ] || [[ "$registry" == *"?"* || "$registry" == *"#"* ]]; then
  echo "registry must contain only the registry host, without a repository name or path component" >&2
  exit 1
fi
image="$registry/ok-folio"
sha="$(git rev-parse HEAD)"
container_name="ok-folio-smoke"
health_log="$(mktemp)"
inspect_log=""
logged_in=0

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  if [ "$logged_in" -eq 1 ]; then
    docker logout "$registry" >/dev/null 2>&1 || true
  fi
  if [ -n "$inspect_log" ]; then
    rm -f "$inspect_log"
  fi
  rm -f "$health_log"
}
trap cleanup EXIT

if [ -n "$(git status --porcelain)" ]; then
  echo "Refusing to build an immutable commit tag from a dirty working tree" >&2
  git status --short >&2
  exit 1
fi

docker buildx build --load -t "$image:smoke-$sha" .
docker run -d --name "$container_name" --network host -v "$PWD/config.smoke.yaml:/config/config.yaml:ro" "$image:smoke-$sha"
for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:18080/health >"$health_log" &&
    jq -e '.status == "healthy" and .database == "connected"' "$health_log" >/dev/null; then
    cat "$health_log"
    break
  fi
  sleep 2
done
if ! jq -e '.status == "healthy" and .database == "connected"' "$health_log" >/dev/null 2>&1; then
  echo "Smoke test did not receive a healthy response with a connected database" >&2
  cat "$health_log" >&2 || true
  docker logs "$container_name" >&2
  exit 1
fi

docker login "$registry"
logged_in=1
inspect_log="$(mktemp)"
if docker manifest inspect "$image:$sha" >"$inspect_log" 2>&1; then
  echo "Immutable image tag already exists: $image:$sha" >&2
  exit 1
fi
if ! grep -Eiq '(manifest unknown|manifest.*not found|no such manifest|not found|name unknown)' "$inspect_log"; then
  echo "Could not verify whether immutable image tag exists: $image:$sha" >&2
  cat "$inspect_log" >&2
  exit 1
fi
docker tag "$image:smoke-$sha" "$image:$sha"
docker tag "$image:smoke-$sha" "$image:dev"
docker push "$image:$sha"
docker push "$image:dev"
```

The fallback smoke config must use disposable local database credentials and must not be copied from live runtime files.

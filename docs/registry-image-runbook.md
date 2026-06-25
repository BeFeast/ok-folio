# Registry Image Release Runbook

OK Folio registry images are released manually from GitHub Actions. The registry has tag deletion disabled, so immutable commit tags must only be pushed after the image has passed a local smoke test.

## GitHub Actions Workflow

- Workflow: `.github/workflows/release-image.yml`
- Trigger: `workflow_dispatch` only
- Image repository: `${REGISTRY_URL}/ok-folio`
- Tags pushed from each run:
  - `${GITHUB_SHA}` for the immutable deploy pin
  - `dev` for the latest development image

The workflow builds the repository `Dockerfile` with Docker Buildx, loads the smoke image into the runner Docker daemon, runs the container locally, curls `http://127.0.0.1:18080/health`, and only then tags and pushes the same image content as `${GITHUB_SHA}` and `dev`.

Do not rerun the workflow for a commit after changing the Docker build context outside that commit. The `${GITHUB_SHA}` tag must never point at different content.

## Required Secrets

Create these repository Actions secrets in GitHub:

- `REGISTRY_URL`: registry host, without a repository name
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

## Builder LXC Fallback

If GitHub Actions cannot reach the registry, use the same tag scheme from a trusted builder LXC that has Docker Buildx and push-scoped registry credentials:

```bash
git fetch origin
git checkout <commit-sha>
registry="<registry-host>"
image="$registry/ok-folio"
sha="$(git rev-parse HEAD)"

docker buildx build --load -t "$image:smoke-$sha" .
docker run --rm -d --name ok-folio-smoke --network host -v "$PWD/config.smoke.yaml:/config/config.yaml:ro" "$image:smoke-$sha"
curl -fsS http://127.0.0.1:18080/health
docker rm -f ok-folio-smoke

docker login "$registry"
docker tag "$image:smoke-$sha" "$image:$sha"
docker tag "$image:smoke-$sha" "$image:dev"
docker push "$image:$sha"
docker push "$image:dev"
```

The fallback smoke config must use disposable local database credentials and must not be copied from live runtime files.

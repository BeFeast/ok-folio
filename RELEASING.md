# Releasing OK Folio

OK Folio versions follow [SemVer 2.0.0](https://semver.org/): `vMAJOR.MINOR.PATCH`.

- **MAJOR** — incompatible API / data-schema changes that need a migration.
- **MINOR** — backward-compatible features (new connectors, gallery surfaces, endpoints).
- **PATCH** — backward-compatible fixes.
- Pre-releases use a suffix, e.g. `v0.1.0-rc.1`.

V1 is pre-1.0 (`0.y.z`): minor versions may still carry breaking changes while the
data model and connector set stabilize.

## Cut a release

1. Make sure `main` is green (CI) and builds a working image (`docker build .`).
2. Tag the release commit and push the tag:
   ```bash
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```
3. The `Release` workflow (`.github/workflows/release.yml`) fires on the `v*` tag and
   creates the matching **GitHub Release** with auto-generated notes.

## Container image

Build the deployment image on a machine with access to your container registry.
Set `REGISTRY_HOST` before tagging and pushing the release image.

```bash
SHA=$(git rev-parse --short HEAD)            # commit the tag points at
docker build -t ${REGISTRY_HOST:?Set your container registry}/ok-folio:v0.1.0 \
             -t ${REGISTRY_HOST:?Set your container registry}/ok-folio:${SHA} .
docker push ${REGISTRY_HOST:?Set your container registry}/ok-folio:v0.1.0
docker push ${REGISTRY_HOST:?Set your container registry}/ok-folio:${SHA}
```

The Dockhand stack pins the immutable `:vX.Y.Z` tag. The `release-image.yml`
(`workflow_dispatch`) workflow is the GitHub-side stub for a future externally
reachable / authenticated registry; until then, build on the LAN.

## Conventions

- One release per `main` commit that ships user-visible change; batch Dependabot/chore
  commits into the next minor/patch.
- Release notes are auto-generated from merged PR titles — keep PR titles descriptive
  (the repo already uses conventional-ish prefixes: `feat:`, `fix:`, `chore:`, `ci:`).

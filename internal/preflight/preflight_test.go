package preflight

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// repoRoot resolves the OK Folio repo root from the test working directory
// (the package dir), so the offline checks run against the real committed
// files.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := FindRepoRoot(".")
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	return root
}

func TestRunAgainstRepoHasNoFailures(t *testing.T) {
	report, err := Run(Options{RepoRoot: repoRoot(t)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Results) != len(offlineChecks) {
		t.Fatalf("expected %d results, got %d", len(offlineChecks), len(report.Results))
	}
	for _, res := range report.Results {
		if res.Status == StatusFail {
			t.Errorf("check %s unexpectedly FAILed: %s\n%v", res.ID, res.Summary, res.Evidence)
		}
		if res.Status == "" || res.Summary == "" {
			t.Errorf("check %s has empty status/summary", res.ID)
		}
	}
	if report.Failed() {
		t.Errorf("report.Failed() = true; expected the current repo to pass all offline checks")
	}
}

func TestKnownDecoupledChecksPass(t *testing.T) {
	root := repoRoot(t)
	// These reflect merged phases (C1/C2 decoupling, C3 maintenance split) and
	// must be green in the current tree.
	for _, id := range []string{
		"normal-stack-decoupled",
		"photoprism-indexing-gated",
		"maintenance-commands-decoupled",
		"connector-state-surfaces",
	} {
		res := runOne(t, root, id)
		if res.Status != StatusPass {
			t.Errorf("%s = %s, want PASS (%s)", id, res.Status, res.Summary)
		}
		if len(res.Evidence) == 0 {
			t.Errorf("%s passed without citing evidence", id)
		}
	}
}

func TestDerivativeFallbackIsPendingUntilC4(t *testing.T) {
	// C4 is not merged in this tree; the check must report PENDING (not FAIL).
	res := runOne(t, repoRoot(t), "derivative-fallback-measured")
	if res.Status != StatusPending && res.Status != StatusPass {
		t.Fatalf("derivative-fallback-measured = %s, want PENDING or PASS", res.Status)
	}
	if res.Status == StatusFail {
		t.Fatalf("derivative-fallback-measured must never FAIL before C4 merges")
	}
}

func runOne(t *testing.T, root, id string) Result {
	t.Helper()
	report, err := Run(Options{RepoRoot: root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, res := range report.Results {
		if res.ID == id {
			return res
		}
	}
	t.Fatalf("no check with id %q", id)
	return Result{}
}

// --- Fixture-based checks for the FAIL / PENDING paths ---

// writeFixtureRepo builds a minimal repo tree with the given file contents plus
// a go.mod so FindRepoRoot and Run resolve it.
func writeFixtureRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module ok-folio\n\ngo 1.25\n"
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

func TestNormalStackDecoupledFailsOnRequiredLegacyVar(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		composePath: "services:\n  app:\n    environment:\n      LEGACY_DB_HOST: ${LEGACY_DB_HOST:?required}\n    volumes:\n      - X:/photoprism/storage:ro\n",
	})
	res := checkNormalStackDecoupled(root)
	if res.Status != StatusFail {
		t.Fatalf("expected FAIL for required LEGACY_* var, got %s", res.Status)
	}
}

func TestNormalStackDecoupledFailsOnWritableLegacyStorage(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		composePath: "services:\n  app:\n    volumes:\n      - X:/photoprism/storage\n",
	})
	res := checkNormalStackDecoupled(root)
	if res.Status != StatusFail {
		t.Fatalf("expected FAIL for writable legacy storage, got %s", res.Status)
	}
}

func TestNormalStackDecoupledFailsOnExternalNetwork(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		composePath: "services:\n  app: {}\nnetworks:\n  legacy:\n    external: true\n",
	})
	res := checkNormalStackDecoupled(root)
	if res.Status != StatusFail {
		t.Fatalf("expected FAIL for external network, got %s", res.Status)
	}
}

func TestPhotoPrismIndexingGatedFailsWhenEnabled(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		configTemplatePath: "photoprism:\n  enabled: true\n  auto_index: false\n",
		configExamplePath:  "photoprism:\n  enabled: false\n  auto_index: false\n",
	})
	res := checkPhotoPrismIndexingGated(root)
	if res.Status != StatusFail {
		t.Fatalf("expected FAIL when photoprism.enabled defaults true, got %s", res.Status)
	}
}

func TestMaintenanceCommandsPendingWhenAdminMissing(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		etlCmdPath: "package main\n",
	})
	res := checkMaintenanceCommandsDecoupled(root)
	if res.Status != StatusPending {
		t.Fatalf("expected PENDING when ok-folio-admin absent (C3 unmerged), got %s", res.Status)
	}
}

func TestMaintenanceCommandsWarnsWhenCommandMissing(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		adminCmdPath: "package main\n// hash-content warm-thumbnails audit-originals\n",
	})
	res := checkMaintenanceCommandsDecoupled(root)
	if res.Status != StatusWarn {
		t.Fatalf("expected WARN when a maintenance command is missing, got %s", res.Status)
	}
}

func TestDerivativeFallbackPassesWhenMarkerPresent(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		composePath:                        "services:\n  app:\n    volumes:\n      - X:/photoprism/storage:ro\n",
		derivativesDir + "/warmer.go":      "package derivatives\n// uses legacy storage fallback and increments a metric\n",
		derivativesDir + "/warmer_test.go": "package derivatives\n// fallback in a test file must be ignored\n",
	})
	res := checkDerivativeFallbackMeasured(root)
	if res.Status != StatusPass {
		t.Fatalf("expected PASS when derivatives references a fallback, got %s (%s)", res.Status, res.Summary)
	}
}

func TestDerivativeFallbackPendingWhenNoMarker(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		composePath:                   "services: {}\n",
		derivativesDir + "/warmer.go": "package derivatives\n// no marker here\n",
	})
	res := checkDerivativeFallbackMeasured(root)
	if res.Status != StatusPending {
		t.Fatalf("expected PENDING when no fallback marker, got %s", res.Status)
	}
}

func TestDerivativeFallbackPendingWhenStorageStillRequired(t *testing.T) {
	// A read-only mount that is still a required (?) substitution is NOT
	// optional: the normal stack cannot boot without the legacy storage path.
	// Even with a fallback marker present the check must stay PENDING, never a
	// false PASS/READY.
	root := writeFixtureRepo(t, map[string]string{
		composePath:                   "services:\n  app:\n    volumes:\n      - ${PHOTOPRISM_STORAGE_HOST_PATH:?PHOTOPRISM_STORAGE_HOST_PATH is required}:/photoprism/storage:ro\n",
		derivativesDir + "/warmer.go": "package derivatives\n// uses legacy storage fallback and increments a metric\n",
	})
	res := checkDerivativeFallbackMeasured(root)
	if res.Status != StatusPending {
		t.Fatalf("expected PENDING while legacy storage is still a required substitution, got %s (%s)", res.Status, res.Summary)
	}
}

func TestConnectorStateSurfacesFailsWhenMissing(t *testing.T) {
	root := writeFixtureRepo(t, map[string]string{
		streamsPath:    "package api\n// nothing here\n",
		webgalleryPath: "package webgallery\n",
	})
	res := checkConnectorStateSurfaces(root)
	if res.Status != StatusFail {
		t.Fatalf("expected FAIL when streams does not surface connector state, got %s", res.Status)
	}
}

// --- Live probe ---

func TestProbeConnectorsPass(t *testing.T) {
	now := time.Now().UTC()
	body := map[string]any{
		"connectors": []map[string]any{
			{
				"id":        "webgallery",
				"last_sync": now,
				"sources": []map[string]any{
					{"id": "webgallery:1", "provider_id": "webgallery:1"},
				},
			},
			{"id": "telegram", "last_sync": now, "sources": []map[string]any{}},
		},
	}
	srv := connectorStubServer(t, body)
	defer srv.Close()

	res := ProbeConnectors(context.Background(), srv.Client(), srv.URL)
	if res.Status != StatusPass {
		t.Fatalf("live probe = %s, want PASS (%s)", res.Status, res.Summary)
	}
}

func TestProbeConnectorsWarnsWhenWebgalleryMissing(t *testing.T) {
	body := map[string]any{
		"connectors": []map[string]any{
			{"id": "telegram", "last_sync": nil, "sources": []map[string]any{}},
		},
	}
	srv := connectorStubServer(t, body)
	defer srv.Close()

	res := ProbeConnectors(context.Background(), srv.Client(), srv.URL)
	if res.Status != StatusWarn {
		t.Fatalf("live probe = %s, want WARN when webgallery id absent", res.Status)
	}
}

func TestProbeConnectorsWarnsWhenOnlyAggregateWebgallery(t *testing.T) {
	// A source that only exposes the bare aggregate provider id "webgallery"
	// (no per-source :<id> suffix) must NOT satisfy the promised webgallery:<id>
	// check; the probe must WARN, not PASS.
	now := time.Now().UTC()
	body := map[string]any{
		"connectors": []map[string]any{
			{
				"id":        "webgallery",
				"last_sync": now,
				"sources": []map[string]any{
					{"id": "webgallery", "provider_id": "webgallery"},
				},
			},
			{"id": "telegram", "last_sync": now, "sources": []map[string]any{}},
		},
	}
	srv := connectorStubServer(t, body)
	defer srv.Close()

	res := ProbeConnectors(context.Background(), srv.Client(), srv.URL)
	if res.Status != StatusWarn {
		t.Fatalf("live probe = %s, want WARN when only the aggregate webgallery id is surfaced (%s)", res.Status, res.Summary)
	}
}

func TestProbeConnectorsWarnsWhenUnreachable(t *testing.T) {
	res := ProbeConnectors(context.Background(), &http.Client{Timeout: time.Second}, "http://127.0.0.1:0")
	if res.Status != StatusWarn {
		t.Fatalf("live probe against dead host = %s, want WARN", res.Status)
	}
}

func connectorStubServer(t *testing.T, body map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("live probe must be read-only GET, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != ConnectorStatusPath {
			t.Errorf("unexpected probe path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
}

package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Repo-relative paths the offline checks read. Kept in one place so the paths
// the verifier depends on are easy to audit.
const (
	composePath        = "deploy/dockhand/ok-folio/compose.yaml"
	composeLegacyPath  = "deploy/dockhand/ok-folio/compose.legacy.yaml"
	configTemplatePath = "deploy/dockhand/ok-folio/config.yaml.template"
	configExamplePath  = "config.example.yaml"
	adminCmdPath       = "cmd/ok-folio-admin/main.go"
	etlCmdPath         = "cmd/ok-folio-etl/main.go"
	streamsPath        = "internal/api/streams.go"
	photoIndexPath     = "internal/api/search.go"
	webgalleryPath     = "internal/provider/webgallery/connector.go"
	derivativesDir     = "internal/derivatives"
	legacyRunbookPath  = "docs/legacy-etl-runbook.md"
	dockhandRunbook    = "docs/dedicated-dockhand-stack-runbook.md"
)

// requiredLegacyVarRe matches any LEGACY_-prefixed compose variable used as a
// mandatory (?) substitution, e.g. ${LEGACY_DB_HOST:?...} or ${LEGACY_X?...}.
var requiredLegacyVarRe = regexp.MustCompile(`\$\{LEGACY_[A-Z0-9_]*:?\?`)

// externalNetworkRe matches an external Docker network declaration.
var externalNetworkRe = regexp.MustCompile(`external:\s*true`)

// checkNormalStackDecoupled verifies the normal runtime template boots without
// legacy DB env, the external legacy network, or writable legacy storage.
func checkNormalStackDecoupled(root string) Result {
	res := Result{ID: "normal-stack-decoupled", Title: "Normal stack boots without legacy DB env, legacy network, or writable legacy storage"}

	compose, err := readRepoFile(root, composePath)
	if err != nil {
		res.Status = StatusFail
		res.Summary = fmt.Sprintf("cannot read %s: %v", composePath, err)
		return res
	}

	var failures []string
	if requiredLegacyVarRe.MatchString(compose) {
		failures = append(failures, fmt.Sprintf("%s: a LEGACY_* variable is a required (?) substitution in the normal runtime", composePath))
	} else {
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: no required ${LEGACY_*:?} substitution", composePath))
	}

	if externalNetworkRe.MatchString(compose) {
		failures = append(failures, fmt.Sprintf("%s: normal runtime attaches an external network", composePath))
	} else {
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: no `external: true` network", composePath))
	}

	if bad := legacyStorageWritableMounts(compose); len(bad) > 0 {
		failures = append(failures, fmt.Sprintf("%s: legacy storage mounted without :ro (%s)", composePath, strings.Join(bad, "; ")))
	} else if strings.Contains(compose, "/photoprism/storage") {
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: legacy storage mounted read-only (:/photoprism/storage:ro)", composePath))
	}

	// Supporting evidence: the legacy DB env and network stay isolated behind
	// the explicit ETL/admin override, not the normal runtime.
	if legacy, err := readRepoFile(root, composeLegacyPath); err == nil {
		if externalNetworkRe.MatchString(legacy) && strings.Contains(legacy, "LEGACY_DOCKER_NETWORK") {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: legacy DB env + network isolated to the opt-in ETL/admin override", composeLegacyPath))
		}
	}

	if len(failures) > 0 {
		res.Status = StatusFail
		res.Summary = "normal runtime still depends on legacy stack"
		res.Evidence = failures
		return res
	}
	res.Status = StatusPass
	res.Summary = "normal runtime is decoupled from the legacy DB env, network, and writable legacy storage"
	return res
}

// legacyStorageWritableMounts returns legacy /photoprism/storage mount lines
// that are not marked read-only (:ro).
func legacyStorageWritableMounts(compose string) []string {
	var bad []string
	for _, line := range strings.Split(compose, "\n") {
		if !strings.Contains(line, "/photoprism/storage") {
			continue
		}
		if !strings.Contains(line, "/photoprism/storage:ro") {
			bad = append(bad, strings.TrimSpace(line))
		}
	}
	return bad
}

// checkPhotoPrismIndexingGated verifies PhotoPrism indexing is disabled by
// default in the rendered templates and gated behind an explicit opt-in.
func checkPhotoPrismIndexingGated(root string) Result {
	res := Result{ID: "photoprism-indexing-gated", Title: "PhotoPrism indexing is disabled / admin-gated by default"}

	var failures []string
	for _, path := range []string{configTemplatePath, configExamplePath} {
		content, err := readRepoFile(root, path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot read: %v", path, err))
			continue
		}
		block := photoprismBlock(content)
		if block == "" {
			failures = append(failures, fmt.Sprintf("%s: no photoprism: block found", path))
			continue
		}
		if !hasYAMLBool(block, "enabled", false) {
			failures = append(failures, fmt.Sprintf("%s: photoprism.enabled is not false by default", path))
		}
		if !hasYAMLBool(block, "auto_index", false) {
			failures = append(failures, fmt.Sprintf("%s: photoprism.auto_index is not false by default", path))
		}
		if len(failures) == 0 {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: photoprism.enabled=false, photoprism.auto_index=false", path))
		}
	}

	// Supporting evidence: the admin index route is a gated escape hatch that
	// returns a disabled response unless photoprism.enabled is set.
	if handler, err := readRepoFile(root, photoIndexPath); err == nil {
		if strings.Contains(handler, "if !s.cfg.PhotoPrism.Enabled") {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: admin index route rejects when photoprism.enabled is false", photoIndexPath))
		}
	}

	if len(failures) > 0 {
		res.Status = StatusFail
		res.Summary = "PhotoPrism indexing is not disabled by default"
		res.Evidence = failures
		return res
	}
	res.Status = StatusPass
	res.Summary = "PhotoPrism indexing defaults to off and stays out of the normal product path"
	return res
}

// checkMaintenanceCommandsDecoupled verifies ongoing maintenance commands live
// in the non-legacy ok-folio-admin CLI (Phase C3). When C3 has not merged, the
// admin binary is absent and the check reports PENDING instead of failing.
func checkMaintenanceCommandsDecoupled(root string) Result {
	res := Result{ID: "maintenance-commands-decoupled", Title: "Maintenance commands are available outside legacy ETL (Phase C3)"}

	if !fileExists(root, adminCmdPath) {
		res.Status = StatusPending
		res.Summary = "Phase C3 (split maintenance commands into ok-folio-admin) is not merged yet"
		res.Evidence = []string{fmt.Sprintf("%s: not present", adminCmdPath)}
		return res
	}

	admin, err := readRepoFile(root, adminCmdPath)
	if err != nil {
		res.Status = StatusFail
		res.Summary = fmt.Sprintf("cannot read %s: %v", adminCmdPath, err)
		return res
	}

	wanted := []string{"hash-content", "warm-thumbnails", "audit-originals", "smoke-read-paths"}
	var missing []string
	for _, cmd := range wanted {
		if !strings.Contains(admin, cmd) {
			missing = append(missing, cmd)
		}
	}
	if len(missing) > 0 {
		res.Status = StatusWarn
		res.Summary = "ok-folio-admin is present but some maintenance commands are missing"
		res.Evidence = []string{fmt.Sprintf("%s: missing %s", adminCmdPath, strings.Join(missing, ", "))}
		return res
	}
	res.Evidence = append(res.Evidence, fmt.Sprintf("%s: hosts %s", adminCmdPath, strings.Join(wanted, ", ")))

	// Supporting evidence: the legacy ETL CLI keeps only migration commands.
	if etl, err := readRepoFile(root, etlCmdPath); err == nil {
		if !strings.Contains(etl, "hash-content") && strings.Contains(etl, "load-dump") {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: retains only legacy migration commands (load-dump, print-legacy-checks)", etlCmdPath))
		}
	}

	res.Status = StatusPass
	res.Summary = "ongoing maintenance commands run from the non-legacy ok-folio-admin CLI"
	return res
}

// checkDerivativeFallbackMeasured verifies the legacy PhotoPrism storage
// fallback is measured in the derivative path (Phase C4). The storage mount is
// already optional/read-only today; the measurement lands with C4, so when the
// derivatives package carries no fallback measurement this reports PENDING.
func checkDerivativeFallbackMeasured(root string) Result {
	res := Result{ID: "derivative-fallback-measured", Title: "Legacy PhotoPrism storage fallback is optional and measured (Phase C4)"}

	// The optional/read-only half is already true regardless of C4.
	if compose, err := readRepoFile(root, composePath); err == nil {
		if strings.Contains(compose, "/photoprism/storage:ro") {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: legacy PhotoPrism storage is optional (mounted read-only)", composePath))
		}
	}

	markers := derivativeFallbackMarkers(root)
	if len(markers) == 0 {
		res.Status = StatusPending
		res.Summary = "Phase C4 (measured legacy storage fallback) is not merged yet"
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: no derivative fallback measurement found", derivativesDir))
		return res
	}
	res.Status = StatusPass
	res.Summary = "legacy storage fallback is optional and measured in the derivative path"
	res.Evidence = append(res.Evidence, markers...)
	return res
}

// derivativeFallbackMarkers returns evidence lines for any fallback measurement
// in the derivatives package (non-test Go sources).
func derivativeFallbackMarkers(root string) []string {
	dir := filepath.Join(root, filepath.FromSlash(derivativesDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var markers []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(data)), "fallback") {
			markers = append(markers, fmt.Sprintf("%s/%s: references a storage fallback", derivativesDir, name))
		}
	}
	return markers
}

// checkConnectorStateSurfaces verifies connector state surfaces per-source
// provider ids (e.g. webgallery:1) and Telegram freshness from durable
// connector state, not from the legacy extractor.
func checkConnectorStateSurfaces(root string) Result {
	res := Result{ID: "connector-state-surfaces", Title: "Connector state surfaces webgallery:1 and Telegram freshness without the legacy extractor"}

	streams, err := readRepoFile(root, streamsPath)
	if err != nil {
		res.Status = StatusFail
		res.Summary = fmt.Sprintf("cannot read %s: %v", streamsPath, err)
		return res
	}

	var failures []string
	if strings.Contains(streams, "GetConnectorStates") {
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: connector status reads durable connector_state (not the extractor)", streamsPath))
	} else {
		failures = append(failures, fmt.Sprintf("%s: connector status does not read GetConnectorStates", streamsPath))
	}
	if strings.Contains(streams, `"webgallery"`) && strings.Contains(streams, `"telegram"`) {
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: surfaces both webgallery and telegram connectors", streamsPath))
	} else {
		failures = append(failures, fmt.Sprintf("%s: does not surface both webgallery and telegram connectors", streamsPath))
	}
	if strings.Contains(streams, "LastSync") {
		res.Evidence = append(res.Evidence, fmt.Sprintf("%s: exposes per-connector last_sync freshness", streamsPath))
	} else {
		failures = append(failures, fmt.Sprintf("%s: does not expose last_sync freshness", streamsPath))
	}

	if wg, err := readRepoFile(root, webgalleryPath); err == nil {
		if strings.Contains(wg, `ProviderID + ":" + c.cfg.SourceID`) {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: emits per-source provider ids of the webgallery:<id> shape", webgalleryPath))
		} else {
			failures = append(failures, fmt.Sprintf("%s: does not emit per-source webgallery:<id> provider ids", webgalleryPath))
		}
	}

	if len(failures) > 0 {
		res.Status = StatusFail
		res.Summary = "connector state does not surface per-source ids and freshness"
		res.Evidence = failures
		return res
	}
	res.Status = StatusPass
	res.Summary = "connector state surfaces per-source ids and freshness independently of the legacy extractor"
	return res
}

// checkLegacyExtractorRetirementDocumented verifies the retirement state is
// documented as expected stopped/startable, so an operator can cite it without
// performing any lifecycle mutation.
func checkLegacyExtractorRetirementDocumented(root string) Result {
	res := Result{ID: "legacy-extractor-retirement-documented", Title: "Legacy retirement is documented as expected stopped/startable (no lifecycle mutation)"}

	documented := false
	if legacy, err := readRepoFile(root, composeLegacyPath); err == nil {
		if strings.Contains(legacy, "NOT part of the normal OK Folio runtime") {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: documents the legacy stack as an opt-in override, not an app dependency", composeLegacyPath))
			documented = true
		}
	}
	for _, path := range []string{dockhandRunbook, legacyRunbookPath} {
		content, err := readRepoFile(root, path)
		if err != nil {
			continue
		}
		if mentionsStoppedStartable(content) {
			res.Evidence = append(res.Evidence, fmt.Sprintf("%s: documents legacy services as stopped/startable fallback components", path))
			documented = true
		}
	}

	res.Evidence = append(res.Evidence, "preflight is read-only: it performs no docker/Dockhand/systemctl/Maestro lifecycle mutation")

	if !documented {
		res.Status = StatusWarn
		res.Summary = "no explicit stopped/startable retirement documentation found"
		return res
	}
	res.Status = StatusPass
	res.Summary = "retirement state is documented as expected stopped/startable"
	return res
}

func mentionsStoppedStartable(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "stopped/startable") ||
		strings.Contains(lower, "stopped-but-startable") ||
		strings.Contains(lower, "stopped but startable")
}

// photoprismBlock returns the indented YAML body under a top-level
// `photoprism:` key, or "" when absent.
func photoprismBlock(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "photoprism:") {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}
	var block []string
	for _, line := range lines[start+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			block = append(block, line)
			continue
		}
		// A new top-level key (no leading whitespace) ends the block.
		if line[0] != ' ' && line[0] != '\t' {
			break
		}
		block = append(block, line)
	}
	return strings.Join(block, "\n")
}

// hasYAMLBool reports whether the block sets `key: value` where value matches
// the wanted boolean, ignoring inline comments.
func hasYAMLBool(block, key string, want bool) bool {
	wantStr := "false"
	if want {
		wantStr = "true"
	}
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		prefix := key + ":"
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if idx := strings.Index(value, "#"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		return value == wantStr
	}
	return false
}

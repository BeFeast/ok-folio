// Package preflight implements the OK Folio Wave 6 legacy-retirement preflight
// verifier.
//
// The verifier is strictly read-only. It inspects committed repository state
// (rendered Dockhand templates, config templates, and source) to produce
// PASS/WARN/FAIL/PENDING evidence an operator or PM can cite before retiring
// remaining legacy services. It never mutates Docker lifecycle, Dockhand,
// systemd, or the Maestro control plane, never opens the legacy database, and
// requires no secret values for its offline checks.
//
// Offline checks read only committed, non-secret files (templates carry
// ${VAR} placeholders, not values). An optional, clearly separated live probe
// performs a single read-only HTTP GET against a running app's connector-status
// endpoint; it is never required for the offline pass.
package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Status is the outcome of a single preflight check.
type Status string

const (
	// StatusPass means the retirement assumption is verified.
	StatusPass Status = "PASS"
	// StatusWarn means a soft, non-blocking concern was found.
	StatusWarn Status = "WARN"
	// StatusFail means a retirement assumption is violated.
	StatusFail Status = "FAIL"
	// StatusPending means the check depends on a cutover phase (C3/C4) that has
	// not merged yet. It is reported rather than failing hard so in-flight work
	// is not blocked.
	StatusPending Status = "PENDING"
)

// Result is the outcome of one named check.
type Result struct {
	ID       string
	Title    string
	Status   Status
	Summary  string
	Evidence []string
}

// Report is the full set of check results from one run.
type Report struct {
	RepoRoot string
	Results  []Result
}

// Failed reports whether any offline check ended in FAIL. This is the signal a
// caller uses for a non-zero exit; PENDING and WARN never fail the run.
func (r Report) Failed() bool {
	for _, res := range r.Results {
		if res.Status == StatusFail {
			return true
		}
	}
	return false
}

// Counts returns the number of results in each status.
func (r Report) Counts() map[Status]int {
	counts := map[Status]int{StatusPass: 0, StatusWarn: 0, StatusFail: 0, StatusPending: 0}
	for _, res := range r.Results {
		counts[res.Status]++
	}
	return counts
}

// Options configures an offline preflight run.
type Options struct {
	// RepoRoot is the OK Folio repository root. When empty, Run resolves it by
	// walking up from the current working directory.
	RepoRoot string
}

// Run executes every offline retirement check and returns the aggregated
// report. It performs no network calls and mutates nothing.
func Run(opts Options) (Report, error) {
	root := opts.RepoRoot
	if strings.TrimSpace(root) == "" {
		found, err := FindRepoRoot(".")
		if err != nil {
			return Report{}, err
		}
		root = found
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return Report{}, fmt.Errorf("repo root %q does not contain go.mod: %w", root, err)
	}

	report := Report{RepoRoot: root}
	for _, check := range offlineChecks {
		report.Results = append(report.Results, check(root))
	}
	return report, nil
}

// offlineChecks is the ordered set of read-only retirement checks.
var offlineChecks = []func(root string) Result{
	checkNormalStackDecoupled,
	checkPhotoPrismIndexingGated,
	checkMaintenanceCommandsDecoupled,
	checkDerivativeFallbackMeasured,
	checkConnectorStateSurfaces,
	checkLegacyExtractorRetirementDocumented,
}

// FindRepoRoot walks up from start until it finds the directory whose go.mod
// declares `module ok-folio`.
func FindRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gomod); err == nil {
			if isOKFolioModule(data) {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find OK Folio repo root (module ok-folio) from %q", start)
		}
		dir = parent
	}
}

func isOKFolioModule(gomod []byte) bool {
	for _, line := range strings.Split(string(gomod), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")) == "ok-folio"
		}
	}
	return false
}

// readRepoFile reads a repo-relative file and returns its contents. A missing
// file is reported to the caller so a check can decide whether that means FAIL
// or PENDING.
func readRepoFile(root, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// fileExists reports whether a repo-relative path exists.
func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

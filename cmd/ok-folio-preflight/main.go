// Command ok-folio-preflight is the read-only Wave 6 legacy-retirement preflight
// verifier.
//
// It inspects committed repository state (rendered Dockhand templates, config
// templates, and source) and prints PASS/WARN/FAIL/PENDING evidence an operator
// or PM can cite before retiring remaining legacy services. It is strictly
// read-only: it never runs docker compose up/down/restart, Dockhand mutation,
// systemctl, or Maestro control-plane mutations, never opens the legacy
// database, and needs no secret values for its offline checks.
//
// An optional live probe (--live-connectors-url) performs a single read-only
// HTTP GET against a running app's connector-status endpoint. It is clearly
// separated from the offline checks and never required for the offline pass.
//
// Exit code is 0 unless an offline check FAILs; PENDING and WARN never fail the
// run so in-flight cutover phases (C3/C4) do not block development.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"ok-folio/internal/preflight"
)

func main() {
	repoRoot := flag.String("repo-root", "", "OK Folio repo root (default: auto-detect from the current directory)")
	liveURL := flag.String("live-connectors-url", "", "Optional: base URL of a running app to read-only probe connector status (e.g. http://host:8080)")
	liveTimeout := flag.Duration("live-timeout", 10*time.Second, "Timeout for the optional live connector probe")
	flag.Parse()

	report, err := preflight.Run(preflight.Options{RepoRoot: *repoRoot})
	if err != nil {
		fmt.Fprintln(os.Stderr, "preflight error:", err)
		os.Exit(2)
	}

	fmt.Println("OK Folio — Wave 6 legacy-retirement preflight")
	fmt.Println("Repo root:", report.RepoRoot)
	fmt.Println("Mode: read-only (no docker/Dockhand/systemctl/Maestro lifecycle mutation performed)")
	fmt.Println()

	fmt.Println("Offline checks:")
	for _, res := range report.Results {
		printResult(res)
	}

	if *liveURL != "" {
		fmt.Println()
		fmt.Println("Live probe (read-only GET " + preflight.ConnectorStatusPath + "):")
		ctx, cancel := context.WithTimeout(context.Background(), *liveTimeout)
		defer cancel()
		printResult(preflight.ProbeConnectors(ctx, nil, *liveURL))
	}

	fmt.Println()
	printSummary(report)

	if report.Failed() {
		os.Exit(1)
	}
}

func printResult(res preflight.Result) {
	fmt.Printf("[%s] %s: %s\n", res.Status, res.ID, res.Summary)
	for _, ev := range res.Evidence {
		fmt.Printf("       - %s\n", ev)
	}
}

func printSummary(report preflight.Report) {
	counts := report.Counts()
	fmt.Printf("Summary: %d PASS, %d WARN, %d FAIL, %d PENDING\n",
		counts[preflight.StatusPass],
		counts[preflight.StatusWarn],
		counts[preflight.StatusFail],
		counts[preflight.StatusPending],
	)

	switch {
	case counts[preflight.StatusFail] > 0:
		fmt.Println("Result: NOT READY — one or more retirement assumptions failed (see FAIL above)")
	case counts[preflight.StatusPending] > 0:
		fmt.Printf("Result: READY-WITH-PENDING — no offline check failed; %s\n", pendingList(report))
	default:
		fmt.Println("Result: READY — all offline retirement checks passed")
	}
}

func pendingList(report preflight.Report) string {
	var ids []string
	for _, res := range report.Results {
		if res.Status == preflight.StatusPending {
			ids = append(ids, res.ID)
		}
	}
	sort.Strings(ids)
	return fmt.Sprintf("%d pending on in-flight phases (%v)", len(ids), ids)
}

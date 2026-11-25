package legacyetl

import (
	"strings"
	"testing"
)

func TestDumpArgsUseLockFreeSnapshotContract(t *testing.T) {
	args, err := DumpArgs("legacy_okfolio", []string{DownloadedPhotosTable}, "id >= 42")
	if err != nil {
		t.Fatalf("DumpArgs failed: %v", err)
	}
	if err := ValidateDumpArgs(args); err != nil {
		t.Fatalf("expected generated dump args to pass: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, required := range []string{"--single-transaction", "--skip-lock-tables", "--no-create-info", "--no-tablespaces", "--compact"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("expected %s in %q", required, joined)
		}
	}
	if strings.Contains(joined, "--password") {
		t.Fatalf("dump command must not put passwords on the CLI: %q", joined)
	}
}

func TestValidateDumpArgsRejectsGlobalLockOptions(t *testing.T) {
	for _, forbidden := range []string{"--master-data", "--source-data", "--lock-all-tables"} {
		args, err := DumpArgs("legacy_okfolio", OwnedTables, "")
		if err != nil {
			t.Fatalf("DumpArgs failed: %v", err)
		}
		args = append(args, forbidden)
		if err := ValidateDumpArgs(args); err == nil {
			t.Fatalf("expected %s to be rejected", forbidden)
		}
	}
}

func TestDumpArgsRejectsNonOwnedTables(t *testing.T) {
	if _, err := DumpArgs("legacy_okfolio", []string{"photos"}, ""); err == nil {
		t.Fatal("expected non-owned table to be rejected")
	}
}

func TestValidateSelectOnlyGrants(t *testing.T) {
	grants := []string{
		"GRANT USAGE ON *.* TO `okfolio_etl`@`%`",
		"GRANT SELECT ON `legacy_okfolio`.`downloaded_photos` TO `okfolio_etl`@`%`",
		"GRANT SELECT ON `legacy_okfolio`.`extraction_runs` TO `okfolio_etl`@`%`",
	}
	if err := ValidateSelectOnlyGrants(grants, "legacy_okfolio"); err != nil {
		t.Fatalf("expected table-level SELECT grants to pass: %v", err)
	}

	bad := append([]string{}, grants...)
	bad = append(bad, "GRANT SELECT ON `legacy_okfolio`.`albums` TO `okfolio_etl`@`%`")
	if err := ValidateSelectOnlyGrants(bad, "legacy_okfolio"); err == nil {
		t.Fatal("expected SELECT on a non-owned table to be rejected")
	}
}

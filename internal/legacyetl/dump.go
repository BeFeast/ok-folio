package legacyetl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	DownloadedPhotosTable = "downloaded_photos"
	ExtractionRunsTable   = "extraction_runs"
)

var OwnedTables = []string{DownloadedPhotosTable, ExtractionRunsTable}

var forbiddenDumpArgs = []string{
	"--master-data",
	"--source-data",
	"--lock-all-tables",
}

// DumpArgs returns the safe mariadb-dump argument contract for the operator-run
// extract. The password is intentionally absent; pass it through MYSQL_PWD or a
// defaults file supplied by the secret store so it never appears in ps output.
func DumpArgs(database string, tables []string, extraWhere string) ([]string, error) {
	if len(tables) == 0 {
		tables = OwnedTables
	}
	for _, table := range tables {
		if !isOwnedTable(table) {
			return nil, fmt.Errorf("refusing to dump non-owned legacy table %q", table)
		}
	}
	args := []string{
		"mariadb-dump",
		"--single-transaction",
		"--skip-lock-tables",
		"--no-create-info",
		"--no-tablespaces",
		"--compact",
	}
	if extraWhere != "" {
		args = append(args, "--where", extraWhere)
	}
	args = append(args, database)
	args = append(args, tables...)
	return args, nil
}

// ValidateDumpArgs rejects options that acquire a global read lock in MariaDB.
func ValidateDumpArgs(args []string) error {
	joined := strings.ToLower(strings.Join(args, "\x00"))
	for _, forbidden := range forbiddenDumpArgs {
		if strings.Contains(joined, forbidden) {
			return fmt.Errorf("forbidden mariadb-dump argument %q: it can trigger FLUSH TABLES WITH READ LOCK", forbidden)
		}
	}
	for _, required := range []string{"--single-transaction", "--skip-lock-tables", "--no-create-info"} {
		if !containsArg(args, required) {
			return fmt.Errorf("missing required mariadb-dump argument %q", required)
		}
	}
	return nil
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if strings.EqualFold(arg, want) {
			return true
		}
	}
	return false
}

func isOwnedTable(table string) bool {
	for _, owned := range OwnedTables {
		if table == owned {
			return true
		}
	}
	return false
}

// EngineCheckSQL verifies the InnoDB precondition that makes
// --single-transaction lock-free for the two owned tables.
func EngineCheckSQL(database string) string {
	return fmt.Sprintf(
		"SHOW TABLE STATUS FROM `%s` WHERE Name IN ('%s','%s');",
		escapeIdentifier(database),
		DownloadedPhotosTable,
		ExtractionRunsTable,
	)
}

// GrantsCheckSQL is the operator-side SELECT-only verification query. The ETL
// binary does not connect to legacy MariaDB; this string documents the exact
// check to run through the host container-exec path.
func GrantsCheckSQL() string {
	return "SHOW GRANTS FOR CURRENT_USER();"
}

// ValidateSelectOnlyGrants checks SHOW GRANTS output for a user with exactly
// SELECT on the two OK Folio-owned legacy tables and no global/database grants.
func ValidateSelectOnlyGrants(grants []string, database string) error {
	allowed := map[string]bool{
		DownloadedPhotosTable: false,
		ExtractionRunsTable:   false,
	}
	selectRe := regexp.MustCompile(`(?i)^GRANT\s+SELECT\s+ON\s+` + "`?" + regexp.QuoteMeta(database) + "`?\\.`?([^`\\s]+)`?\\s+TO\\s+")
	for _, grant := range grants {
		normalized := strings.TrimSpace(grant)
		upper := strings.ToUpper(normalized)
		if strings.HasPrefix(upper, "GRANT USAGE ") {
			continue
		}
		if strings.Contains(upper, "GRANT OPTION") {
			return fmt.Errorf("legacy user must not have GRANT OPTION: %s", normalized)
		}
		if strings.Contains(normalized, "*.*") || strings.Contains(normalized, "`"+database+"`.*") || strings.Contains(normalized, database+".*") {
			return fmt.Errorf("legacy user must not have global or database-wide grants: %s", normalized)
		}
		matches := selectRe.FindStringSubmatch(normalized)
		if matches == nil {
			return fmt.Errorf("unexpected legacy grant; only table-level SELECT is allowed: %s", normalized)
		}
		table := strings.Trim(matches[1], "`")
		if _, ok := allowed[table]; !ok {
			return fmt.Errorf("legacy user has SELECT on non-owned table %q", table)
		}
		allowed[table] = true
	}
	var missing []string
	for table, seen := range allowed {
		if !seen {
			missing = append(missing, table)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("legacy user is missing table-level SELECT on %s", strings.Join(missing, ", "))
	}
	return nil
}

func escapeIdentifier(value string) string {
	return strings.ReplaceAll(value, "`", "``")
}

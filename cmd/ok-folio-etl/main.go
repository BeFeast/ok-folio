package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/legacyetl"

	"github.com/rs/zerolog"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "load-dump":
		loadDump(os.Args[2:])
	case "hash-content":
		hashContent(os.Args[2:])
	case "smoke-read-paths":
		smokeReadPaths(os.Args[2:])
	case "warm-thumbnails":
		warmThumbnails(os.Args[2:])
	case "print-legacy-checks":
		printLegacyChecks(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func loadDump(args []string) {
	fs := flag.NewFlagSet("load-dump", flag.ExitOnError)
	configPath := fs.String("config", "/config/config.yaml", "Path to OK Folio configuration")
	legacyTZ := fs.String("legacy-timezone", "", "Verified legacy source timezone for naive datetime values")
	setSequences := fs.Bool("setval", false, "Run id sequence setval after load")
	advanceWatermark := fs.Bool("advance-watermark", true, "Advance etl_watermarks after the load transaction commits")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		exitErr(err)
	}
	db, err := database.New(&cfg.Database)
	if err != nil {
		exitErr(err)
	}
	rows, err := legacyetl.ParseDump(os.Stdin)
	if err != nil {
		exitErr(err)
	}
	result, err := legacyetl.LoadDump(db, rows, legacyetl.LoadOptions{
		LegacyTimeZone:   *legacyTZ,
		SetSequences:     *setSequences,
		AdvanceWatermark: *advanceWatermark,
	})
	if err != nil {
		exitErr(err)
	}
	fmt.Printf("loaded downloaded_photos=%d extraction_runs=%d photo_max_id=%d run_max_id=%d\n",
		result.DownloadedPhotos,
		result.ExtractionRuns,
		result.PhotoMaxID,
		result.RunMaxID,
	)
}

func hashContent(args []string) {
	fs := flag.NewFlagSet("hash-content", flag.ExitOnError)
	configPath := fs.String("config", "/config/config.yaml", "Path to OK Folio configuration")
	originalsRoot := fs.String("originals-root", "", "Read-only originals mount root")
	limit := fs.Int("limit", 500, "Maximum rows to hash in this pass")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		exitErr(err)
	}
	db, err := database.New(&cfg.Database)
	if err != nil {
		exitErr(err)
	}
	result, err := legacyetl.FillMissingContentHashes(db, *originalsRoot, *limit)
	if err != nil {
		exitErr(err)
	}
	fmt.Printf("content_hash scanned=%d updated=%d skipped=%d\n", result.Scanned, result.Updated, result.Skipped)
}

func printLegacyChecks(args []string) {
	fs := flag.NewFlagSet("print-legacy-checks", flag.ExitOnError)
	databaseName := fs.String("legacy-database", "", "Legacy database name")
	tablesFlag := fs.String("tables", strings.Join(legacyetl.OwnedTables, ","), "Comma-separated owned legacy tables to dump")
	where := fs.String("where", "", "Optional dump WHERE clause for incremental overlap")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	if *databaseName == "" {
		exitErr(fmt.Errorf("--legacy-database is required"))
	}
	tables := splitTables(*tablesFlag)
	dumpArgs, err := legacyetl.DumpArgs(*databaseName, tables, *where)
	if err != nil {
		exitErr(err)
	}
	if err := legacyetl.ValidateDumpArgs(dumpArgs); err != nil {
		exitErr(err)
	}
	fmt.Println("InnoDB precondition:")
	fmt.Println(legacyetl.EngineCheckSQL(*databaseName))
	fmt.Println()
	fmt.Println("SELECT-only grant check:")
	fmt.Println(legacyetl.GrantsCheckSQL())
	fmt.Println()
	fmt.Println("Safe dump command arguments:")
	fmt.Println(strings.Join(dumpArgs, " "))
}

func warmThumbnails(args []string) {
	fs := flag.NewFlagSet("warm-thumbnails", flag.ExitOnError)
	configPath := fs.String("config", "/config/config.yaml", "Path to OK Folio configuration")
	widthsFlag := fs.String("widths", "400,700", "Comma-separated thumbnail widths to generate")
	concurrency := fs.Int("concurrency", 2, "Maximum concurrent thumbnail generators")
	batchSize := fs.Int("batch-size", 500, "Catalog rows fetched per database batch")
	limit := fs.Int("limit", 0, "Maximum catalog rows to scan; 0 scans all downloaded rows")
	progress := fs.Int("progress", 100, "Log progress every N scanned rows")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		exitErr(err)
	}
	db, err := database.New(&cfg.Database)
	if err != nil {
		exitErr(err)
	}
	widths, err := parseWidths(*widthsFlag)
	if err != nil {
		exitErr(err)
	}
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	result, err := derivatives.WarmThumbnails(context.Background(), db, cfg.Storage, derivatives.WarmOptions{
		Widths:      widths,
		Concurrency: *concurrency,
		BatchSize:   *batchSize,
		Limit:       *limit,
		Progress:    *progress,
	}, logger)
	if err != nil {
		exitErr(err)
	}
	fmt.Printf("warm_thumbnails scanned=%d generated=%d skipped=%d missing=%d failed=%d\n",
		result.Scanned,
		result.Generated,
		result.Skipped,
		result.Missing,
		result.Failed,
	)
	if result.Failed > 0 {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ok-folio-etl <load-dump|hash-content|smoke-read-paths|warm-thumbnails|print-legacy-checks> [flags]")
}

func parseWidths(value string) ([]int, error) {
	var widths []int
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		width, err := strconv.Atoi(part)
		if err != nil || width <= 0 {
			return nil, fmt.Errorf("invalid width %q", part)
		}
		widths = append(widths, width)
	}
	if len(widths) == 0 {
		return nil, derivatives.ErrNoWidths
	}
	return widths, nil
}

func splitTables(value string) []string {
	var tables []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			tables = append(tables, part)
		}
	}
	return tables
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

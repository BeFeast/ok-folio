// Command ok-folio-admin is the non-legacy OK Folio maintenance CLI.
//
// It hosts ongoing catalog maintenance commands that outlive the legacy ETL
// binary: content hashing, thumbnail warming, originals auditing, and the
// read-path smoke gate. Legacy dump/load commands stay in ok-folio-etl.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

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
	case "hash-content":
		hashContent(os.Args[2:])
	case "smoke-read-paths":
		smokeReadPaths(os.Args[2:])
	case "warm-thumbnails":
		warmThumbnails(os.Args[2:])
	case "audit-originals":
		auditOriginals(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
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

func warmThumbnails(args []string) {
	fs := flag.NewFlagSet("warm-thumbnails", flag.ExitOnError)
	configPath := fs.String("config", "/config/config.yaml", "Path to OK Folio configuration")
	widthsFlag := fs.String("widths", "400,700", "Comma-separated thumbnail widths to generate")
	concurrency := fs.Int("concurrency", 2, fmt.Sprintf("Maximum concurrent thumbnail generators (clamped to %d)", derivatives.MaxWarmConcurrency))
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := derivatives.WarmThumbnails(ctx, db, cfg.Storage, derivatives.WarmOptions{
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

func auditOriginals(args []string) {
	fs := flag.NewFlagSet("audit-originals", flag.ExitOnError)
	configPath := fs.String("config", "/config/config.yaml", "Path to OK Folio configuration")
	exclude := fs.Bool("exclude", false, "Mark undecodable originals status='failed' so gallery, warm, and backfill sweeps skip them")
	batchSize := fs.Int("batch-size", 500, "Catalog rows fetched per database batch")
	limit := fs.Int("limit", 0, "Maximum catalog rows to scan; 0 scans all downloaded rows")
	progress := fs.Int("progress", 1000, "Log progress every N scanned rows")
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
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := derivatives.AuditOriginals(ctx, db, cfg.Storage, derivatives.AuditOptions{
		BatchSize: *batchSize,
		Limit:     *limit,
		Progress:  *progress,
		Exclude:   *exclude,
	}, logger)
	if err != nil {
		exitErr(err)
	}
	for _, finding := range result.Findings {
		fmt.Printf("id=%d path=%q size=%d mime=%q first_bytes=%s class=%s excluded=%t decode_error=%q\n",
			finding.PhotoID,
			finding.FilePath,
			finding.FileSize,
			finding.SniffedMIME,
			finding.FirstBytes,
			finding.Class,
			finding.Excluded,
			finding.DecodeError,
		)
	}
	fmt.Printf("audit_originals scanned=%d decodable=%d missing=%d undecodable=%d excluded=%d\n",
		result.Scanned,
		result.Decodable,
		result.Missing,
		result.Undecodable,
		result.Excluded,
	)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ok-folio-admin <hash-content|smoke-read-paths|warm-thumbnails|audit-originals> [flags]")
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

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

package testguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ok-folio/internal/config"
)

var unsafePathMarkers = []string{
	"/data",
	"/mnt",
	"/media",
	"/srv",
	"/var/lib",
	"/var/db",
	"/opt",
	"/photoprism",
	"/originals",
	"/thumbnails",
	"/thumbs",
	"/cookies",
	"/secrets",
}

// ValidateConfig rejects test configuration that could point at production
// media, database, PhotoPrism storage, cookies, env files, or secret material.
func ValidateConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("test config is nil")
	}
	if err := ValidatePath("storage.base_directory", cfg.Storage.BaseDirectory); err != nil {
		return err
	}
	if err := ValidatePath("storage.daily_directory", cfg.Storage.DailyDirectory); err != nil {
		return err
	}
	if cfg.PhotoPrism.Enabled && cfg.PhotoPrism.AutoIndex {
		return errors.New("photoprism auto-indexing must stay disabled in repository tests")
	}
	if cfg.PhotoPrism.Password != "" && !isFixtureSecret(cfg.PhotoPrism.Password) {
		return errors.New("photoprism password in tests must be a fixture placeholder")
	}
	if err := ValidateDatabase(cfg.Database); err != nil {
		return err
	}
	return nil
}

// ValidatePath allows only temporary directories and committed fixture paths.
func ValidatePath(label, path string) error {
	if path == "" {
		return nil
	}
	clean := filepath.Clean(path)
	lower := strings.ToLower(filepath.ToSlash(clean))

	if strings.Contains(lower, ".env") || strings.Contains(lower, "cookie") || strings.Contains(lower, "secret") {
		return fmt.Errorf("%s must not reference runtime env, cookie, or secret material", label)
	}
	if hasPathComponent(lower, "testdata") {
		return nil
	}
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("%s must be an absolute temp path or a testdata fixture path", label)
	}

	tempRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return fmt.Errorf("resolve temp directory: %w", err)
	}
	if isWithin(clean, tempRoot) {
		return nil
	}

	for _, marker := range unsafePathMarkers {
		if lower == marker || strings.HasPrefix(lower, marker+"/") {
			return fmt.Errorf("%s points at production-like storage target %q", label, marker)
		}
	}
	return fmt.Errorf("%s must be under %s for repository tests", label, tempRoot)
}

// ValidateDatabase allows in-memory or explicitly named local test databases.
func ValidateDatabase(cfg config.DatabaseConfig) error {
	if cfg.Host == "" && cfg.Database == "" && cfg.User == "" && cfg.Password == "" {
		return nil
	}
	host := strings.ToLower(cfg.Host)
	if host != "" && host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return errors.New("test database host must be local")
	}
	dbName := strings.ToLower(cfg.Database)
	if dbName != "" && !strings.Contains(dbName, "test") && !strings.Contains(dbName, "fixture") {
		return errors.New("test database name must clearly be a test or fixture database")
	}
	if cfg.User == "root" {
		return errors.New("tests must not use database root credentials")
	}
	if cfg.Password != "" && !isFixtureSecret(cfg.Password) {
		return errors.New("database password in tests must be a fixture placeholder")
	}
	return nil
}

func isFixtureSecret(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "test") ||
		strings.Contains(lower, "fixture") ||
		strings.Contains(lower, "change-me") ||
		strings.Contains(lower, "placeholder") ||
		strings.Contains(lower, "override")
}

func isWithin(path, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "..")
}

func hasPathComponent(path, component string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == component {
			return true
		}
	}
	return false
}

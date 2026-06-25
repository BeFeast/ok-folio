package testguard

import (
	"path/filepath"
	"testing"

	"ok-folio/internal/config"
)

func TestValidateConfigAcceptsTempStorageAndFixtureDatabase(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{
			BaseDirectory:  filepath.Join(tempDir, "originals"),
			DailyDirectory: filepath.Join(tempDir, "daily"),
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			User:     "testuser",
			Password: "testpass",
			Database: "ok_sight_ex_test",
		},
		PhotoPrism: config.PhotoPrismConfig{
			Enabled:   true,
			AutoIndex: false,
			Password:  "fixture-password",
		},
	}

	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected temp-backed test config to pass: %v", err)
	}
}

func TestValidatePathRejectsProductionLikeStorageTargets(t *testing.T) {
	for _, path := range []string{
		"/data/originals",
		"/mnt/pool/media",
		"/var/lib/mysql",
		"/photoprism/storage",
		"/srv/gallery/thumbnails",
	} {
		t.Run(path, func(t *testing.T) {
			if err := ValidatePath("storage.base_directory", path); err == nil {
				t.Fatalf("expected %q to be rejected", path)
			}
		})
	}
}

func TestValidatePathAllowsTestdataFixtures(t *testing.T) {
	if err := ValidatePath("provider.fixture", filepath.Join("internal", "provider", "webgallery", "testdata", "photo.html")); err != nil {
		t.Fatalf("expected testdata fixture to pass: %v", err)
	}
}

func TestValidateConfigRejectsUnsafeTestConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "photoprism auto index",
			cfg: &config.Config{
				Storage: config.StorageConfig{BaseDirectory: t.TempDir()},
				PhotoPrism: config.PhotoPrismConfig{
					Enabled:   true,
					AutoIndex: true,
					Password:  "fixture-password",
				},
			},
		},
		{
			name: "production database name",
			cfg: &config.Config{
				Storage:  config.StorageConfig{BaseDirectory: t.TempDir()},
				Database: config.DatabaseConfig{Host: "localhost", User: "app", Password: "testpass", Database: "ok_sight_ex"},
			},
		},
		{
			name: "remote database host",
			cfg: &config.Config{
				Storage:  config.StorageConfig{BaseDirectory: t.TempDir()},
				Database: config.DatabaseConfig{Host: "db.internal", User: "app", Password: "testpass", Database: "ok_sight_ex_test"},
			},
		},
		{
			name: "runtime env path",
			cfg: &config.Config{
				Storage: config.StorageConfig{BaseDirectory: filepath.Join(t.TempDir(), ".env")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateConfig(tt.cfg); err == nil {
				t.Fatal("expected unsafe test config to be rejected")
			}
		})
	}
}

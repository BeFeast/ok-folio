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

func TestGuardAppDatabaseConfigRejectsLegacyHosts(t *testing.T) {
	for _, host := range []string{"mariadb", "photoprism-mariadb", "photoprism-db", "legacy-mysql", "MariaDB"} {
		t.Run(host, func(t *testing.T) {
			if !IsLegacyDatabaseHost(host) {
				t.Fatalf("expected %q to be detected as a legacy host", host)
			}
			if err := GuardAppDatabaseConfig(config.DatabaseConfig{Host: host}); err == nil {
				t.Fatalf("expected boot guard to refuse legacy host %q", host)
			}
		})
	}
}

func TestGuardAppDatabaseConfigHonorsLegacyHostOverride(t *testing.T) {
	t.Setenv("LEGACY_DB_HOST", "old-db.internal")
	if !IsLegacyDatabaseHost("old-db.internal") {
		t.Fatal("expected LEGACY_DB_HOST value to be treated as a legacy host")
	}
	if err := GuardAppDatabaseConfig(config.DatabaseConfig{Host: "old-db.internal"}); err == nil {
		t.Fatal("expected boot guard to refuse the LEGACY_DB_HOST value")
	}
}

func TestGuardAppDatabaseConfigAllowsPostgresHost(t *testing.T) {
	for _, host := range []string{"postgres", "okfolio-postgres", "localhost", "127.0.0.1", ""} {
		if IsLegacyDatabaseHost(host) {
			t.Fatalf("did not expect %q to be flagged as legacy", host)
		}
		if err := GuardAppDatabaseConfig(config.DatabaseConfig{Host: host}); err != nil {
			t.Fatalf("expected postgres host %q to pass the boot guard: %v", host, err)
		}
	}
}

func TestAssertNonLegacyDSNRejectsMySQLDSN(t *testing.T) {
	legacyDSNs := []string{
		"okfolio:change-me@tcp(mariadb:3306)/okfolio?charset=utf8mb4&parseTime=True&loc=Local",
		"user:pass@tcp(127.0.0.1:3306)/db",
	}
	for _, dsn := range legacyDSNs {
		if err := AssertNonLegacyDSN(dsn); err == nil {
			t.Fatalf("expected a legacy MySQL DSN to be rejected: %q", dsn)
		}
	}

	postgresDSNs := []string{
		"host=postgres port=5432 user=okfolio password=change-me dbname=okfolio sslmode=disable TimeZone=UTC",
		"postgres://okfolio:change-me@postgres:5432/okfolio?sslmode=disable",
	}
	for _, dsn := range postgresDSNs {
		if err := AssertNonLegacyDSN(dsn); err != nil {
			t.Fatalf("expected a Postgres DSN to pass: %q (%v)", dsn, err)
		}
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

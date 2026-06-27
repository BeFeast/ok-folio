package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Clear any existing environment variables that might interfere
	oldDBHost := os.Getenv("DB_HOST")
	oldDBUser := os.Getenv("DB_USER")
	oldDBPass := os.Getenv("DB_PASSWORD")
	oldDBName := os.Getenv("DB_NAME")
	oldDerivativesDir := os.Getenv("OK_FOLIO_DERIVATIVES_DIR")
	oldDerivativesMaxBytes := os.Getenv("OK_FOLIO_DERIVATIVES_MAX_BYTES")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("OK_FOLIO_DERIVATIVES_DIR")
	os.Unsetenv("OK_FOLIO_DERIVATIVES_MAX_BYTES")
	defer func() {
		if oldDBHost != "" {
			os.Setenv("DB_HOST", oldDBHost)
		}
		if oldDBUser != "" {
			os.Setenv("DB_USER", oldDBUser)
		}
		if oldDBPass != "" {
			os.Setenv("DB_PASSWORD", oldDBPass)
		}
		if oldDBName != "" {
			os.Setenv("DB_NAME", oldDBName)
		}
		if oldDerivativesDir != "" {
			os.Setenv("OK_FOLIO_DERIVATIVES_DIR", oldDerivativesDir)
		}
		if oldDerivativesMaxBytes != "" {
			os.Setenv("OK_FOLIO_DERIVATIVES_MAX_BYTES", oldDerivativesMaxBytes)
		}
	}()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	storageDir := filepath.Join(tmpDir, "storage")

	configContent := fmt.Sprintf(`
source:
  base_url: "https://example.com"
  category_id: 1

storage:
  base_directory: %q
  daily_directory: %q
  derivatives_directory: %q
  derivatives_max_bytes: 1048576

database:
  host: "localhost"
  port: 3306
  user: "testuser"
  password: "testpass"
  database: "testdb"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 1h

cache:
  host: "valkey"
  port: 6379
  password: "cachepass"

api:
  enabled: true
  port: 8080
  host: "0.0.0.0"

scheduler:
  enabled: true
  schedule: "0 0 */6 * * *"
  pages: [1, 2, 3]

retry:
  max_attempts: 3
  initial_delay: 1s
  max_delay: 30s
  multiplier: 2.0

exif:
  set_artist: true
  set_date: true
  set_title: true

photoprism:
  enabled: false
  service_url: "http://photoprism:2342"
  auto_index: false

logging:
  level: "info"
  format: "json"
  output: "stdout"

download:
  concurrent_limit: 5
  timeout: 30s
  user_agent: "PhotoExtractor/1.0"
`, filepath.Join(storageDir, "photos"), filepath.Join(storageDir, "daily"), filepath.Join(storageDir, "derivatives"))

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test source config
	if cfg.Source.BaseURL != "https://example.com" {
		t.Errorf("Expected BaseURL 'https://example.com', got '%s'", cfg.Source.BaseURL)
	}
	if cfg.Source.CategoryID != 1 {
		t.Errorf("Expected CategoryID 1, got %d", cfg.Source.CategoryID)
	}

	// Test storage config
	if cfg.Storage.BaseDirectory != filepath.Join(storageDir, "photos") {
		t.Errorf("Expected BaseDirectory under temp storage, got '%s'", cfg.Storage.BaseDirectory)
	}
	if cfg.Storage.DerivativesDirectory != filepath.Join(storageDir, "derivatives") {
		t.Errorf("Expected DerivativesDirectory under temp storage, got '%s'", cfg.Storage.DerivativesDirectory)
	}
	if cfg.Storage.DerivativesMaxBytes != 1048576 {
		t.Errorf("Expected DerivativesMaxBytes 1048576, got %d", cfg.Storage.DerivativesMaxBytes)
	}

	// Test database config
	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected DB Host 'localhost', got '%s'", cfg.Database.Host)
	}
	if cfg.Database.Port != 3306 {
		t.Errorf("Expected DB Port 3306, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Expected DB User 'testuser', got '%s'", cfg.Database.User)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("Expected DB Password 'testpass', got '%s'", cfg.Database.Password)
	}
	if cfg.Database.Database != "testdb" {
		t.Errorf("Expected DB Database 'testdb', got '%s'", cfg.Database.Database)
	}
	if cfg.Database.ConnMaxLifetime != 1*time.Hour {
		t.Errorf("Expected ConnMaxLifetime 1h, got %v", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Cache.Host != "valkey" {
		t.Errorf("Expected Cache Host 'valkey', got '%s'", cfg.Cache.Host)
	}
	if cfg.Cache.Port != 6379 {
		t.Errorf("Expected Cache Port 6379, got %d", cfg.Cache.Port)
	}
	if cfg.Cache.Password != "cachepass" {
		t.Errorf("Expected Cache Password 'cachepass', got '%s'", cfg.Cache.Password)
	}

	// Test API config
	if !cfg.API.Enabled {
		t.Error("Expected API to be enabled")
	}
	if cfg.API.Port != 8080 {
		t.Errorf("Expected API Port 8080, got %d", cfg.API.Port)
	}

	// Test scheduler config
	if !cfg.Scheduler.Enabled {
		t.Error("Expected Scheduler to be enabled")
	}
	if cfg.Scheduler.Schedule != "0 0 */6 * * *" {
		t.Errorf("Expected Schedule '0 0 */6 * * *', got '%s'", cfg.Scheduler.Schedule)
	}
	if len(cfg.Scheduler.Pages) != 3 {
		t.Errorf("Expected 3 pages, got %d", len(cfg.Scheduler.Pages))
	}

	// Test retry config
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay 1s, got %v", cfg.Retry.InitialDelay)
	}
	if cfg.Retry.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier 2.0, got %f", cfg.Retry.Multiplier)
	}

	// Test EXIF config
	if !cfg.EXIF.SetArtist {
		t.Error("Expected EXIF SetArtist to be true")
	}

	// Test logging config
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Logging Level 'info', got '%s'", cfg.Logging.Level)
	}

	// Test download config
	if cfg.Download.ConcurrentLimit != 5 {
		t.Errorf("Expected ConcurrentLimit 5, got %d", cfg.Download.ConcurrentLimit)
	}
	if cfg.Download.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout 30s, got %v", cfg.Download.Timeout)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `
source:
  base_url: [this is not valid yaml syntax
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
database:
  host: "localhost"
  port: 3306
  user: "testuser"
  password: "testpass"
  database: "testdb"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Set environment variables
	os.Setenv("DB_HOST", "override-host")
	os.Setenv("DB_USER", "override-user")
	os.Setenv("DB_PASSWORD", "override-pass")
	os.Setenv("DB_NAME", "override-db")
	os.Setenv("CACHE_HOST", "override-valkey")
	os.Setenv("CACHE_PORT", "6380")
	os.Setenv("CACHE_PASSWORD", "override-cache-pass")
	os.Setenv("OK_FOLIO_DERIVATIVES_DIR", filepath.Join(tmpDir, "override-derivatives"))
	os.Setenv("OK_FOLIO_DERIVATIVES_MAX_BYTES", "2048")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("CACHE_HOST")
		os.Unsetenv("CACHE_PORT")
		os.Unsetenv("CACHE_PASSWORD")
		os.Unsetenv("OK_FOLIO_DERIVATIVES_DIR")
		os.Unsetenv("OK_FOLIO_DERIVATIVES_MAX_BYTES")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables override file values
	if cfg.Database.Host != "override-host" {
		t.Errorf("Expected DB Host 'override-host', got '%s'", cfg.Database.Host)
	}
	if cfg.Database.User != "override-user" {
		t.Errorf("Expected DB User 'override-user', got '%s'", cfg.Database.User)
	}
	if cfg.Database.Password != "override-pass" {
		t.Errorf("Expected DB Password 'override-pass', got '%s'", cfg.Database.Password)
	}
	if cfg.Database.Database != "override-db" {
		t.Errorf("Expected DB Database 'override-db', got '%s'", cfg.Database.Database)
	}
	if cfg.Cache.Host != "override-valkey" {
		t.Errorf("Expected Cache Host 'override-valkey', got '%s'", cfg.Cache.Host)
	}
	if cfg.Cache.Port != 6380 {
		t.Errorf("Expected Cache Port 6380, got %d", cfg.Cache.Port)
	}
	if cfg.Cache.Password != "override-cache-pass" {
		t.Errorf("Expected Cache Password override, got '%s'", cfg.Cache.Password)
	}
	if cfg.Storage.DerivativesDirectory != filepath.Join(tmpDir, "override-derivatives") {
		t.Errorf("Expected derivative dir override, got '%s'", cfg.Storage.DerivativesDirectory)
	}
	if cfg.Storage.DerivativesMaxBytes != 2048 {
		t.Errorf("Expected derivative max bytes override, got %d", cfg.Storage.DerivativesMaxBytes)
	}
}

func TestLoad_PhotoPrismEnvironmentOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
photoprism:
  enabled: true
  service_url: "http://photoprism:2342"
  auto_index: true
  username: "config-user"
  password: "config-pass"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	t.Setenv("PHOTOPRISM_SERVICE_URL", "http://override-photoprism:2342")
	t.Setenv("PHOTOPRISM_USERNAME", "override-user")
	t.Setenv("PHOTOPRISM_PASSWORD", "override-pass")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.PhotoPrism.ServiceURL != "http://override-photoprism:2342" {
		t.Errorf("Expected PhotoPrism ServiceURL override, got '%s'", cfg.PhotoPrism.ServiceURL)
	}
	if cfg.PhotoPrism.Username != "override-user" {
		t.Errorf("Expected PhotoPrism Username override, got '%s'", cfg.PhotoPrism.Username)
	}
	if cfg.PhotoPrism.Password != "override-pass" {
		t.Errorf("Expected PhotoPrism Password override, got '%s'", cfg.PhotoPrism.Password)
	}
}

func TestLoad_PartialEnvironmentOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
database:
  host: "localhost"
  port: 3306
  user: "testuser"
  password: "testpass"
  database: "testdb"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Only set DB_HOST
	os.Setenv("DB_HOST", "override-host")
	defer os.Unsetenv("DB_HOST")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify only host is overridden
	if cfg.Database.Host != "override-host" {
		t.Errorf("Expected DB Host 'override-host', got '%s'", cfg.Database.Host)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Expected DB User 'testuser', got '%s'", cfg.Database.User)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("Expected DB Password 'testpass', got '%s'", cfg.Database.Password)
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	tests := []struct {
		name     string
		config   DatabaseConfig
		expected string
	}{
		{
			name: "key/value form with TimeZone=UTC",
			config: DatabaseConfig{
				Host:     "postgres",
				Port:     5432,
				User:     "okfolio",
				Password: "testpass",
				Database: "okfolio",
				SSLMode:  "disable",
			},
			expected: "host=postgres port=5432 user=okfolio password=testpass dbname=okfolio sslmode=disable TimeZone=UTC",
		},
		{
			name: "missing sslmode defaults to disable",
			config: DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				User:     "admin",
				Password: "secret",
				Database: "production",
			},
			expected: "host=db.example.com port=5433 user=admin password=secret dbname=production sslmode=disable TimeZone=UTC",
		},
		{
			name: "DATABASE_URL is returned verbatim",
			config: DatabaseConfig{
				// Other fields are ignored when URL is set.
				Host: "postgres",
				Port: 5432,
				URL:  "postgres://okfolio:testpass@db.internal:6543/okfolio?sslmode=require",
			},
			expected: "postgres://okfolio:testpass@db.internal:6543/okfolio?sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.DSN()
			if result != tt.expected {
				t.Errorf("Expected DSN '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestLoad_DatabaseEnvOverridesAndDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config without a database block exercises the applied defaults.
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DATABASE_URL", "postgres://okfolio@db.internal:6543/okfolio")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Database.Host != DefaultDatabaseHost {
		t.Errorf("Expected default host %q, got %q", DefaultDatabaseHost, cfg.Database.Host)
	}
	if cfg.Database.Port != 6543 {
		t.Errorf("Expected DB_PORT override 6543, got %d", cfg.Database.Port)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("Expected DB_SSLMODE override 'require', got %q", cfg.Database.SSLMode)
	}
	if cfg.Database.URL != "postgres://okfolio@db.internal:6543/okfolio" {
		t.Errorf("Expected DATABASE_URL override, got %q", cfg.Database.URL)
	}
}

func TestLoad_DefaultsPointAtPostgresService(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Database.Host != "postgres" || cfg.Database.Port != 5432 || cfg.Database.SSLMode != "disable" {
		t.Fatalf("Expected defaults host=postgres port=5432 sslmode=disable, got host=%q port=%d sslmode=%q",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.SSLMode)
	}
}

func TestLoad_DefaultsPointAtValkeyService(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Cache.Host != "valkey" || cfg.Cache.Port != 6379 {
		t.Fatalf("Expected cache defaults host=valkey port=6379, got host=%q port=%d",
			cfg.Cache.Host, cfg.Cache.Port)
	}
	if cfg.Cache.Addr() != "valkey:6379" {
		t.Fatalf("Expected cache addr valkey:6379, got %q", cfg.Cache.Addr())
	}
}

func TestLoad_DefaultsLogging(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Logging.Level != DefaultLoggingLevel {
		t.Fatalf("Expected default logging level %q, got %q", DefaultLoggingLevel, cfg.Logging.Level)
	}
	if cfg.Logging.Format != DefaultLoggingFormat {
		t.Fatalf("Expected default logging format %q, got %q", DefaultLoggingFormat, cfg.Logging.Format)
	}
}

func TestLoad_DefaultsThumbnailDerivatives(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Storage.DerivativesDirectory != DefaultDerivativesDirectory {
		t.Fatalf("Expected default derivatives directory %q, got %q", DefaultDerivativesDirectory, cfg.Storage.DerivativesDirectory)
	}
	if cfg.Storage.DerivativesMaxBytes != DefaultDerivativesMaxBytes {
		t.Fatalf("Expected default derivatives max bytes %d, got %d", DefaultDerivativesMaxBytes, cfg.Storage.DerivativesMaxBytes)
	}
}

func TestLoad_InvalidDBPort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("DB_PORT", "not-a-number")
	if _, err := Load(configPath); err == nil {
		t.Fatal("Expected error for invalid DB_PORT, got nil")
	}
}

func TestLoad_InvalidCachePort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("CACHE_PORT", "not-a-number")
	if _, err := Load(configPath); err == nil {
		t.Fatal("Expected error for invalid CACHE_PORT, got nil")
	}
}

func TestLoad_InvalidDerivativesMaxBytes(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("source:\n  base_url: \"https://example.com\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("OK_FOLIO_DERIVATIVES_MAX_BYTES", "not-a-number")
	if _, err := Load(configPath); err == nil {
		t.Fatal("Expected error for invalid OK_FOLIO_DERIVATIVES_MAX_BYTES, got nil")
	}
}

func TestLoad_MinimalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal.yaml")
	storageDir := filepath.Join(tmpDir, "storage")

	// Minimal config with just required fields
	configContent := fmt.Sprintf(`
source:
  base_url: "https://example.com"
  category_id: 1

storage:
  base_directory: %q
  daily_directory: %q

database:
  host: "localhost"
  port: 3306
  user: "user"
  password: "testpass"
  database: "testdb"
`, filepath.Join(storageDir, "originals"), filepath.Join(storageDir, "daily"))

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load minimal config: %v", err)
	}

	// Verify required fields are set
	if cfg.Source.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
	if cfg.Database.Host == "" {
		t.Error("Database host should not be empty")
	}
}

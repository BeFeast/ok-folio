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
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
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

database:
  host: "localhost"
  port: 3306
  user: "testuser"
  password: "testpass"
  database: "testdb"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 1h

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
`, filepath.Join(storageDir, "photos"), filepath.Join(storageDir, "daily"))

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
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
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
			name: "standard config",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     3306,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			expected: "testuser:testpass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name: "different host and port",
			config: DatabaseConfig{
				Host:     "db.example.com",
				Port:     3307,
				User:     "admin",
				Password: "secret",
				Database: "production",
			},
			expected: "admin:secret@tcp(db.example.com:3307)/production?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name: "empty password",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "",
				Database: "testdb",
			},
			expected: "root:@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name: "special characters in password",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     3306,
				User:     "user",
				Password: "p@ssw0rd!#$",
				Database: "testdb",
			},
			expected: "user:p@ssw0rd!#$@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local",
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

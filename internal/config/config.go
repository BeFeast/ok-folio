package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultDatabaseHost is the OK Folio-owned Postgres service name. OK Folio
	// runs its own dedicated Postgres and never touches the legacy MariaDB.
	DefaultDatabaseHost = "postgres"
	// DefaultDatabasePort is the standard Postgres port.
	DefaultDatabasePort = 5432
	// DefaultDatabaseSSLMode keeps TLS off for the private LAN/container network.
	DefaultDatabaseSSLMode = "disable"
	// DefaultCacheHost is the private stack service name for Valkey.
	DefaultCacheHost = "valkey"
	// DefaultCachePort is Valkey's private service port, not a published host port.
	DefaultCachePort = 6379
)

type Config struct {
	Source     SourceConfig     `yaml:"source"`
	Storage    StorageConfig    `yaml:"storage"`
	Database   DatabaseConfig   `yaml:"database"`
	Cache      CacheConfig      `yaml:"cache"`
	API        APIConfig        `yaml:"api"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
	Retry      RetryConfig      `yaml:"retry"`
	EXIF       EXIFConfig       `yaml:"exif"`
	PhotoPrism PhotoPrismConfig `yaml:"photoprism"`
	Logging    LoggingConfig    `yaml:"logging"`
	Download   DownloadConfig   `yaml:"download"`
}

type SourceConfig struct {
	BaseURL    string `yaml:"base_url"`
	CategoryID int    `yaml:"category_id"`
}

type StorageConfig struct {
	BaseDirectory  string `yaml:"base_directory"`
	DailyDirectory string `yaml:"daily_directory"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"sslmode"`
	URL             string        `yaml:"url"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type CacheConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
}

type APIConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

type SchedulerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Schedule string `yaml:"schedule"`
	Pages    []int  `yaml:"pages"`
}

type RetryConfig struct {
	MaxAttempts  int           `yaml:"max_attempts"`
	InitialDelay time.Duration `yaml:"initial_delay"`
	MaxDelay     time.Duration `yaml:"max_delay"`
	Multiplier   float64       `yaml:"multiplier"`
}

type EXIFConfig struct {
	SetArtist bool `yaml:"set_artist"`
	SetDate   bool `yaml:"set_date"`
	SetTitle  bool `yaml:"set_title"`
}

type PhotoPrismConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ServiceURL string `yaml:"service_url"`
	AutoIndex  bool   `yaml:"auto_index"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type DownloadConfig struct {
	ConcurrentLimit  int           `yaml:"concurrent_limit"`
	Timeout          time.Duration `yaml:"timeout"`
	UserAgent        string        `yaml:"user_agent"`
	DelayBetween     time.Duration `yaml:"delay_between"`
	RateLimitBackoff time.Duration `yaml:"rate_limit_backoff"`
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides. DB_* keys configure OK Folio's own
	// Postgres; they are intentionally kept separate from any LEGACY_DB_* keys.
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		port, err := strconv.Atoi(dbPort)
		if err != nil {
			return nil, fmt.Errorf("invalid DB_PORT %q: %w", dbPort, err)
		}
		cfg.Database.Port = port
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		cfg.Database.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		cfg.Database.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.Database.Database = dbName
	}
	if dbSSLMode := os.Getenv("DB_SSLMODE"); dbSSLMode != "" {
		cfg.Database.SSLMode = dbSSLMode
	}
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.Database.URL = dbURL
	}
	if cacheHost := os.Getenv("CACHE_HOST"); cacheHost != "" {
		cfg.Cache.Host = cacheHost
	}
	if cachePort := os.Getenv("CACHE_PORT"); cachePort != "" {
		port, err := strconv.Atoi(cachePort)
		if err != nil {
			return nil, fmt.Errorf("invalid CACHE_PORT %q: %w", cachePort, err)
		}
		cfg.Cache.Port = port
	}
	if cachePassword := os.Getenv("CACHE_PASSWORD"); cachePassword != "" {
		cfg.Cache.Password = cachePassword
	}
	if photoPrismServiceURL := os.Getenv("PHOTOPRISM_SERVICE_URL"); photoPrismServiceURL != "" {
		cfg.PhotoPrism.ServiceURL = photoPrismServiceURL
	}
	if photoPrismUsername := os.Getenv("PHOTOPRISM_USERNAME"); photoPrismUsername != "" {
		cfg.PhotoPrism.Username = photoPrismUsername
	}
	if photoPrismPassword := os.Getenv("PHOTOPRISM_PASSWORD"); photoPrismPassword != "" {
		cfg.PhotoPrism.Password = photoPrismPassword
	}

	cfg.Database.applyDefaults()
	cfg.Cache.applyDefaults()

	return &cfg, nil
}

// applyDefaults fills in the OK Folio Postgres defaults for any unset field so
// the app points at its own private Postgres service out of the box.
func (c *DatabaseConfig) applyDefaults() {
	if c.Host == "" {
		c.Host = DefaultDatabaseHost
	}
	if c.Port == 0 {
		c.Port = DefaultDatabasePort
	}
	if c.SSLMode == "" {
		c.SSLMode = DefaultDatabaseSSLMode
	}
}

func (c *CacheConfig) applyDefaults() {
	if c.Host == "" {
		c.Host = DefaultCacheHost
	}
	if c.Port == 0 {
		c.Port = DefaultCachePort
	}
}

func (c *CacheConfig) Addr() string {
	port := c.Port
	if port == 0 {
		port = DefaultCachePort
	}
	host := c.Host
	if host == "" {
		host = DefaultCacheHost
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// DSN returns the Postgres connection string for the pgx stdlib driver.
//
// When URL is set it is returned verbatim (pgx accepts either a URL or a
// key/value DSN). Otherwise a key/value DSN is built. TimeZone=UTC pins the
// session zone so timestamptz round-trips are stable; it does NOT reinterpret
// legacy timezone-naive values (that is the loader's responsibility).
func (c *DatabaseConfig) DSN() string {
	if c.URL != "" {
		return c.URL
	}
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = DefaultDatabaseSSLMode
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.Host, c.Port, c.User, c.Password, c.Database, sslMode)
}

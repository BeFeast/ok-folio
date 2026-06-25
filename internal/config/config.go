package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Source     SourceConfig     `yaml:"source"`
	Storage    StorageConfig    `yaml:"storage"`
	Database   DatabaseConfig   `yaml:"database"`
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
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
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

	// Apply environment variable overrides
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.Database.Host = dbHost
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
	if photoPrismServiceURL := os.Getenv("PHOTOPRISM_SERVICE_URL"); photoPrismServiceURL != "" {
		cfg.PhotoPrism.ServiceURL = photoPrismServiceURL
	}
	if photoPrismUsername := os.Getenv("PHOTOPRISM_USERNAME"); photoPrismUsername != "" {
		cfg.PhotoPrism.Username = photoPrismUsername
	}
	if photoPrismPassword := os.Getenv("PHOTOPRISM_PASSWORD"); photoPrismPassword != "" {
		cfg.PhotoPrism.Password = photoPrismPassword
	}

	return &cfg, nil
}

// DSN returns the database connection string
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

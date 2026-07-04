package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	// DefaultDerivativesDirectory is the in-container writable derivative mount.
	DefaultDerivativesDirectory = "/derivatives"
	// DefaultDerivativesMaxBytes bounds generated thumbnails at 20 GiB.
	DefaultDerivativesMaxBytes int64 = 20 * 1024 * 1024 * 1024
	// DefaultLoggingLevel keeps logs visible when logging.level is omitted.
	DefaultLoggingLevel = "info"
	// DefaultLoggingFormat keeps container logs machine-readable.
	DefaultLoggingFormat = "json"
	// DefaultWarmOnIngestWidthSmall is the grid thumbnail width warmed on ingest.
	DefaultWarmOnIngestWidthSmall = 400
	// DefaultWarmOnIngestWidthLarge is the magazine thumbnail width warmed on ingest.
	DefaultWarmOnIngestWidthLarge = 700
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
	Telegram   TelegramConfig   `yaml:"telegram"`
	Logging    LoggingConfig    `yaml:"logging"`
	Download   DownloadConfig   `yaml:"download"`
	Similarity SimilarityConfig `yaml:"similarity"`
}

type SourceConfig struct {
	BaseURL    string `yaml:"base_url"`
	CategoryID int    `yaml:"category_id"`
	Schedule   string `yaml:"schedule"`
}

type StorageConfig struct {
	BaseDirectory        string `yaml:"base_directory"`
	DailyDirectory       string `yaml:"daily_directory"`
	DerivativesDirectory string `yaml:"derivatives_directory"`
	DerivativesMaxBytes  int64  `yaml:"derivatives_max_bytes"`
	WarmOnIngest         bool   `yaml:"warm_on_ingest"`
	WarmOnIngestWidths   []int  `yaml:"warm_on_ingest_widths"`
}

func (c *StorageConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawStorageConfig StorageConfig
	out := rawStorageConfig{
		WarmOnIngest:       true,
		WarmOnIngestWidths: defaultWarmOnIngestWidths(),
	}
	if err := value.Decode(&out); err != nil {
		return err
	}
	*c = StorageConfig(out)
	return nil
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

// PhotoPrismConfig configures the legacy PhotoPrism integration. During Wave 6
// legacy retirement PhotoPrism is a stopped-but-startable fallback, so this
// integration is disabled by default and stays out of the normal OK Folio
// product path. Enabled gates the whole integration (client creation and the
// admin-only manual index route); AutoIndex additionally opts the download flow
// into triggering PhotoPrism indexing after new downloads.
type PhotoPrismConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ServiceURL string `yaml:"service_url"`
	AutoIndex  bool   `yaml:"auto_index"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

// AutoIndexEnabled reports whether new downloads should automatically trigger
// legacy PhotoPrism indexing. It stays off unless an operator explicitly opts in
// with both photoprism.enabled and photoprism.auto_index, keeping PhotoPrism
// indexing out of the normal OK Folio download path.
func (c PhotoPrismConfig) AutoIndexEnabled() bool {
	return c.Enabled && c.AutoIndex
}

type TelegramConfig struct {
	BotToken    string                 `yaml:"bot_token"`
	BaseURL     string                 `yaml:"base_url"`
	FileBaseURL string                 `yaml:"file_base_url"`
	ChatID      string                 `yaml:"chat_id"`
	Sources     []TelegramSourceConfig `yaml:"sources"`
	DisplayName string                 `yaml:"display_name"`
	Limit       int                    `yaml:"limit"`
	Schedule    string                 `yaml:"schedule"`
}

type TelegramSourceConfig struct {
	ChatID string `yaml:"chat_id"`
	Label  string `yaml:"label"`
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

type SimilarityConfig struct {
	Enabled    bool   `yaml:"enabled"`
	SidecarURL string `yaml:"sidecar_url"`
	Backfill   bool   `yaml:"backfill"`
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

	var sections map[string]any
	if err := yaml.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("failed to inspect config sections: %w", err)
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
	if telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN"); telegramBotToken != "" {
		cfg.Telegram.BotToken = telegramBotToken
	}
	if telegramChatID := os.Getenv("TELEGRAM_CHAT_ID"); telegramChatID != "" {
		cfg.Telegram.ChatID = telegramChatID
	}
	if telegramBaseURL := os.Getenv("TELEGRAM_BASE_URL"); telegramBaseURL != "" {
		cfg.Telegram.BaseURL = telegramBaseURL
	}
	if telegramFileBaseURL := os.Getenv("TELEGRAM_FILE_BASE_URL"); telegramFileBaseURL != "" {
		cfg.Telegram.FileBaseURL = telegramFileBaseURL
	}
	if derivativesDir := os.Getenv("OK_FOLIO_DERIVATIVES_DIR"); derivativesDir != "" {
		cfg.Storage.DerivativesDirectory = derivativesDir
	}
	if derivativesMaxBytes := os.Getenv("OK_FOLIO_DERIVATIVES_MAX_BYTES"); derivativesMaxBytes != "" {
		maxBytes, err := strconv.ParseInt(derivativesMaxBytes, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid OK_FOLIO_DERIVATIVES_MAX_BYTES %q: %w", derivativesMaxBytes, err)
		}
		cfg.Storage.DerivativesMaxBytes = maxBytes
	}
	if warmOnIngest := os.Getenv("OK_FOLIO_WARM_ON_INGEST"); warmOnIngest != "" {
		enabled, err := strconv.ParseBool(warmOnIngest)
		if err != nil {
			return nil, fmt.Errorf("invalid OK_FOLIO_WARM_ON_INGEST %q: %w", warmOnIngest, err)
		}
		cfg.Storage.WarmOnIngest = enabled
	}
	if warmWidths := os.Getenv("OK_FOLIO_WARM_ON_INGEST_WIDTHS"); warmWidths != "" {
		widths, err := parseWarmWidths(warmWidths)
		if err != nil {
			return nil, fmt.Errorf("invalid OK_FOLIO_WARM_ON_INGEST_WIDTHS %q: %w", warmWidths, err)
		}
		cfg.Storage.WarmOnIngestWidths = widths
	}
	if similarityEnabled := os.Getenv("OK_FOLIO_SIMILARITY_ENABLED"); similarityEnabled != "" {
		enabled, err := strconv.ParseBool(similarityEnabled)
		if err != nil {
			return nil, fmt.Errorf("invalid OK_FOLIO_SIMILARITY_ENABLED %q: %w", similarityEnabled, err)
		}
		cfg.Similarity.Enabled = enabled
	}
	if similaritySidecarURL := os.Getenv("OK_FOLIO_SIMILARITY_SIDECAR_URL"); similaritySidecarURL != "" {
		cfg.Similarity.SidecarURL = similaritySidecarURL
	}
	if similarityBackfill := os.Getenv("OK_FOLIO_SIMILARITY_BACKFILL"); similarityBackfill != "" {
		enabled, err := strconv.ParseBool(similarityBackfill)
		if err != nil {
			return nil, fmt.Errorf("invalid OK_FOLIO_SIMILARITY_BACKFILL %q: %w", similarityBackfill, err)
		}
		cfg.Similarity.Backfill = enabled
	}

	cfg.Storage.applyDefaults()
	cfg.Database.applyDefaults()
	cfg.Cache.applyDefaults()
	cfg.Logging.applyDefaults()

	if err := cfg.validateCompleteness(sections); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validateCompleteness(sections map[string]any) error {
	for _, section := range []string{"source", "logging", "download"} {
		if _, ok := sections[section]; !ok {
			return fmt.Errorf("config section %q is required", section)
		}
	}

	if strings.TrimSpace(c.Source.BaseURL) == "" {
		return fmt.Errorf("source.base_url is required")
	}
	if c.Source.CategoryID <= 0 {
		return fmt.Errorf("source.category_id must be greater than zero")
	}
	if err := validateSourceURL(c.Source.BaseURL); err != nil {
		return err
	}

	if strings.TrimSpace(c.Logging.Level) == "" {
		return fmt.Errorf("logging.level is required")
	}
	if strings.TrimSpace(c.Logging.Format) == "" {
		return fmt.Errorf("logging.format is required")
	}
	if strings.TrimSpace(c.Logging.Output) == "" {
		return fmt.Errorf("logging.output is required")
	}

	if c.Download.ConcurrentLimit <= 0 {
		return fmt.Errorf("download.concurrent_limit must be greater than zero")
	}
	if c.Download.Timeout <= 0 {
		return fmt.Errorf("download.timeout must be greater than zero")
	}
	if strings.TrimSpace(c.Download.UserAgent) == "" {
		return fmt.Errorf("download.user_agent is required")
	}
	if c.Download.DelayBetween <= 0 {
		return fmt.Errorf("download.delay_between must be greater than zero")
	}
	if c.Download.RateLimitBackoff <= 0 {
		return fmt.Errorf("download.rate_limit_backoff must be greater than zero")
	}

	return nil
}

func validateSourceURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("source.base_url is invalid: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("source.base_url must be an absolute URL")
	}
	if !strings.EqualFold(parsed.Host, "sight.photo") {
		return nil
	}

	if hasCategoryQuery(parsed.Query()) {
		return nil
	}
	if strings.Contains(parsed.Path, "/photos/category/") && !strings.HasSuffix(parsed.Path, "/") {
		return fmt.Errorf("source.base_url for sight.photo category paths must end with a slash")
	}
	if !strings.Contains(parsed.Path, "/photos/category/") {
		return fmt.Errorf("source.base_url for sight.photo must point at a category listing")
	}
	return nil
}

func hasCategoryQuery(values url.Values) bool {
	for _, key := range []string{"category", "category_id", "cat"} {
		if strings.TrimSpace(values.Get(key)) != "" {
			return true
		}
	}
	return false
}

func (c *StorageConfig) applyDefaults() {
	if c.DerivativesDirectory == "" {
		c.DerivativesDirectory = DefaultDerivativesDirectory
	}
	if c.DerivativesMaxBytes == 0 {
		c.DerivativesMaxBytes = DefaultDerivativesMaxBytes
	}
	if len(c.WarmOnIngestWidths) == 0 {
		c.WarmOnIngestWidths = defaultWarmOnIngestWidths()
	}
}

func defaultWarmOnIngestWidths() []int {
	return []int{DefaultWarmOnIngestWidthSmall, DefaultWarmOnIngestWidthLarge}
}

func parseWarmWidths(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	widths := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		width, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		widths = append(widths, width)
	}
	if len(widths) == 0 {
		return nil, fmt.Errorf("at least one width is required")
	}
	return widths, nil
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

func (c *LoggingConfig) applyDefaults() {
	if c.Level == "" {
		c.Level = DefaultLoggingLevel
	}
	if c.Format == "" {
		c.Format = DefaultLoggingFormat
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

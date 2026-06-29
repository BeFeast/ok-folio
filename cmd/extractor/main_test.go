package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/provider/telegram"
	"ok-folio/internal/provider/webgallery"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBuildConnectorsSkipsTelegramWithoutBotToken(t *testing.T) {
	cfg := &config.Config{}

	connectors := buildConnectors(cfg, nil, zerolog.Nop())

	if len(connectors) != 1 {
		t.Fatalf("expected only webgallery connector, got %d", len(connectors))
	}
	if connectors[0].Provider().ID == telegram.ProviderID {
		t.Fatal("telegram connector should not be built without a bot token")
	}
}

func TestBuildConnectorsAddsTelegramWithBotToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.BotToken = "test-token"
	cfg.Telegram.Schedule = "0 0 * * * *"

	connectors := buildConnectors(cfg, nil, zerolog.Nop())

	for _, connector := range connectors {
		if connector.Provider().ID == telegram.ProviderID {
			if connector.Provider().Schedule != "0 0 * * * *" {
				t.Fatalf("expected telegram schedule to be configured, got %q", connector.Provider().Schedule)
			}
			return
		}
	}
	t.Fatal("expected telegram connector with configured bot token")
}

func TestBuildConnectorsAddsWebGallerySchedule(t *testing.T) {
	cfg := &config.Config{}
	cfg.Source.Schedule = "0 0 */6 * * *"

	connectors := buildConnectors(cfg, nil, zerolog.Nop())

	if connectors[0].Provider().ID != webgallery.ProviderID {
		t.Fatalf("expected first connector to be webgallery, got %q", connectors[0].Provider().ID)
	}
	if connectors[0].Provider().Schedule != "0 0 */6 * * *" {
		t.Fatalf("expected webgallery schedule to be configured, got %q", connectors[0].Provider().Schedule)
	}
}

func TestBuildConnectorsAddsEveryEnabledWebGallerySource(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&database.ConnectorSource{}); err != nil {
		t.Fatalf("migrate connector sources: %v", err)
	}
	db := &database.DB{DB: gormDB}

	first := mustJSONConfig(t, webgallery.DefaultConfig("https://one.example.test/gallery"))
	second := mustJSONConfig(t, webgallery.WebGalleryConfig{
		ListURL: "https://two.example.test/archive",
		Pagination: webgallery.PaginationConfig{
			Strategy: "none",
		},
		Selectors: webgallery.SelectorConfig{
			ItemLink: "a.item",
			Image:    webgallery.FieldSelector{Selector: "img.full", Attr: "data-src"},
		},
	})
	if _, err := db.CreateConnectorSource(database.ConnectorSource{Type: webgallery.ProviderID, ChatID: "one", Label: "One", Config: first, Enabled: true}); err != nil {
		t.Fatalf("create first source: %v", err)
	}
	if _, err := db.CreateConnectorSource(database.ConnectorSource{Type: webgallery.ProviderID, ChatID: "two", Label: "Two", Config: second, Enabled: true}); err != nil {
		t.Fatalf("create second source: %v", err)
	}
	if _, err := db.CreateConnectorSource(database.ConnectorSource{Type: webgallery.ProviderID, ChatID: "paused", Label: "Paused", Config: second, Enabled: false}); err != nil {
		t.Fatalf("create disabled source: %v", err)
	}

	connectors := buildConnectors(&config.Config{}, db, zerolog.Nop())
	var webgalleryIDs []string
	for _, connector := range connectors {
		if strings.HasPrefix(connector.Provider().ID, webgallery.ProviderID) {
			webgalleryIDs = append(webgalleryIDs, connector.Provider().ID)
		}
	}
	if len(webgalleryIDs) != 2 {
		t.Fatalf("expected two enabled webgallery connectors, got %#v", webgalleryIDs)
	}
	if webgalleryIDs[0] == webgallery.ProviderID || webgalleryIDs[0] == webgalleryIDs[1] {
		t.Fatalf("expected independent per-source provider IDs, got %#v", webgalleryIDs)
	}
}

func TestBuildConnectorsDoesNotFallbackWhenManagedWebGallerySourcesAreDisabled(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&database.ConnectorSource{}); err != nil {
		t.Fatalf("migrate connector sources: %v", err)
	}
	db := &database.DB{DB: gormDB}

	sourceConfig := mustJSONConfig(t, webgallery.DefaultConfig("https://paused.example.test/gallery"))
	if _, err := db.CreateConnectorSource(database.ConnectorSource{
		Type:    webgallery.ProviderID,
		ChatID:  "paused",
		Label:   "Paused",
		Config:  sourceConfig,
		Enabled: false,
	}); err != nil {
		t.Fatalf("create disabled source: %v", err)
	}

	cfg := &config.Config{}
	cfg.Source.BaseURL = "https://legacy.example.test/gallery"
	connectors := buildConnectors(cfg, db, zerolog.Nop())
	for _, connector := range connectors {
		if strings.HasPrefix(connector.Provider().ID, webgallery.ProviderID) {
			t.Fatalf("expected no webgallery connectors when all managed sources are disabled, got %q", connector.Provider().ID)
		}
	}
}

func mustJSONConfig(t *testing.T, cfg webgallery.WebGalleryConfig) database.JSONConfig {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal webgallery config: %v", err)
	}
	return database.JSONConfig(data)
}

func TestSetupLoggerEmptyLevelEmitsInfo(t *testing.T) {
	output := captureSetupLoggerInfoOutput(t, "")
	if !strings.Contains(output, "info log should be visible") {
		t.Fatalf("expected info log to be emitted, got %q", output)
	}
}

func TestSetupLoggerInvalidLevelEmitsInfo(t *testing.T) {
	output := captureSetupLoggerInfoOutput(t, "not-a-level")
	if !strings.Contains(output, "info log should be visible") {
		t.Fatalf("expected info log to be emitted, got %q", output)
	}
}

func captureSetupLoggerInfoOutput(t *testing.T, level string) string {
	t.Helper()

	oldStdout := os.Stdout
	oldLevel := zerolog.GlobalLevel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	defer func() {
		os.Stdout = oldStdout
		zerolog.SetGlobalLevel(oldLevel)
		reader.Close()
	}()

	os.Stdout = writer
	logger := setupLogger(&config.Config{Logging: config.LoggingConfig{Level: level}})
	logger.Info().Msg("info log should be visible")
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close stdout pipe writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	return string(output)
}

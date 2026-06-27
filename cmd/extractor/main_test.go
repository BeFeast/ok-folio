package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"ok-folio/internal/config"
	"ok-folio/internal/provider/telegram"
	"ok-folio/internal/provider/webgallery"

	"github.com/rs/zerolog"
)

func TestBuildConnectorsSkipsTelegramWithoutBotToken(t *testing.T) {
	cfg := &config.Config{}

	connectors := buildConnectors(cfg, zerolog.Nop())

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

	connectors := buildConnectors(cfg, zerolog.Nop())

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

	connectors := buildConnectors(cfg, zerolog.Nop())

	if connectors[0].Provider().ID != webgallery.ProviderID {
		t.Fatalf("expected first connector to be webgallery, got %q", connectors[0].Provider().ID)
	}
	if connectors[0].Provider().Schedule != "0 0 */6 * * *" {
		t.Fatalf("expected webgallery schedule to be configured, got %q", connectors[0].Provider().Schedule)
	}
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

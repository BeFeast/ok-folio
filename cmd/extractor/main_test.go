package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"ok-folio/internal/config"
	"ok-folio/internal/provider/telegram"

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

	connectors := buildConnectors(cfg, zerolog.Nop())

	for _, connector := range connectors {
		if connector.Provider().ID == telegram.ProviderID {
			return
		}
	}
	t.Fatal("expected telegram connector with configured bot token")
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

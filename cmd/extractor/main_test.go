package main

import (
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

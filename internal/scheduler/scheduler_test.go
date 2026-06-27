package scheduler

import (
	"context"
	"testing"

	"ok-folio/internal/config"
	"ok-folio/internal/provider"

	"github.com/rs/zerolog"
)

func TestStartRegistersConnectorSchedules(t *testing.T) {
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled:  true,
			Schedule: "0 0 0 1 1 *",
		},
	}
	connectors := []provider.Connector{
		fakeConnector{source: provider.Source{ID: "webgallery"}},
		fakeConnector{source: provider.Source{ID: "telegram", Schedule: "0 0 0 2 1 *"}},
	}
	s := New(cfg, nil, connectors, nil, nil, zerolog.Nop())

	if err := s.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer s.Stop()

	entries := s.cron.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected two cron entries, got %d", len(entries))
	}
	if got := connectorSchedule(connectors[0].Provider(), cfg.Scheduler.Schedule); got != cfg.Scheduler.Schedule {
		t.Fatalf("expected webgallery to use fallback schedule, got %q", got)
	}
	if got := connectorSchedule(connectors[1].Provider(), cfg.Scheduler.Schedule); got != "0 0 0 2 1 *" {
		t.Fatalf("expected telegram to use custom schedule, got %q", got)
	}
}

func TestStartRejectsDuplicateConnectorProvider(t *testing.T) {
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled:  true,
			Schedule: "0 0 0 1 1 *",
		},
	}
	connectors := []provider.Connector{
		fakeConnector{source: provider.Source{ID: "telegram"}},
		fakeConnector{source: provider.Source{ID: "telegram", Schedule: "0 0 0 2 1 *"}},
	}
	s := New(cfg, nil, connectors, nil, nil, zerolog.Nop())

	if err := s.Start(); err == nil {
		t.Fatal("expected duplicate provider error, got nil")
	}
}

func TestStartReportsConnectorScheduleError(t *testing.T) {
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled:  true,
			Schedule: "0 0 0 1 1 *",
		},
	}
	connectors := []provider.Connector{
		fakeConnector{source: provider.Source{ID: "telegram", Schedule: "not-a-cron"}},
	}
	s := New(cfg, nil, connectors, nil, nil, zerolog.Nop())

	if err := s.Start(); err == nil {
		t.Fatal("expected invalid connector schedule error, got nil")
	}
}

func TestRunConnectorExtractionSkipsOverlappingRun(t *testing.T) {
	s := New(&config.Config{}, nil, nil, nil, nil, zerolog.Nop())
	job := &connectorJob{connector: fakeConnector{source: provider.Source{ID: "telegram"}}}
	job.mu.Lock()
	defer job.mu.Unlock()

	s.runConnectorExtraction(job)
}

type fakeConnector struct {
	source provider.Source
}

func (c fakeConnector) Provider() provider.Source {
	return c.source
}

func (c fakeConnector) DiscoverPage(context.Context, provider.PageRequest) (*provider.PageResult, error) {
	return &provider.PageResult{}, nil
}

func (c fakeConnector) ResolveMedia(context.Context, provider.DiscoveredMedia) (*provider.DiscoveredMedia, error) {
	return nil, nil
}

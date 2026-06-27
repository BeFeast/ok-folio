package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"ok-folio/internal/config"
	"ok-folio/internal/ingest"
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

func TestRunConnectorExtractionSerializesSharedIngestor(t *testing.T) {
	runner := &recordingRunner{
		entered: make(chan string, 2),
		release: make(chan struct{}),
	}
	s := &Scheduler{
		cfg:      &config.Config{},
		ingestor: runner,
		logger:   zerolog.Nop(),
	}
	first := &connectorJob{connector: fakeConnector{source: provider.Source{ID: "telegram"}}}
	second := &connectorJob{connector: fakeConnector{source: provider.Source{ID: "webgallery"}}}
	done := make(chan struct{}, 2)

	go func() {
		s.runConnectorExtraction(first)
		done <- struct{}{}
	}()
	if got := waitEntered(t, runner.entered); got != "telegram" {
		t.Fatalf("expected telegram to enter first, got %q", got)
	}

	go func() {
		s.runConnectorExtraction(second)
		done <- struct{}{}
	}()
	select {
	case providerID := <-runner.entered:
		t.Fatalf("expected second connector to wait for shared ingestor, but %s entered", providerID)
	case <-time.After(50 * time.Millisecond):
	}

	runner.release <- struct{}{}
	waitDone(t, done)
	if got := waitEntered(t, runner.entered); got != "webgallery" {
		t.Fatalf("expected webgallery to enter after telegram completed, got %q", got)
	}
	runner.release <- struct{}{}
	waitDone(t, done)

	if runner.maxActiveRuns() != 1 {
		t.Fatalf("expected at most one active connector ingestion, got %d", runner.maxActiveRuns())
	}
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

type recordingRunner struct {
	mu        sync.Mutex
	active    int
	maxActive int
	entered   chan string
	release   chan struct{}
}

func (r *recordingRunner) RunConnectorWithOptions(_ context.Context, connector provider.Connector, _ ingest.RunOptions) (ingest.Result, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()

	r.entered <- connector.Provider().ID
	<-r.release

	r.mu.Lock()
	r.active--
	r.mu.Unlock()

	return ingest.Result{}, nil
}

func (r *recordingRunner) maxActiveRuns() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.maxActive
}

func waitEntered(t *testing.T, entered <-chan string) string {
	t.Helper()

	select {
	case providerID := <-entered:
		return providerID
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connector ingestion to start")
		return ""
	}
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connector ingestion to finish")
	}
}

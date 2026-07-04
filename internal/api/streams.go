package api

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ok-folio/internal/database"
)

type connectorStatusResponse struct {
	Connectors []connectorStatus `json:"connectors"`
}

type connectorStatus struct {
	ID           string                  `json:"id"`
	DisplayName  string                  `json:"display_name"`
	Health       string                  `json:"health"`
	State        string                  `json:"state"`
	LastSync     *time.Time              `json:"last_sync"`
	Counts       connectorCounts         `json:"counts"`
	Sources      []connectorSourceStatus `json:"sources"`
	RecentRuns   []connectorRunStatus    `json:"recent_runs"`
	RecentErrors []connectorErrorStatus  `json:"recent_errors"`
	hasState     bool
	lastStatus   string
	// activeStateAt records the LastRunAt of the connector_state row currently
	// driving hasState/lastStatus. It lets the family card prefer the newest
	// source-scoped state over a stale family-level one regardless of the order
	// in which rows arrive.
	activeStateAt *time.Time
	// activeStateProviderID records the provider ID of that same state row so
	// ties on LastRunAt can prefer a source-scoped row over a family-level one.
	activeStateProviderID string
}

type connectorCounts struct {
	Downloaded int64 `json:"downloaded"`
	Failed     int64 `json:"failed"`
	Pending    int64 `json:"pending"`
	Total      int64 `json:"total"`
}

type connectorSourceStatus struct {
	ID          string          `json:"id"`
	DisplayName string          `json:"display_name"`
	ProviderID  string          `json:"provider_id"`
	LastSync    *time.Time      `json:"last_sync"`
	Counts      connectorCounts `json:"counts"`
}

type connectorRunStatus struct {
	ID               uint64     `json:"id"`
	StartTime        *time.Time `json:"start_time"`
	EndTime          *time.Time `json:"end_time"`
	Status           string     `json:"status"`
	PagesProcessed   int        `json:"pages_processed"`
	PhotosFound      int        `json:"photos_found"`
	PhotosDownloaded int        `json:"photos_downloaded"`
	PhotosSkipped    int        `json:"photos_skipped"`
	PhotosFailed     int        `json:"photos_failed"`
	ErrorMessage     string     `json:"error_message,omitempty"`
}

type connectorErrorStatus struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	Source     string    `json:"source"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

type connectorSourceRef struct {
	connector *connectorStatus
	index     int
}

func (s *Server) handleConnectorStatus(w http.ResponseWriter, r *http.Request) {
	sourceStats, err := s.db.GetConnectorSourceStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector source stats")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	runs, err := s.db.GetRecentConnectorRuns(5)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector runs")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	states, err := s.db.GetConnectorStates()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector state")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	recentErrors, err := s.db.GetRecentConnectorErrors(10)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector errors")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	connectors := buildConnectorStatuses(sourceStats, runs, states, recentErrors)
	s.writeJSON(w, http.StatusOK, connectorStatusResponse{Connectors: connectors})
}

func buildConnectorStatuses(sourceStats []database.ConnectorSourceStats, runs []database.ExtractionRun, states []database.ConnectorState, recentErrors []database.ConnectorError) []connectorStatus {
	byConnector := map[string]*connectorStatus{
		"webgallery": {
			ID:           "webgallery",
			DisplayName:  "Web Gallery",
			Health:       "idle",
			State:        "Idle",
			Sources:      []connectorSourceStatus{},
			RecentRuns:   []connectorRunStatus{},
			RecentErrors: []connectorErrorStatus{},
		},
		"telegram": {
			ID:           "telegram",
			DisplayName:  "Telegram",
			Health:       "idle",
			State:        "Not synced",
			Sources:      []connectorSourceStatus{},
			RecentRuns:   []connectorRunStatus{},
			RecentErrors: []connectorErrorStatus{},
		},
	}
	sourceIndex := make(map[string]connectorSourceRef)
	runErrorFallbacks := make(map[string]connectorErrorStatus)
	latestRuns := latestConnectorRunsByProvider(runs)

	for _, stat := range sourceStats {
		providerID := connectorProviderIDFromStored(stat.Provider)
		if providerID == "unknown" {
			providerID = connectorProviderIDFromSource(stat.SourcePage, stat.URL)
			if providerID == "unknown" {
				providerID = "webgallery"
			}
		}
		connector := ensureConnectorStatus(byConnector, connectorFamilyID(providerID))
		sourceID := connectorSourceID(stat.SourcePage, stat.URL, providerID)
		sourceKey := providerID + "\x00" + sourceID

		sourceRef, ok := sourceIndex[sourceKey]
		if !ok {
			connector.Sources = append(connector.Sources, connectorSourceStatus{
				ID:          sourceID,
				DisplayName: connectorSourceDisplayName(sourceID, providerID),
				ProviderID:  providerID,
			})
			sourceRef = connectorSourceRef{
				connector: connector,
				index:     len(connector.Sources) - 1,
			}
			sourceIndex[sourceKey] = sourceRef
		}
		source := &sourceRef.connector.Sources[sourceRef.index]

		applyConnectorCount(&source.Counts, stat.Status, stat.Count)
		applyConnectorCount(&connector.Counts, stat.Status, stat.Count)
		if stat.LastActivity != nil && !stat.LastActivity.IsZero() {
			source.LastSync = maxTime(source.LastSync, *stat.LastActivity)
		}
	}

	for _, run := range runs {
		providerID := connectorRunProviderID(run)
		connector := ensureConnectorStatus(byConnector, connectorFamilyID(providerID))
		if runLastSync := extractionRunLastSync(run); runLastSync != nil {
			connector.LastSync = maxTime(connector.LastSync, *runLastSync)
		}
		connector.RecentRuns = append(connector.RecentRuns, connectorRunStatus{
			ID:               run.ID,
			StartTime:        run.StartTime,
			EndTime:          run.EndTime,
			Status:           run.Status,
			PagesProcessed:   run.PagesProcessed,
			PhotosFound:      run.PhotosFound,
			PhotosDownloaded: run.PhotosDownloaded,
			PhotosSkipped:    run.PhotosSkipped,
			PhotosFailed:     run.PhotosFailed,
			ErrorMessage:     run.ErrorMessage,
		})
		if run.ErrorMessage != "" && latestRuns[providerID].ID == run.ID && shouldSurfaceRunError(run) {
			if _, exists := runErrorFallbacks[providerID]; !exists {
				occurredAt := run.StartTime
				if lastSync := extractionRunLastSync(run); lastSync != nil {
					occurredAt = lastSync
				}
				if occurredAt != nil {
					runErrorFallbacks[providerID] = connectorErrorStatus{
						ID:         "run:" + strconvUint(run.ID),
						SourceID:   providerID,
						Source:     connectorDisplayName(providerID),
						Title:      "Latest run",
						Message:    run.ErrorMessage,
						OccurredAt: *occurredAt,
					}
				}
			}
		}
	}

	for _, state := range states {
		providerID := state.ProviderID
		if providerID == "" {
			providerID = "webgallery"
		}
		connector := ensureConnectorStatus(byConnector, connectorFamilyID(providerID))
		// A family card can back multiple state rows (a stale family-level
		// `webgallery` and an active `webgallery:<source_id>`). Drive the card's
		// health from the newest state so historical rows never mark it stale.
		if connectorStateIsNewer(state, providerID, connector) {
			connector.hasState = true
			connector.lastStatus = state.LastStatus
			connector.activeStateAt = state.LastRunAt
			connector.activeStateProviderID = providerID
		}
		if state.LastRunAt != nil && !state.LastRunAt.IsZero() {
			connector.LastSync = maxTime(connector.LastSync, *state.LastRunAt)
		}
	}

	providersWithMediaError := make(map[string]bool)
	for _, connectorError := range recentErrors {
		providerID := connectorProviderIDFromStored(connectorError.Provider)
		if providerID == "unknown" {
			providerID = connectorProviderIDFromSource(connectorError.SourcePage, connectorError.URL)
			if providerID == "unknown" {
				providerID = "webgallery"
			}
		}
		providersWithMediaError[providerID] = true
		connector := ensureConnectorStatus(byConnector, connectorFamilyID(providerID))
		sourceID := connectorSourceID(connectorError.SourcePage, connectorError.URL, providerID)
		message := connectorError.ErrorMessage
		if message == "" {
			message = "Download failed"
		}
		connector.RecentErrors = append(connector.RecentErrors, connectorErrorStatus{
			ID:         strconvUint(connectorError.ID),
			SourceID:   sourceID,
			Source:     connectorSourceDisplayName(sourceID, providerID),
			Title:      connectorError.Title,
			Message:    message,
			OccurredAt: connectorError.OccurredAt,
		})
	}
	// Surface each source-scoped run-error fallback independently. A family card
	// can back several `webgallery:<source_id>` providers, so suppress a fallback
	// only when that same provider already has a media-level error to show —
	// never let one source's error hide another source's failed-run message.
	for providerID, fallback := range runErrorFallbacks {
		connector := ensureConnectorStatus(byConnector, connectorFamilyID(providerID))
		if !providersWithMediaError[providerID] {
			connector.RecentErrors = append(connector.RecentErrors, fallback)
		}
	}

	connectors := make([]connectorStatus, 0, len(byConnector))
	for _, connector := range byConnector {
		sort.Slice(connector.Sources, func(i, j int) bool {
			if connector.Sources[i].Counts.Total == connector.Sources[j].Counts.Total {
				return connector.Sources[i].DisplayName < connector.Sources[j].DisplayName
			}
			return connector.Sources[i].Counts.Total > connector.Sources[j].Counts.Total
		})
		connector.Health, connector.State = connectorHealth(*connector)
		connectors = append(connectors, *connector)
	}

	sort.Slice(connectors, func(i, j int) bool {
		if connectors[i].ID == "webgallery" {
			return true
		}
		if connectors[j].ID == "webgallery" {
			return false
		}
		return connectors[i].DisplayName < connectors[j].DisplayName
	})

	return connectors
}

func shouldSurfaceRunError(run database.ExtractionRun) bool {
	status := normalizeConnectorStatus(run.Status)
	return status == "failed" || status == "completed_with_errors" || run.PhotosFailed > 0
}

func latestConnectorRunsByProvider(runs []database.ExtractionRun) map[string]database.ExtractionRun {
	latest := map[string]database.ExtractionRun{}
	for _, run := range runs {
		providerID := connectorRunProviderID(run)
		current, ok := latest[providerID]
		if !ok || extractionRunIsNewer(run, current) {
			latest[providerID] = run
		}
	}
	return latest
}

func connectorRunProviderID(run database.ExtractionRun) string {
	if run.Provider == "" {
		return "webgallery"
	}
	return run.Provider
}

// connectorFamilyID collapses a source-scoped provider key
// (`webgallery:<source_id>`) onto its provider family (`webgallery`) so Streams
// renders one connector card per family with source-scoped rows nested under it.
// Bare provider IDs (`webgallery`, `telegram`) are returned unchanged.
func connectorFamilyID(providerID string) string {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return "webgallery"
	}
	if family, _, ok := strings.Cut(providerID, ":"); ok && family != "" {
		return family
	}
	return providerID
}

// connectorStateIsNewer reports whether a connector_state row should replace the
// one currently driving the family card's health. The newest LastRunAt wins so a
// stale family-level state cannot override an active source-scoped state.
func connectorStateIsNewer(state database.ConnectorState, providerID string, connector *connectorStatus) bool {
	if !connector.hasState {
		return true
	}
	if state.LastRunAt == nil || state.LastRunAt.IsZero() {
		return false
	}
	if connector.activeStateAt == nil {
		return true
	}
	if state.LastRunAt.After(*connector.activeStateAt) {
		return true
	}
	// On equal LastRunAt, prefer a source-scoped row (`webgallery:<id>`) over a
	// family-level one (`webgallery`). States arrive ordered by provider ID, so
	// the stale family row would otherwise win the tie and keep the card halted
	// even when the active source-scoped state is completed.
	if state.LastRunAt.Equal(*connector.activeStateAt) {
		return connectorProviderIsSourceScoped(providerID) &&
			!connectorProviderIsSourceScoped(connector.activeStateProviderID)
	}
	return false
}

// connectorProviderIsSourceScoped reports whether a provider ID carries a source
// suffix (`webgallery:<source_id>`) rather than being a bare provider family.
func connectorProviderIsSourceScoped(providerID string) bool {
	return strings.Contains(providerID, ":")
}

func extractionRunIsNewer(candidate database.ExtractionRun, current database.ExtractionRun) bool {
	if candidate.StartTime != nil && current.StartTime != nil {
		if !candidate.StartTime.Equal(*current.StartTime) {
			return candidate.StartTime.After(*current.StartTime)
		}
		return candidate.ID > current.ID
	}
	if candidate.StartTime != nil {
		return true
	}
	if current.StartTime != nil {
		return false
	}
	return candidate.ID > current.ID
}

func ensureConnectorStatus(byConnector map[string]*connectorStatus, providerID string) *connectorStatus {
	if providerID == "" {
		providerID = "webgallery"
	}
	connector := byConnector[providerID]
	if connector == nil {
		connector = &connectorStatus{
			ID:           providerID,
			DisplayName:  connectorDisplayName(providerID),
			Health:       "idle",
			State:        "Idle",
			Sources:      []connectorSourceStatus{},
			RecentRuns:   []connectorRunStatus{},
			RecentErrors: []connectorErrorStatus{},
		}
		byConnector[providerID] = connector
	}
	return connector
}

func applyConnectorCount(counts *connectorCounts, status string, count int64) {
	switch status {
	case "downloaded":
		counts.Downloaded += count
	case "failed":
		counts.Failed += count
	case "pending":
		counts.Pending += count
	}
	counts.Total += count
}

func connectorHealth(connector connectorStatus) (string, string) {
	runStatus := ""
	if len(connector.RecentRuns) > 0 {
		runStatus = connector.RecentRuns[0].Status
	}
	if connector.hasState {
		return connectorHealthFromStatuses(connector.lastStatus, runStatus, connector.Counts.Failed > 0 || len(connector.RecentErrors) > 0)
	}

	switch normalizeConnectorStatus(runStatus) {
	case "running":
		return "syncing", "Syncing"
	case "failed":
		return "error", "Needs review"
	case "completed_with_errors":
		return "degraded", "Degraded"
	case "completed":
		if connector.Counts.Failed > 0 || len(connector.RecentErrors) > 0 {
			return "degraded", "Degraded"
		}
		return "healthy", "Healthy"
	default:
		return "idle", "Not synced"
	}
}

func connectorHealthFromStatuses(lastStatus string, runStatus string, hasFailures bool) (string, string) {
	if normalizeConnectorStatus(runStatus) == "running" {
		return "syncing", "Syncing"
	}

	status := normalizeConnectorStatus(lastStatus)
	if status == "" {
		status = normalizeConnectorStatus(runStatus)
	}

	switch status {
	case "running":
		return "syncing", "Syncing"
	case "completed":
		if hasFailures {
			return "degraded", "Degraded"
		}
		return "healthy", "Healthy"
	case "completed_with_errors":
		return "degraded", "Degraded"
	case "failed", "permission_halt":
		return "error", "Needs review"
	case "idle":
		return "idle", "Not synced"
	default:
		return "degraded", "Needs review"
	}
}

func normalizeConnectorStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "not_synced", "never_synced", "idle":
		return "idle"
	case "running", "syncing", "in_progress", "started":
		return "running"
	case "completed", "complete", "success", "succeeded", "ok", "healthy":
		return "completed"
	case "completed_with_errors", "partial", "partial_success", "degraded":
		return "completed_with_errors"
	case "failed", "failure", "error", "errored":
		return "failed"
	case "permission_halt", "permission_halted", "permission", "halted", "needs_review":
		return "permission_halt"
	default:
		return status
	}
}

func extractionRunLastSync(run database.ExtractionRun) *time.Time {
	if run.EndTime != nil && !run.EndTime.IsZero() {
		return run.EndTime
	}
	if run.StartTime != nil && !run.StartTime.IsZero() {
		return run.StartTime
	}
	return nil
}

func maxTime(current *time.Time, candidate time.Time) *time.Time {
	if candidate.IsZero() {
		return current
	}
	if current == nil || candidate.After(*current) {
		value := candidate
		return &value
	}
	return current
}

func strconvUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func connectorProviderIDFromSource(sourcePage string, storedURL string) string {
	if sourcePage != "" {
		if providerID := connectorProviderIDFromSourcePage(sourcePage); providerID != "unknown" {
			return providerID
		}
	}
	if providerID := providerIDFromDedupeKey(storedURL); providerID != "" {
		return providerID
	}
	return "unknown"
}

func connectorProviderIDFromStored(providerID string) string {
	providerID = strings.TrimSpace(providerID)
	switch providerID {
	case "", "sight.photo":
		return "unknown"
	default:
		return providerID
	}
}

func connectorProviderIDFromSourcePage(sourcePage string) string {
	if sourcePage == "" {
		return "unknown"
	}
	parsed, err := url.Parse(sourcePage)
	if err != nil || parsed.Hostname() == "" {
		return sourcePage
	}
	switch strings.TrimPrefix(parsed.Hostname(), "www.") {
	case "t.me", "telegram.me":
		return "telegram"
	default:
		return "webgallery"
	}
}

func connectorDisplayName(providerID string) string {
	switch providerID {
	case "webgallery":
		return "Web Gallery"
	case "telegram":
		return "Telegram"
	default:
		return providerDisplayName(providerID)
	}
}

func connectorSourceID(sourcePage string, storedURL string, providerID string) string {
	if sourcePage != "" {
		return sourcePage
	}
	if providerID != "" && providerID != "unknown" {
		return providerID
	}
	if storedURL != "" {
		return storedURL
	}
	return "unknown"
}

func connectorSourceDisplayName(sourceID string, providerID string) string {
	if sourceID == "" || sourceID == providerID || sourceID == "unknown" {
		return connectorDisplayName(providerID)
	}
	return sourceDisplayName(sourceID)
}

func providerIDFromDedupeKey(value string) string {
	prefix, _, ok := strings.Cut(value, ":")
	if !ok || prefix == "" {
		return ""
	}
	if prefix == "http" || prefix == "https" {
		return ""
	}
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return ""
	}
	return prefix
}

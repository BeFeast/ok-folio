package api

import (
	"net/http"
	"sort"
	"strconv"
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
	ID               uint       `json:"id"`
	StartTime        time.Time  `json:"start_time"`
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

func (s *Server) handleConnectorStatus(w http.ResponseWriter, r *http.Request) {
	sourceStats, err := s.db.GetConnectorSourceStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector source stats")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	runs, err := s.db.GetRecentRuns(5)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector runs")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	recentErrors, err := s.db.GetRecentConnectorErrors(10)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch connector errors")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector status")
		return
	}

	connectors := buildConnectorStatuses(sourceStats, runs, recentErrors)
	s.writeJSON(w, http.StatusOK, connectorStatusResponse{Connectors: connectors})
}

func buildConnectorStatuses(sourceStats []database.ConnectorSourceStats, runs []database.ExtractionRun, recentErrors []database.ConnectorError) []connectorStatus {
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
	}
	sourceIndex := make(map[string]*connectorSourceStatus)

	for _, stat := range sourceStats {
		providerID := connectorProviderIDFromSourcePage(stat.SourcePage)
		if providerID == "unknown" {
			providerID = "webgallery"
		}
		connector := ensureConnectorStatus(byConnector, providerID)

		source := sourceIndex[stat.SourcePage]
		if source == nil {
			connector.Sources = append(connector.Sources, connectorSourceStatus{
				ID:          stat.SourcePage,
				DisplayName: sourceDisplayName(stat.SourcePage),
				ProviderID:  providerID,
			})
			source = &connector.Sources[len(connector.Sources)-1]
			sourceIndex[stat.SourcePage] = source
		}

		applyConnectorCount(&source.Counts, stat.Status, stat.Count)
		applyConnectorCount(&connector.Counts, stat.Status, stat.Count)
		if stat.LastActivity != nil && !stat.LastActivity.IsZero() {
			source.LastSync = maxTime(source.LastSync, *stat.LastActivity)
			connector.LastSync = maxTime(connector.LastSync, *stat.LastActivity)
		}
	}

	webgallery := ensureConnectorStatus(byConnector, "webgallery")
	for _, run := range runs {
		webgallery.RecentRuns = append(webgallery.RecentRuns, connectorRunStatus{
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
		if run.EndTime != nil {
			webgallery.LastSync = maxTime(webgallery.LastSync, *run.EndTime)
		} else {
			webgallery.LastSync = maxTime(webgallery.LastSync, run.StartTime)
		}
	}

	for _, connectorError := range recentErrors {
		providerID := connectorProviderIDFromSourcePage(connectorError.SourcePage)
		if providerID == "unknown" {
			providerID = "webgallery"
		}
		connector := ensureConnectorStatus(byConnector, providerID)
		message := connectorError.ErrorMessage
		if message == "" {
			message = "Download failed"
		}
		connector.RecentErrors = append(connector.RecentErrors, connectorErrorStatus{
			ID:         strconvUint(connectorError.ID),
			SourceID:   connectorError.SourcePage,
			Source:     sourceDisplayName(connectorError.SourcePage),
			Title:      connectorError.Title,
			Message:    message,
			OccurredAt: connectorError.OccurredAt,
		})
		connector.LastSync = maxTime(connector.LastSync, connectorError.OccurredAt)
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
	if len(connector.RecentRuns) > 0 && connector.RecentRuns[0].Status == "running" {
		return "syncing", "Syncing"
	}
	if len(connector.RecentRuns) > 0 && connector.RecentRuns[0].Status == "failed" {
		return "error", "Needs review"
	}
	if connector.Counts.Failed > 0 || len(connector.RecentErrors) > 0 {
		return "degraded", "Degraded"
	}
	if connector.LastSync == nil {
		return "idle", "Not synced"
	}
	return "healthy", "Healthy"
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

func strconvUint(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func connectorProviderIDFromSourcePage(sourcePage string) string {
	providerID := providerIDFromSourcePage(sourcePage)
	switch providerID {
	case "t.me", "telegram.me":
		return "telegram"
	default:
		return providerID
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

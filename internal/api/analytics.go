package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// handleStatsTimeline returns historical stats aggregated by time period
func (s *Server) handleStatsTimeline(w http.ResponseWriter, r *http.Request) {
	// Get period from query (default: daily)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "daily"
	}

	// Get days back (default: 7)
	daysStr := r.URL.Query().Get("days")
	days := 7
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
		days = d
	}

	// Get runs within the time period
	runs, err := s.db.GetRecentRuns(days * 10) // Get more runs to aggregate
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get timeline data")
		s.writeError(w, http.StatusInternalServerError, "Failed to get timeline data")
		return
	}

	// Aggregate by day
	timeline := make(map[string]map[string]int)
	for _, run := range runs {
		if run.Status != "completed" {
			continue
		}

		dateKey := run.StartTime.Format("2006-01-02")
		if timeline[dateKey] == nil {
			timeline[dateKey] = make(map[string]int)
		}

		timeline[dateKey]["downloaded"] += run.PhotosDownloaded
		timeline[dateKey]["skipped"] += run.PhotosSkipped
		timeline[dateKey]["failed"] += run.PhotosFailed
		timeline[dateKey]["runs"]++
	}

	// Convert to array format for charts
	type TimelineEntry struct {
		Date       string `json:"date"`
		Downloaded int    `json:"downloaded"`
		Skipped    int    `json:"skipped"`
		Failed     int    `json:"failed"`
		Runs       int    `json:"runs"`
	}

	var result []TimelineEntry
	for date, data := range timeline {
		result = append(result, TimelineEntry{
			Date:       date,
			Downloaded: data["downloaded"],
			Skipped:    data["skipped"],
			Failed:     data["failed"],
			Runs:       data["runs"],
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"timeline": result,
		"period":   period,
		"days":     days,
	})
}

// handleTopArtists returns artists with the most photos
func (s *Server) handleTopArtists(w http.ResponseWriter, r *http.Request) {
	// Get limit from query (default: 10)
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
		limit = l
	}

	artists, err := s.db.GetTopArtists(limit)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get top artists")
		s.writeError(w, http.StatusInternalServerError, "Failed to get top artists")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"artists": artists,
		"limit":   limit,
	})
}

// handleFailedPhotos returns photos that failed to download
func (s *Server) handleFailedPhotos(w http.ResponseWriter, r *http.Request) {
	// Get limit from query (default: 20)
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	photos, err := s.db.GetFailedPhotos(limit)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get failed photos")
		s.writeError(w, http.StatusInternalServerError, "Failed to get failed photos")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"photos": photos,
		"count":  len(photos),
	})
}

// handleRetryPhoto retries downloading a failed photo
func (s *Server) handleRetryPhoto(w http.ResponseWriter, r *http.Request) {
	// Get photo ID from URL
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Reset the photo status to pending
	if err := s.db.ResetPhotoStatus(id); err != nil {
		s.logger.Error().Err(err).Uint64("photo_id", id).Msg("Failed to reset photo status")
		s.writeError(w, http.StatusInternalServerError, "Failed to reset photo status")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Photo status reset to pending",
		"photo_id": id,
	})
}

// handleRunPhotos returns photos downloaded during a specific extraction run
func (s *Server) handleRunPhotos(w http.ResponseWriter, r *http.Request) {
	// Get run ID from URL
	runIDStr := chi.URLParam(r, "id")
	runID, err := strconv.ParseUint(runIDStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid run ID")
		return
	}

	limit, offset := s.parsePagination(r)

	photos, total, err := s.db.GetPhotosByRunID(runID, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Uint64("run_id", runID).Msg("Failed to get photos for run")
		s.writeError(w, http.StatusInternalServerError, "Failed to get photos for run")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"photos": photos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"run_id": runID,
	})
}

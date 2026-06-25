package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleStatsStream streams stats updates via Server-Sent Events
func (s *Server) handleStatsStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a ticker for periodic updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send initial stats immediately
	if err := s.sendStatsEvent(w); err != nil {
		return
	}

	// Stream updates
	for {
		select {
		case <-ticker.C:
			if err := s.sendStatsEvent(w); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		case <-s.ctx.Done():
			return
		}
	}
}

// sendStatsEvent sends a single stats event
func (s *Server) sendStatsEvent(w http.ResponseWriter) error {
	stats, err := s.db.GetDownloadStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get stats for SSE")
		return err
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	return nil
}

// handleWorkerStatus returns the current status of extraction workers
func (s *Server) handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	queueLen := len(s.jobQueue)
	queueCap := cap(s.jobQueue)

	status := map[string]interface{}{
		"total_workers":    MaxConcurrentExtractions,
		"queue_size":       queueLen,
		"queue_capacity":   queueCap,
		"workers_busy":     queueLen,
		"workers_idle":     MaxConcurrentExtractions - queueLen,
		"queue_utilization": float64(queueLen) / float64(queueCap) * 100,
	}

	s.writeJSON(w, http.StatusOK, status)
}

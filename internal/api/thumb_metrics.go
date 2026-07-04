package api

import (
	"net/http"
	"strings"
	"sync/atomic"
)

// Thumbnail source tiers. Serving a thumbnail resolves through these tiers in
// order (Valkey hot cache, own disk derivative cache, freshly generated from the
// original, and finally the optional read-only legacy PhotoPrism storage
// fallback). The counters make each tier's usage visible so operators can tell
// whether the legacy storage mount is still hit before dropping it.
const (
	thumbnailTierValkeyHit     = "valkey-hit"
	thumbnailTierDiskHit       = "disk-hit"
	thumbnailTierGenerated     = "generated"
	thumbnailTierLegacyStorage = "legacy-storage"
)

// thumbnailTierMetrics is a tiny lock-free counter set for thumbnail source
// tiers. It is deliberately a small internal helper rather than a full metrics
// framework; the counts are process-lifetime totals surfaced on the stats API at
// GET /api/v1/stats/thumbnail-tiers.
type thumbnailTierMetrics struct {
	valkeyHit     atomic.Int64
	diskHit       atomic.Int64
	generated     atomic.Int64
	legacyStorage atomic.Int64
}

func (m *thumbnailTierMetrics) record(tier string) {
	switch tier {
	case thumbnailTierValkeyHit:
		m.valkeyHit.Add(1)
	case thumbnailTierDiskHit:
		m.diskHit.Add(1)
	case thumbnailTierGenerated:
		m.generated.Add(1)
	case thumbnailTierLegacyStorage:
		m.legacyStorage.Add(1)
	}
}

func (m *thumbnailTierMetrics) snapshot() map[string]int64 {
	return map[string]int64{
		"valkey_hit":     m.valkeyHit.Load(),
		"disk_hit":       m.diskHit.Load(),
		"generated":      m.generated.Load(),
		"legacy_storage": m.legacyStorage.Load(),
	}
}

// handleThumbnailTiers exposes process-lifetime counts of how thumbnail bytes
// were sourced. Operators use the `legacy_storage` counter to measure whether
// the optional legacy PhotoPrism storage fallback is still hit; once it holds at
// zero across a reconcile window the read-only storage mount can be dropped.
func (s *Server) handleThumbnailTiers(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tiers":                              s.thumbnailTiers.snapshot(),
		"legacy_storage_fallback_configured": strings.TrimSpace(s.cfg.Storage.LegacyThumbDirectory) != "",
	})
}

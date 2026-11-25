package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ConnectorStatusPath is the read-only app endpoint the optional live probe
// queries. The probe only ever issues a GET against it.
const ConnectorStatusPath = "/api/v1/streams/connectors/status"

// webgallerySourceIDRe matches the per-source webgallery:<id> shape, e.g.
// webgallery:1. The :<id> suffix is mandatory: the bare aggregate provider id
// "webgallery" does NOT prove any per-source id is surfaced, so accepting it
// would let the live probe pass without the evidence it promises.
var webgallerySourceIDRe = regexp.MustCompile(`^webgallery:[0-9A-Za-z._-]+$`)

// liveConnector is the subset of the connector-status response the probe reads.
type liveConnector struct {
	ID       string             `json:"id"`
	LastSync *time.Time         `json:"last_sync"`
	Sources  []liveConnectorSrc `json:"sources"`
}

type liveConnectorSrc struct {
	ID         string `json:"id"`
	ProviderID string `json:"provider_id"`
}

type liveConnectorStatus struct {
	Connectors []liveConnector `json:"connectors"`
}

// ProbeConnectors performs the optional live probe: a single read-only GET
// against baseURL + ConnectorStatusPath. It confirms a webgallery per-source id
// of the webgallery:<id> shape and Telegram freshness are surfaced by the
// running app. It never mutates anything. The result is reported separately
// from the offline checks; a failed or unreachable probe does not affect the
// offline pass.
func ProbeConnectors(ctx context.Context, client *http.Client, baseURL string) Result {
	res := Result{ID: "live-connector-state", Title: "Live probe: connector status surfaces webgallery:<id> and Telegram freshness"}

	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	endpoint := strings.TrimRight(baseURL, "/") + ConnectorStatusPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		res.Status = StatusWarn
		res.Summary = fmt.Sprintf("could not build live probe request: %v", err)
		return res
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Status = StatusWarn
		res.Summary = fmt.Sprintf("live probe skipped: %s unreachable", endpoint)
		res.Evidence = []string{fmt.Sprintf("GET %s: %v", ConnectorStatusPath, err)}
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		res.Status = StatusWarn
		res.Summary = fmt.Sprintf("live probe got HTTP %d from %s", resp.StatusCode, ConnectorStatusPath)
		return res
	}

	var status liveConnectorStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		res.Status = StatusWarn
		res.Summary = fmt.Sprintf("live probe could not decode connector status: %v", err)
		return res
	}

	return evaluateLiveConnectors(status, res)
}

// evaluateLiveConnectors turns a decoded connector-status response into a probe
// result. Split out so it can be tested without a live server.
func evaluateLiveConnectors(status liveConnectorStatus, res Result) Result {
	var webgallerySourceID string
	telegramPresent := false
	var telegramSync *time.Time

	for _, connector := range status.Connectors {
		if connector.ID == "telegram" {
			telegramPresent = true
			telegramSync = connector.LastSync
		}
		for _, src := range connector.Sources {
			if webgallerySourceID == "" && matchesWebgallerySourceID(src) {
				webgallerySourceID = firstNonEmpty(src.ID, src.ProviderID)
			}
		}
	}

	var missing []string
	if webgallerySourceID != "" {
		res.Evidence = append(res.Evidence, fmt.Sprintf("connector status exposes webgallery source id %q", webgallerySourceID))
	} else {
		missing = append(missing, "no webgallery:<id> source id surfaced")
	}
	if telegramPresent {
		res.Evidence = append(res.Evidence, fmt.Sprintf("telegram connector last_sync freshness = %s", formatSync(telegramSync)))
	} else {
		missing = append(missing, "telegram connector not surfaced")
	}

	if len(missing) > 0 {
		res.Status = StatusWarn
		res.Summary = "live connector status incomplete: " + strings.Join(missing, "; ")
		return res
	}
	res.Status = StatusPass
	res.Summary = "running app surfaces webgallery:<id> and Telegram freshness"
	return res
}

func matchesWebgallerySourceID(src liveConnectorSrc) bool {
	return webgallerySourceIDRe.MatchString(src.ID) || webgallerySourceIDRe.MatchString(src.ProviderID)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func formatSync(t *time.Time) string {
	if t == nil {
		return "never (no successful sync recorded yet)"
	}
	return t.UTC().Format(time.RFC3339)
}

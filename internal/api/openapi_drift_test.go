package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

const openAPISpecPath = "../../api/openapi.yaml"

// undocumentedRoutePrefixes lists route prefixes that are registered on the
// server but intentionally NOT documented in api/openapi.yaml. The spec covers
// only the read/curation surface consumed by clients (gallery, photos,
// favorites, inbox, folios); extraction, analytics, admin, settings, and
// streaming endpoints are internal/operator-facing and excluded on purpose.
//
// A NEW route under /api/v1 fails TestOpenAPISpecMatchesRegisteredRoutes
// unless it is either documented in the spec or its prefix is added here.
// Prefixes are matched against normalized routes (path params collapsed to
// "{}", no trailing slash).
var undocumentedRoutePrefixes = []string{
	"/api/v1/runs",              // extraction run history
	"/api/v1/extract",           // extraction triggers (admin)
	"/api/v1/stats/",            // analytics sub-resources (timeline, artists/top, thumbnail-tiers); bare /api/v1/stats IS documented
	"/api/v1/workers/",          // worker status (admin)
	"/api/v1/photos/failed",     // failed photo management (admin)
	"/api/v1/photos/retry",      // failed photo management (admin)
	"/api/v1/photos/today",      // legacy dashboard widgets
	"/api/v1/photos/week",       // legacy dashboard widgets
	"/api/v1/artists",           // legacy artist browsing endpoints
	"/api/v1/pieces",            // manual piece upload (dashboard-only)
	"/api/v1/catalog/bulk-edit", // bulk metadata edit (dashboard-only)
	"/api/v1/gallery/decision",  // gallery architecture prototype metadata
	"/api/v1/streams/",          // connector status (streams surface)
	"/api/v1/settings/",         // connector source management (settings surface)
	"/api/v1/photoprism/",       // legacy PhotoPrism escape hatch
	"/api/v1/stream/",           // SSE real-time streaming
}

var pathParamPattern = regexp.MustCompile(`\{[^{}]*\}`)

// normalizeRoute makes chi route templates and OpenAPI path templates
// comparable: path parameter names are collapsed to "{}" ({id}, {photoId},
// {page} all become {}), chi mount artifacts ("/*/") are flattened, and
// trailing slashes are dropped.
func normalizeRoute(route string) string {
	route = strings.ReplaceAll(route, "/*/", "/")
	if route != "/" {
		route = strings.TrimSuffix(route, "/")
	}
	return pathParamPattern.ReplaceAllString(route, "{}")
}

func isAllowlistedUndocumented(route string) bool {
	for _, prefix := range undocumentedRoutePrefixes {
		normalized := normalizeRoute(prefix)
		if route == normalized || strings.HasPrefix(route, normalized+"/") || strings.HasPrefix(route, prefix) {
			return true
		}
	}
	return false
}

// specOperations loads api/openapi.yaml and returns the set of documented
// "METHOD /normalized/path" pairs. It also validates the invariants the spec
// must hold: it parses as YAML, every path is /health or under /api/v1, and
// every operation defines a success response (200, 201, or 204).
func specOperations(t *testing.T) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(openAPISpecPath))
	if err != nil {
		t.Fatalf("failed to read OpenAPI spec %s: %v", openAPISpecPath, err)
	}

	var doc struct {
		OpenAPI string                    `yaml:"openapi"`
		Paths   map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("OpenAPI spec %s is not valid YAML: %v", openAPISpecPath, err)
	}
	if doc.OpenAPI == "" {
		t.Fatalf("OpenAPI spec %s has no top-level 'openapi' version field", openAPISpecPath)
	}
	if len(doc.Paths) == 0 {
		t.Fatalf("OpenAPI spec %s documents no paths", openAPISpecPath)
	}

	httpMethods := []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"}

	documented := make(map[string]bool)
	for path, item := range doc.Paths {
		if path != "/health" && !strings.HasPrefix(path, "/api/v1/") {
			t.Errorf("documented path %q must be /health or start with /api/v1/", path)
		}
		operations := 0
		for _, method := range httpMethods {
			opAny, ok := item[method]
			if !ok {
				continue
			}
			operations++
			documented[strings.ToUpper(method)+" "+normalizeRoute(path)] = true

			op, ok := opAny.(map[string]any)
			if !ok {
				t.Errorf("operation %s %s is not a mapping", strings.ToUpper(method), path)
				continue
			}
			responses, ok := op["responses"].(map[string]any)
			if !ok {
				t.Errorf("operation %s %s has no responses mapping", strings.ToUpper(method), path)
				continue
			}
			if _, ok := responses["200"]; !ok {
				if _, ok := responses["201"]; !ok {
					if _, ok := responses["204"]; !ok {
						t.Errorf("operation %s %s defines no success response (200, 201, or 204)", strings.ToUpper(method), path)
					}
				}
			}
		}
		if operations == 0 {
			t.Errorf("documented path %q has no operations", path)
		}
	}
	return documented
}

// registeredOperations builds the real chi router (via the same in-memory
// server construction the rest of the API tests use) and walks it, returning
// the set of registered "METHOD /normalized/path" pairs under /health and
// /api/v1. Dashboard/static routes are ignored.
func registeredOperations(t *testing.T) map[string]bool {
	t.Helper()

	server, _ := setupTestServer(t)

	registered := make(map[string]bool)
	walkFn := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		normalized := normalizeRoute(route)
		if normalized != "/health" && !strings.HasPrefix(normalized, "/api/v1/") {
			return nil
		}
		registered[method+" "+normalized] = true
		return nil
	}
	if err := chi.Walk(server.router, walkFn); err != nil {
		t.Fatalf("failed to walk chi router: %v", err)
	}
	if len(registered) == 0 {
		t.Fatal("chi.Walk found no routes under /health or /api/v1 - route registration or walking is broken")
	}
	return registered
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestOpenAPISpecMatchesRegisteredRoutes is the drift check between
// api/openapi.yaml and the routes actually registered on the chi router.
//
// It fails when a registered route is neither documented nor covered by the
// undocumentedRoutePrefixes allowlist, and when the spec documents an
// operation that the router does not register.
func TestOpenAPISpecMatchesRegisteredRoutes(t *testing.T) {
	documented := specOperations(t)
	registered := registeredOperations(t)

	var undocumented []string
	for _, op := range sortedKeys(registered) {
		if documented[op] {
			continue
		}
		route := strings.SplitN(op, " ", 2)[1]
		if isAllowlistedUndocumented(route) {
			continue
		}
		undocumented = append(undocumented, op)
	}
	if len(undocumented) > 0 {
		t.Errorf(
			"routes registered on the server but missing from %s (document them or add their prefix to undocumentedRoutePrefixes with a justification):\n  %s",
			openAPISpecPath, strings.Join(undocumented, "\n  "),
		)
	}

	var unregistered []string
	for _, op := range sortedKeys(documented) {
		if !registered[op] {
			unregistered = append(unregistered, op)
		}
	}
	if len(unregistered) > 0 {
		t.Errorf(
			"operations documented in %s but not registered on the server (stale spec entries):\n  %s",
			openAPISpecPath, strings.Join(unregistered, "\n  "),
		)
	}

	if t.Failed() {
		t.Logf("registered routes considered: %s", fmt.Sprintf("%d", len(registered)))
	}
}

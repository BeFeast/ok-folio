package api

import (
	"io/fs"
	"net/http"
	"strings"

	"ok-folio/internal/dashboard"
)

// setupDashboardRoutes adds routes for serving the embedded dashboard
func (s *Server) setupDashboardRoutes() {
	// Get the embedded dashboard files
	distFS, err := dashboard.GetDist()
	if err != nil {
		s.logger.Warn().Err(err).Msg("Dashboard not available (build frontend first)")
		return
	}

	// Create a file server for the dist directory
	fileServer := http.FileServer(http.FS(distFS))

	// Serve dashboard on all non-API routes
	s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Don't serve dashboard for API or health check routes
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
			http.NotFound(w, r)
			return
		}

		// Try to serve the requested file
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if file exists
		if _, err := fs.Stat(distFS, path); err != nil {
			if !isDashboardRoute(r.URL.Path) {
				http.NotFound(w, r)
				return
			}

			// Serve index.html for known client-side routes.
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})

	s.logger.Info().Msg("Dashboard routes configured")
}

func isDashboardRoute(path string) bool {
	switch {
	case path == "/":
		return true
	case path == "/today":
		return true
	case path == "/week":
		return true
	case path == "/analytics":
		return true
	case path == "/failed":
		return true
	case path == "/search":
		return true
	case path == "/artists":
		return true
	case strings.HasPrefix(path, "/artists/"):
		return true
	case strings.HasPrefix(path, "/pieces/"):
		return true
	case strings.HasPrefix(path, "/runs/"):
		return true
	default:
		return false
	}
}

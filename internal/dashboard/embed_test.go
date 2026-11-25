//go:build !dev
// +build !dev

package dashboard

import (
	"io/fs"
	"testing"
)

// TestGetDistHasIndex guards the invariant that keeps `go build`/`go test`
// working on a clean checkout: `//go:embed dist/*` needs at least one matching
// file, so a fallback index.html must always be embedded. If the committed
// placeholder is removed (and no frontend build is staged), this fails fast
// with a clear message instead of an opaque "pattern dist/*: no matching files"
// build error across every package that imports the dashboard.
func TestGetDistHasIndex(t *testing.T) {
	distFS, err := GetDist()
	if err != nil {
		t.Fatalf("GetDist() returned error: %v", err)
	}

	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		t.Fatalf("embedded dist is missing index.html: %v", err)
	}

	data, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		t.Fatalf("reading embedded index.html: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("embedded index.html is empty")
	}
}

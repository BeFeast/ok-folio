package derivatives

import (
	"os"
	"path/filepath"
	"strings"

	"ok-folio/internal/database"
)

// LegacyThumbnailName is the deterministic file name OK Folio looks up inside the
// optional legacy PhotoPrism storage fallback directory for a given piece. It is
// keyed by the piece's stable content token so a fallback thumbnail can be
// matched without a live PhotoPrism query. The token changes when the original's
// content changes, so a stale legacy thumbnail is never served for new bytes.
func LegacyThumbnailName(photo *database.DownloadedPhoto, validator string) string {
	return ContentToken(photo, validator) + ".jpg"
}

// LegacyThumbnail performs a read-only lookup of a pre-rendered thumbnail from
// the optional legacy PhotoPrism storage fallback directory. It returns the file
// bytes when a matching thumbnail exists and (nil, false) when the fallback is
// disabled (empty dir), the piece is nil, the file is absent, or it cannot be
// read.
//
// The lookup is intentionally read-only: the legacy PhotoPrism storage mount is
// kernel-enforced read-only during retirement, and OK Folio must never create,
// write, or modify anything under dir. This function only opens the file for
// reading, so a writable regression would still leave the legacy dataset
// untouched.
func LegacyThumbnail(dir string, photo *database.DownloadedPhoto, validator string) ([]byte, bool) {
	dir = strings.TrimSpace(dir)
	if dir == "" || photo == nil {
		return nil, false
	}
	path := filepath.Join(dir, LegacyThumbnailName(photo, validator))
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

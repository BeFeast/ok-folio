package derivatives

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"ok-folio/internal/database"
)

func TestLegacyThumbnailDisabledWhenDirEmpty(t *testing.T) {
	photo := &database.DownloadedPhoto{ID: 1, ContentHash: bytes.Repeat([]byte{0x11}, 32)}
	if _, ok := LegacyThumbnail("", photo, ""); ok {
		t.Fatalf("expected no legacy fallback when dir is empty")
	}
	if _, ok := LegacyThumbnail("   ", photo, ""); ok {
		t.Fatalf("expected no legacy fallback when dir is blank")
	}
}

func TestLegacyThumbnailMissReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	photo := &database.DownloadedPhoto{ID: 2, ContentHash: bytes.Repeat([]byte{0x22}, 32)}
	if _, ok := LegacyThumbnail(dir, photo, ""); ok {
		t.Fatalf("expected miss for an unpopulated legacy dir")
	}
	if _, ok := LegacyThumbnail(dir, nil, ""); ok {
		t.Fatalf("expected miss for a nil photo")
	}
}

func TestLegacyThumbnailServesMatchingFileReadOnly(t *testing.T) {
	dir := t.TempDir()
	photo := &database.DownloadedPhoto{ID: 3, ContentHash: bytes.Repeat([]byte{0x33}, 32)}
	want := []byte("legacy-thumbnail-bytes")

	name := LegacyThumbnailName(photo, "")
	if err := os.WriteFile(filepath.Join(dir, name), want, 0o444); err != nil {
		t.Fatalf("seed legacy thumbnail: %v", err)
	}

	got, ok := LegacyThumbnail(dir, photo, "")
	if !ok {
		t.Fatalf("expected legacy fallback hit for a seeded thumbnail")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected legacy thumbnail bytes %q, got %q", want, got)
	}

	// The lookup must never create or modify files under the read-only legacy
	// mount. Only the seeded file may exist afterwards.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read legacy dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Fatalf("expected legacy dir untouched with only %q, got %v", name, entries)
	}
}

func TestLegacyThumbnailKeyChangesWithContent(t *testing.T) {
	first := &database.DownloadedPhoto{ID: 4, ContentHash: bytes.Repeat([]byte{0x44}, 32)}
	second := &database.DownloadedPhoto{ID: 4, ContentHash: bytes.Repeat([]byte{0x45}, 32)}
	if LegacyThumbnailName(first, "") == LegacyThumbnailName(second, "") {
		t.Fatalf("expected legacy thumbnail name to change when content hash changes")
	}
}

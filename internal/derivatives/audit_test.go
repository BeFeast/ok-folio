package derivatives

import (
	"bytes"
	"context"
	"image"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/image/bmp"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

func TestAuditOriginalsClassifiesAndExcludes(t *testing.T) {
	db := setupWarmDB(t)
	storage := warmStorage(t)

	valid := createWarmPhoto(t, db, storage, "valid.jpg")
	// Formats the product accepts must audit as decodable, or exclude mode
	// would fail valid pieces out of the gallery and backfill.
	validPNG := createAuditPhoto(t, db, storage, "valid.png", encodedImage(t, png.Encode))
	validGIF := createAuditPhoto(t, db, storage, "valid.gif", encodedImage(t, func(w io.Writer, m image.Image) error {
		return gif.Encode(w, m, nil)
	}))
	validBMP := createAuditPhoto(t, db, storage, "valid.bmp", encodedImage(t, bmp.Encode))
	html := createAuditPhoto(t, db, storage, "html.jpg", []byte("<!DOCTYPE html><html><head><title>502</title></head><body>Bad Gateway</body></html>"))
	truncated := createAuditPhoto(t, db, storage, "truncated.jpg", truncatedJPEG(t))
	avif := createAuditPhoto(t, db, storage, "avif.jpg", append([]byte{0, 0, 0, 0x20}, []byte("ftypavif\x00\x00\x00\x00avifmif1miaf")...))
	empty := createAuditPhoto(t, db, storage, "empty.jpg", nil)
	missing := createAuditPhoto(t, db, storage, "missing.jpg", []byte("placeholder"))
	if err := os.Remove(filepath.Join(storage.BaseDirectory, "missing.jpg")); err != nil {
		t.Fatalf("remove missing original: %v", err)
	}

	result, err := AuditOriginals(context.Background(), db, storage, AuditOptions{BatchSize: 2}, zerolog.Nop())
	if err != nil {
		t.Fatalf("AuditOriginals failed: %v", err)
	}
	if result.Scanned != 9 || result.Decodable != 4 || result.Missing != 1 || result.Undecodable != 4 || result.Excluded != 0 {
		t.Fatalf("unexpected audit result: %#v", result)
	}

	classes := map[uint64]AuditFinding{}
	for _, finding := range result.Findings {
		classes[finding.PhotoID] = finding
	}
	assertClass(t, classes, html.ID, "non-image-payload-html")
	assertClass(t, classes, truncated.ID, "truncated-or-corrupt-jpeg")
	assertClass(t, classes, avif.ID, "unsupported-format-avif")
	assertClass(t, classes, empty.ID, "empty-file")
	if finding := classes[html.ID]; finding.FirstBytes == "" || finding.SniffedMIME == "" {
		t.Fatalf("expected populated sniff fields, got %#v", finding)
	}

	// Report-only mode must not change any statuses.
	for _, id := range []uint64{valid.ID, validPNG.ID, validGIF.ID, validBMP.ID, html.ID, truncated.ID, avif.ID, empty.ID, missing.ID} {
		assertStatus(t, db, id, "downloaded", "")
	}

	result, err = AuditOriginals(context.Background(), db, storage, AuditOptions{Exclude: true}, zerolog.Nop())
	if err != nil {
		t.Fatalf("AuditOriginals exclude pass failed: %v", err)
	}
	if result.Undecodable != 4 || result.Excluded != 4 {
		t.Fatalf("unexpected exclude result: %#v", result)
	}
	for _, finding := range result.Findings {
		if !finding.Excluded {
			t.Fatalf("expected finding excluded, got %#v", finding)
		}
	}

	for _, id := range []uint64{valid.ID, validPNG.ID, validGIF.ID, validBMP.ID, missing.ID} {
		assertStatus(t, db, id, "downloaded", "")
	}
	for _, id := range []uint64{html.ID, truncated.ID, avif.ID, empty.ID} {
		var stored database.DownloadedPhoto
		if err := db.First(&stored, id).Error; err != nil {
			t.Fatalf("load photo %d: %v", id, err)
		}
		if stored.Status != "failed" {
			t.Fatalf("expected photo %d excluded, got status %q", id, stored.Status)
		}
		if !strings.HasPrefix(stored.ErrorMessage, "undecodable original (") {
			t.Fatalf("expected classification in error message, got %q", stored.ErrorMessage)
		}
	}

	// Excluded rows must leave every downloaded-only sweep predicate.
	var sweepable int64
	if err := db.Model(&database.DownloadedPhoto{}).Where("status = ?", "downloaded").Count(&sweepable).Error; err != nil {
		t.Fatalf("count downloaded: %v", err)
	}
	if sweepable != 5 {
		t.Fatalf("expected 5 downloaded rows after exclusion, got %d", sweepable)
	}

	result, err = AuditOriginals(context.Background(), db, storage, AuditOptions{Exclude: true}, zerolog.Nop())
	if err != nil {
		t.Fatalf("AuditOriginals rerun failed: %v", err)
	}
	if result.Scanned != 5 || result.Undecodable != 0 {
		t.Fatalf("expected excluded rows to be skipped on rerun, got %#v", result)
	}
}

func TestAuditOriginalsHonorsLimit(t *testing.T) {
	db := setupWarmDB(t)
	storage := warmStorage(t)
	createWarmPhoto(t, db, storage, "one.jpg")
	createWarmPhoto(t, db, storage, "two.jpg")

	result, err := AuditOriginals(context.Background(), db, storage, AuditOptions{Limit: 1}, zerolog.Nop())
	if err != nil {
		t.Fatalf("AuditOriginals failed: %v", err)
	}
	if result.Scanned != 1 {
		t.Fatalf("expected 1 scanned row, got %#v", result)
	}
}

func TestClassifyUndecodable(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		size   int64
		want   string
	}{
		{"empty", nil, 0, "empty-file"},
		{"html", []byte("<html><body>error</body></html>"), 31, "non-image-payload-html"},
		{"text", []byte("not found"), 9, "non-image-payload-text"},
		{"heic", append([]byte{0, 0, 0, 0x18}, []byte("ftypheic")...), 24, "unsupported-format-heic"},
		{"avif", append([]byte{0, 0, 0, 0x20}, []byte("ftypavif")...), 32, "unsupported-format-avif"},
		{"bmff-other", append([]byte{0, 0, 0, 0x18}, []byte("ftypqt  ")...), 24, "unsupported-format-iso-bmff"},
		{"jxl-codestream", []byte{0xFF, 0x0A, 0x00}, 3, "unsupported-format-jxl"},
		{"jxl-container", []byte("\x00\x00\x00\x0cJXL \x0d\x0a\x87\x0a"), 12, "unsupported-format-jxl"},
		{"tiff", []byte("II*\x00garbage"), 11, "truncated-or-corrupt-tiff"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, 4, "truncated-or-corrupt-jpeg"},
		{"binary", []byte{0x00, 0x01, 0x02, 0x03}, 4, "unknown-payload"},
	}
	for _, tc := range cases {
		if got := ClassifyUndecodable(tc.header, tc.size); got != tc.want {
			t.Errorf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func assertClass(t *testing.T, findings map[uint64]AuditFinding, id uint64, want string) {
	t.Helper()
	finding, ok := findings[id]
	if !ok {
		t.Fatalf("expected finding for photo %d", id)
	}
	if finding.Class != want {
		t.Fatalf("expected class %q for photo %d, got %q", want, id, finding.Class)
	}
}

func assertStatus(t *testing.T, db *database.DB, id uint64, wantStatus, wantError string) {
	t.Helper()
	var stored database.DownloadedPhoto
	if err := db.First(&stored, id).Error; err != nil {
		t.Fatalf("load photo %d: %v", id, err)
	}
	if stored.Status != wantStatus || stored.ErrorMessage != wantError {
		t.Fatalf("photo %d: expected status %q error %q, got %q %q", id, wantStatus, wantError, stored.Status, stored.ErrorMessage)
	}
}

func createAuditPhoto(t *testing.T, db *database.DB, storage config.StorageConfig, name string, payload []byte) database.DownloadedPhoto {
	t.Helper()
	path := filepath.Join(storage.BaseDirectory, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir originals dir: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}
	photo := database.DownloadedPhoto{
		URL:          "https://example.test/" + name,
		FilePath:     name,
		FileName:     name,
		FileSize:     int64(len(payload)),
		Status:       "downloaded",
		DownloadedAt: ptrWarmTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("create photo: %v", err)
	}
	return photo
}

func encodedImage(t *testing.T, encode func(io.Writer, image.Image) error) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
	return buf.Bytes()
}

func truncatedJPEG(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "full.jpg")
	createWarmJPEG(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jpeg: %v", err)
	}
	return data[:len(data)/2]
}

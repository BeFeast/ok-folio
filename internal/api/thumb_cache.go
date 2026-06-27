package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

const thumbnailHotTierTTL = 24 * time.Hour

type thumbnailCache struct {
	dir      string
	maxBytes int64
}

type thumbnailCacheEntry struct {
	path string
	key  string
}

func newThumbnailCache(cfg config.StorageConfig) *thumbnailCache {
	dir := strings.TrimSpace(cfg.DerivativesDirectory)
	if dir == "" {
		dir = config.DefaultDerivativesDirectory
	}
	maxBytes := cfg.DerivativesMaxBytes
	if maxBytes == 0 {
		maxBytes = config.DefaultDerivativesMaxBytes
	}
	return &thumbnailCache{dir: dir, maxBytes: maxBytes}
}

func (c *thumbnailCache) entry(photo *database.DownloadedPhoto, width int, validator string) thumbnailCacheEntry {
	token := thumbnailContentToken(photo, validator)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", photo.ID, width, token)))
	name := fmt.Sprintf("%d-w%d-%s.jpg", photo.ID, width, token)
	shard := hex.EncodeToString(sum[:])
	return thumbnailCacheEntry{
		path: filepath.Join(c.dir, shard[:2], shard[2:4], name),
		key:  okfcache.ThumbKey(photo.ID, width, width) + ":" + token,
	}
}

func thumbnailContentToken(photo *database.DownloadedPhoto, validator string) string {
	if len(photo.ContentHash) > 0 {
		return hex.EncodeToString(photo.ContentHash)
	}
	return strings.Trim(validator, `"`)
}

func (c *thumbnailCache) read(entry thumbnailCacheEntry) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	data, err := os.ReadFile(entry.path)
	if err != nil {
		return nil, false
	}
	_ = os.Chtimes(entry.path, time.Now(), time.Now())
	return data, true
}

func (c *thumbnailCache) write(entry thumbnailCacheEntry, data []byte) error {
	if c == nil || len(data) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(entry.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(entry.path), ".thumb-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, entry.path); err != nil {
		return err
	}
	return c.prune()
}

func (c *thumbnailCache) prune() error {
	if c == nil || c.maxBytes <= 0 {
		return nil
	}

	type cachedFile struct {
		path    string
		size    int64
		modTime time.Time
	}
	var files []cachedFile
	var total int64
	err := filepath.WalkDir(c.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !c.isCacheFile(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		files = append(files, cachedFile{path: path, size: info.Size(), modTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return err
	}
	if total <= c.maxBytes {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	target := c.maxBytes * 9 / 10
	for _, file := range files {
		if total <= target {
			break
		}
		if err := os.Remove(file.path); err == nil {
			total -= file.size
		}
	}
	return nil
}

func (c *thumbnailCache) isCacheFile(path string) bool {
	if c == nil {
		return false
	}
	rel, err := filepath.Rel(c.dir, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 3 {
		return false
	}
	if !isLowerHex(parts[0], 2) || !isLowerHex(parts[1], 2) {
		return false
	}
	name := parts[2]
	if filepath.Ext(name) != ".jpg" {
		return false
	}
	stem := strings.TrimSuffix(name, ".jpg")
	id, rest, ok := strings.Cut(stem, "-w")
	if !ok || !isDecimal(id) {
		return false
	}
	width, token, ok := strings.Cut(rest, "-")
	if !ok || !isDecimal(width) || token == "" {
		return false
	}
	return isCacheToken(token)
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			if ch < 'a' || ch > 'f' {
				return false
			}
		}
	}
	return true
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isCacheToken(value string) bool {
	for _, ch := range value {
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) serveThumbnailFromCache(w http.ResponseWriter, r *http.Request, entry thumbnailCacheEntry) bool {
	if _, err := os.Stat(entry.path); err != nil {
		return false
	}
	if data, ok := s.cache.GetBytes(r.Context(), entry.key); ok {
		w.Header().Set("X-OK-Folio-Thumbnail-Cache", "valkey-hit")
		_, _ = w.Write(data)
		return true
	}
	if data, ok := s.thumbCache.read(entry); ok {
		s.cache.SetBytes(r.Context(), entry.key, data, thumbnailHotTierTTL)
		w.Header().Set("X-OK-Folio-Thumbnail-Cache", "disk-hit")
		_, _ = w.Write(data)
		return true
	}
	return false
}

func (s *Server) generateThumbnail(ctx context.Context, filePath string, size int) ([]byte, error) {
	imgFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer imgFile.Close()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return nil, err
	}

	thumbnail := imaging.Fit(img, size, size, imaging.Lanczos)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: ThumbnailQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) storeThumbnail(entry thumbnailCacheEntry, data []byte) {
	if err := s.thumbCache.write(entry, data); err != nil {
		s.logger.Warn().Err(err).Str("path", entry.path).Msg("Thumbnail disk cache write failed")
		return
	}
	s.cache.SetBytes(context.Background(), entry.key, data, thumbnailHotTierTTL)
}

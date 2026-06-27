package derivatives

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

const (
	DefaultThumbnailSize = 400
	MinThumbnailSize     = 64
	MaxThumbnailSize     = 1024
	ThumbnailQuality     = 82

	pruneDebounce = 5 * time.Second
)

type Cache struct {
	dir            string
	maxBytes       int64
	pruneMu        sync.Mutex
	pruneTimer     *time.Timer
	pruneRunning   bool
	pruneRequested bool
}

type Entry struct {
	Path string
	Key  string
}

func NewCache(cfg config.StorageConfig) *Cache {
	dir := strings.TrimSpace(cfg.DerivativesDirectory)
	if dir == "" {
		dir = config.DefaultDerivativesDirectory
	}
	maxBytes := cfg.DerivativesMaxBytes
	if maxBytes == 0 {
		maxBytes = config.DefaultDerivativesMaxBytes
	}
	return &Cache{dir: dir, maxBytes: maxBytes}
}

func NewCacheForDir(dir string, maxBytes int64) *Cache {
	return &Cache{dir: dir, maxBytes: maxBytes}
}

func (c *Cache) Entry(photo *database.DownloadedPhoto, width int, validator string) Entry {
	token := ContentToken(photo, validator)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", photo.ID, width, token)))
	name := fmt.Sprintf("%d-w%d-%s.jpg", photo.ID, width, token)
	shard := hex.EncodeToString(sum[:])
	return Entry{
		Path: filepath.Join(c.dir, shard[:2], shard[2:4], name),
		Key:  okfcache.ThumbKey(photo.ID, width, width) + ":" + token,
	}
}

func ContentToken(photo *database.DownloadedPhoto, validator string) string {
	if len(photo.ContentHash) > 0 {
		return hex.EncodeToString(photo.ContentHash)
	}
	return strings.Trim(validator, `"`)
}

func Validator(photo *database.DownloadedPhoto, filePath string) (string, error) {
	if len(photo.ContentHash) > 0 {
		return QuoteETag(fmt.Sprintf("%d-%s", photo.ID, hex.EncodeToString(photo.ContentHash))), nil
	}

	if filePath == "" {
		filePath = photo.FilePath
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	fileSize := photo.FileSize
	if fileSize <= 0 {
		fileSize = info.Size()
	}
	return QuoteETag(fmt.Sprintf("%d-%d-%d", photo.ID, fileSize, info.ModTime().UnixNano())), nil
}

func QuoteETag(value string) string {
	return `"` + value + `"`
}

func (c *Cache) Exists(entry Entry) bool {
	if c == nil {
		return false
	}
	_, err := os.Stat(entry.Path)
	return err == nil
}

func (c *Cache) Read(entry Entry) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, false
	}
	_ = os.Chtimes(entry.Path, time.Now(), time.Now())
	return data, true
}

func (c *Cache) Touch(entry Entry) {
	if c == nil {
		return
	}
	now := time.Now()
	_ = os.Chtimes(entry.Path, now, now)
}

func (c *Cache) Write(entry Entry, data []byte) error {
	if err := c.write(entry, data); err != nil {
		return err
	}
	c.SchedulePrune()
	return nil
}

func (c *Cache) write(entry Entry, data []byte) error {
	if c == nil || len(data) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(entry.Path), ".thumb-*.tmp")
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
	if err := os.Rename(tmpPath, entry.Path); err != nil {
		return err
	}
	return nil
}

func (c *Cache) SchedulePrune() {
	if c == nil || c.maxBytes <= 0 {
		return
	}

	c.pruneMu.Lock()
	defer c.pruneMu.Unlock()
	if c.pruneRunning {
		c.pruneRequested = true
		return
	}
	if c.pruneTimer == nil {
		c.pruneTimer = time.AfterFunc(pruneDebounce, c.runScheduledPrune)
		return
	}
	c.pruneTimer.Reset(pruneDebounce)
}

func (c *Cache) runScheduledPrune() {
	c.pruneMu.Lock()
	c.pruneTimer = nil
	c.pruneRunning = true
	c.pruneMu.Unlock()

	_ = c.Prune()

	c.pruneMu.Lock()
	c.pruneRunning = false
	if c.pruneRequested {
		c.pruneRequested = false
		c.pruneTimer = time.AfterFunc(pruneDebounce, c.runScheduledPrune)
	}
	c.pruneMu.Unlock()
}

func (c *Cache) Prune() error {
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
		if d.IsDir() || !c.IsCacheFile(path) {
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

func (c *Cache) SweepTempFiles(maxAge time.Duration) error {
	if c == nil {
		return nil
	}
	cutoff := time.Now().Add(-maxAge)
	err := filepath.WalkDir(c.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), ".thumb-") || !strings.HasSuffix(d.Name(), ".tmp") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if maxAge > 0 && info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *Cache) IsCacheFile(path string) bool {
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

func GenerateThumbnail(ctx context.Context, filePath string, size int) ([]byte, error) {
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

func ClampWidth(width int) int {
	if width < MinThumbnailSize {
		return MinThumbnailSize
	}
	if width > MaxThumbnailSize {
		return MaxThumbnailSize
	}
	return width
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

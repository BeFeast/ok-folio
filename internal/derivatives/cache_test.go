package derivatives

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ok-folio/internal/database"
)

func TestCacheWriteSchedulesPruneOffRequestPath(t *testing.T) {
	dir := t.TempDir()
	cache := NewCacheForDir(dir, 50)

	oldEntry := cache.Entry(&database.DownloadedPhoto{ID: 1, ContentHash: bytes.Repeat([]byte{0x11}, 32)}, 320, "")
	if err := os.MkdirAll(filepath.Dir(oldEntry.Path), 0o755); err != nil {
		t.Fatalf("mkdir old shard: %v", err)
	}
	if err := os.WriteFile(oldEntry.Path, bytes.Repeat([]byte("a"), 40), 0o644); err != nil {
		t.Fatalf("write old cache file: %v", err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldEntry.Path, oldTime, oldTime); err != nil {
		t.Fatalf("set old cache time: %v", err)
	}

	newEntry := cache.Entry(&database.DownloadedPhoto{ID: 2, ContentHash: bytes.Repeat([]byte{0x22}, 32)}, 320, "")
	if err := cache.Write(newEntry, bytes.Repeat([]byte("b"), 40)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if _, err := os.Stat(oldEntry.Path); err != nil {
		t.Fatalf("expected Write to return before pruning old cache file: %v", err)
	}
	if _, err := os.Stat(newEntry.Path); err != nil {
		t.Fatalf("expected new cache file: %v", err)
	}
}

func TestCacheScheduledPruneIsNotPostponedByRepeatedWrites(t *testing.T) {
	dir := t.TempDir()
	cache := NewCacheForDir(dir, 50)
	oldDebounce := pruneDebounce
	pruneDebounce = 20 * time.Millisecond
	t.Cleanup(func() {
		pruneDebounce = oldDebounce
	})

	oldEntry := cache.Entry(&database.DownloadedPhoto{ID: 1, ContentHash: bytes.Repeat([]byte{0x11}, 32)}, 320, "")
	if err := os.MkdirAll(filepath.Dir(oldEntry.Path), 0o755); err != nil {
		t.Fatalf("mkdir old shard: %v", err)
	}
	if err := os.WriteFile(oldEntry.Path, bytes.Repeat([]byte("a"), 40), 0o644); err != nil {
		t.Fatalf("write old cache file: %v", err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldEntry.Path, oldTime, oldTime); err != nil {
		t.Fatalf("set old cache time: %v", err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				cache.SchedulePrune()
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	newEntry := cache.Entry(&database.DownloadedPhoto{ID: 2, ContentHash: bytes.Repeat([]byte{0x22}, 32)}, 320, "")
	if err := cache.Write(newEntry, bytes.Repeat([]byte("b"), 40)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(oldEntry.Path); os.IsNotExist(err) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := os.Stat(oldEntry.Path); err == nil {
		t.Fatal("expected scheduled prune to run while writes continue")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat old cache file: %v", err)
	}
}

func TestCacheTouchUpdatesDiskMTime(t *testing.T) {
	dir := t.TempDir()
	cache := NewCacheForDir(dir, 1024)
	entry := cache.Entry(&database.DownloadedPhoto{ID: 1, ContentHash: bytes.Repeat([]byte{0x11}, 32)}, 320, "")
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0o755); err != nil {
		t.Fatalf("mkdir cache shard: %v", err)
	}
	if err := os.WriteFile(entry.Path, []byte("thumb"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(entry.Path, oldTime, oldTime); err != nil {
		t.Fatalf("set cache time: %v", err)
	}

	cache.Touch(entry)

	info, err := os.Stat(entry.Path)
	if err != nil {
		t.Fatalf("stat touched cache file: %v", err)
	}
	if !info.ModTime().After(oldTime) {
		t.Fatalf("expected touched cache mtime after %s, got %s", oldTime, info.ModTime())
	}
}

func TestCacheSweepTempFilesRemovesOnlyStaleThumbTemps(t *testing.T) {
	dir := t.TempDir()
	cache := NewCacheForDir(dir, 1024)
	stale := filepath.Join(dir, "aa", "bb", ".thumb-stale.tmp")
	fresh := filepath.Join(dir, "aa", "bb", ".thumb-fresh.tmp")
	other := filepath.Join(dir, "aa", "bb", "other.tmp")
	for _, path := range []string{stale, fresh, other} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir temp dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("tmp"), 0o644); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, staleTime, staleTime); err != nil {
		t.Fatalf("set stale temp time: %v", err)
	}

	if err := cache.SweepTempFiles(time.Hour); err != nil {
		t.Fatalf("SweepTempFiles failed: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale temp file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("expected fresh temp file to remain: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("expected unrelated temp file to remain: %v", err)
	}
}

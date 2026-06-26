package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func testClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), Protocol: 2})
	return NewForRedis(rdb, zerolog.Nop()), mr
}

func TestGetOrComputeCachesValue(t *testing.T) {
	ctx := context.Background()
	c, _ := testClient(t)
	var calls atomic.Int32

	compute := func(context.Context) (string, error) {
		calls.Add(1)
		return "catalog-page", nil
	}

	first, err := GetOrCompute(ctx, c, CatalogPrefix+"test", time.Minute, compute)
	if err != nil {
		t.Fatalf("first GetOrCompute failed: %v", err)
	}
	second, err := GetOrCompute(ctx, c, CatalogPrefix+"test", time.Minute, compute)
	if err != nil {
		t.Fatalf("second GetOrCompute failed: %v", err)
	}
	if first != "catalog-page" || second != "catalog-page" {
		t.Fatalf("expected cached value, got first=%q second=%q", first, second)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one compute call, got %d", calls.Load())
	}
}

func TestGetOrComputeUsesPerKeyLock(t *testing.T) {
	ctx := context.Background()
	c, _ := testClient(t)
	var calls atomic.Int32
	release := make(chan struct{})

	compute := func(context.Context) (string, error) {
		calls.Add(1)
		<-release
		return "shared", nil
	}

	var wg sync.WaitGroup
	results := make(chan string, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := GetOrCompute(ctx, c, CatalogPrefix+"locked", time.Minute, compute)
			if err != nil {
				errs <- err
				return
			}
			results <- value
		}()
	}

	for calls.Load() != 1 {
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("GetOrCompute failed: %v", err)
	}
	for value := range results {
		if value != "shared" {
			t.Fatalf("expected shared value, got %q", value)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected stampede lock to allow one compute call, got %d", calls.Load())
	}
}

func TestGetOrComputePassthroughOnCacheFailure(t *testing.T) {
	ctx := context.Background()
	c, mr := testClient(t)
	mr.Close()

	value, err := GetOrCompute(ctx, c, CatalogPrefix+"down", time.Minute, func(context.Context) (string, error) {
		return "computed", nil
	})
	if err != nil {
		t.Fatalf("passthrough compute failed: %v", err)
	}
	if value != "computed" {
		t.Fatalf("expected passthrough value, got %q", value)
	}
	if !c.Passthrough() {
		t.Fatalf("expected cache to switch to passthrough")
	}
}

func TestGetOrComputeComputeErrorIsReturned(t *testing.T) {
	ctx := context.Background()
	c, _ := testClient(t)
	expected := errors.New("db unavailable")

	_, err := GetOrCompute(ctx, c, CatalogPrefix+"error", time.Minute, func(context.Context) (string, error) {
		return "", expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected compute error, got %v", err)
	}
}

func TestInvalidation(t *testing.T) {
	ctx := context.Background()
	c, mr := testClient(t)
	if err := c.BumpEpoch(ctx); err != nil {
		t.Fatalf("BumpEpoch failed: %v", err)
	}
	got, err := mr.Get(EpochKey)
	if err != nil {
		t.Fatalf("expected epoch key: %v", err)
	}
	if got != "1" {
		t.Fatalf("expected epoch 1, got %q", got)
	}

	key := PhotoKey(42)
	mr.Set(key, "cached")
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mr.Exists(key) {
		t.Fatalf("expected targeted photo key to be deleted")
	}
}

func TestKeySchemeAndFilterHash(t *testing.T) {
	empty := ""
	withAbsentArtist := struct {
		Artist *string `json:"artist,omitempty"`
	}{}
	withEmptyArtist := struct {
		Artist *string `json:"artist,omitempty"`
	}{Artist: &empty}

	absentHash, err := FilterHash(withAbsentArtist)
	if err != nil {
		t.Fatalf("FilterHash absent failed: %v", err)
	}
	emptyHash, err := FilterHash(withEmptyArtist)
	if err != nil {
		t.Fatalf("FilterHash empty failed: %v", err)
	}
	if absentHash == emptyHash {
		t.Fatalf("expected absent and empty filter values to hash differently")
	}

	catalogKey, err := CatalogKey(7, withEmptyArtist, 24, 48)
	if err != nil {
		t.Fatalf("CatalogKey failed: %v", err)
	}
	for _, key := range []string{
		EpochKey,
		catalogKey,
		mustFacetKey(t, 7, "artists", withEmptyArtist),
		PhotoKey(99),
		DedupeHashKey([]byte{0xde, 0xad, 0xbe, 0xef}),
		SeenKey("sight.photo"),
		ThumbKey(99, 320, 240),
	} {
		if !ValidKey(key) {
			t.Fatalf("expected okf: prefix for %q", key)
		}
	}
	if !strings.HasPrefix(DedupeHashKey([]byte{0xde, 0xad}), DedupePrefix+"dead") {
		t.Fatalf("expected content hash key to use hex, got %q", DedupeHashKey([]byte{0xde, 0xad}))
	}
	if !strings.Contains(catalogKey, "e7:") {
		t.Fatalf("expected catalog key to embed epoch, got %q", catalogKey)
	}
}

func mustFacetKey(t *testing.T, epoch int64, name string, filters any) string {
	t.Helper()
	key, err := FacetKey(epoch, name, filters)
	if err != nil {
		t.Fatalf("FacetKey failed: %v", err)
	}
	return key
}

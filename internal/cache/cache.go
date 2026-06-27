package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	mathrand "math/rand/v2"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"ok-folio/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	Prefix        = "okf:"
	EpochKey      = Prefix + "epoch"
	CatalogPrefix = Prefix + "cat:"
	FacetPrefix   = Prefix + "facet:"
	PhotoPrefix   = Prefix + "photo:"
	DedupePrefix  = Prefix + "dh:"
	SeenPrefix    = Prefix + "seen:"
	ThumbPrefix   = Prefix + "thumb:"

	staleWindow = 5 * time.Minute
	lockTTL     = 5 * time.Second
	waitTimeout = 2 * time.Second
	waitStep    = 50 * time.Millisecond
)

var ErrMiss = errors.New("cache miss")

type Client struct {
	r           redis.Cmdable
	logger      zerolog.Logger
	passthrough atomic.Bool
}

type envelope struct {
	ExpiresAt int64           `json:"expires_at"`
	Payload   json.RawMessage `json:"payload"`
}

type ComputeFunc[T any] func(context.Context) (T, error)

func New(ctx context.Context, cfg config.CacheConfig, logger zerolog.Logger) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		Protocol:     2,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	})

	c := &Client{r: rdb, logger: logger}
	if err := rdb.Ping(ctx).Err(); err != nil {
		c.passthrough.Store(true)
		logger.Warn().Err(err).Str("addr", cfg.Addr()).Msg("Valkey unavailable; cache passthrough enabled")
		return c
	}

	logger.Info().Str("addr", cfg.Addr()).Msg("Valkey cache connected")
	return c
}

func NewForRedis(r redis.Cmdable, logger zerolog.Logger) *Client {
	return &Client{r: r, logger: logger}
}

func (c *Client) Passthrough() bool {
	return c == nil || c.passthrough.Load() || c.r == nil
}

// GetOrCompute implements cache-aside reads with a short per-key SET NX PX lock.
// Cached values keep a stale envelope past their fresh TTL; lock losers can
// serve stale data while the winner refreshes, or wait briefly for the new value.
func GetOrCompute[T any](ctx context.Context, c *Client, key string, ttl time.Duration, fn ComputeFunc[T]) (T, error) {
	var zero T
	if c == nil || c.Passthrough() || ttl <= 0 {
		return fn(ctx)
	}

	now := time.Now()
	if value, fresh, err := getEnvelope[T](ctx, c, key, now); err == nil && fresh {
		return value, nil
	} else if err != nil && !errors.Is(err, ErrMiss) {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Str("key", key).Msg("Cache read failed; using passthrough")
		return fn(ctx)
	}

	token := randomToken()
	locked, err := c.r.SetNX(ctx, lockKey(key), token, lockTTL).Result()
	if err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Str("key", key).Msg("Cache lock failed; using passthrough")
		return fn(ctx)
	}

	if locked {
		defer releaseLock(ctx, c, key, token)
		value, err := fn(ctx)
		if err != nil {
			return zero, err
		}
		if err := setEnvelope(ctx, c, key, value, ttl); err != nil {
			c.passthrough.Store(true)
			c.logger.Warn().Err(err).Str("key", key).Msg("Cache write failed; using passthrough")
		}
		return value, nil
	}

	if value, _, err := getEnvelope[T](ctx, c, key, now); err == nil {
		return value, nil
	}

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(waitStep):
		}
		if value, fresh, err := getEnvelope[T](ctx, c, key, time.Now()); err == nil && fresh {
			return value, nil
		}
	}

	return fn(ctx)
}

func (c *Client) BumpEpoch(ctx context.Context) error {
	if c == nil || c.Passthrough() {
		return nil
	}
	if err := c.r.Incr(ctx, EpochKey).Err(); err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Msg("Cache epoch bump failed; using passthrough")
	}
	return nil
}

func (c *Client) Seen(ctx context.Context, providerID string, dedupeKey string) (bool, error) {
	if c == nil || c.Passthrough() || providerID == "" || dedupeKey == "" {
		return false, nil
	}
	seen, err := c.r.SIsMember(ctx, SeenKey(providerID), dedupeKey).Result()
	if err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Str("provider", providerID).Msg("Seen-set read failed; using passthrough")
		return false, nil
	}
	return seen, nil
}

func (c *Client) MarkSeen(ctx context.Context, providerID string, dedupeKey string) error {
	if c == nil || c.Passthrough() || providerID == "" || dedupeKey == "" {
		return nil
	}
	if err := c.r.SAdd(ctx, SeenKey(providerID), dedupeKey).Err(); err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Str("provider", providerID).Msg("Seen-set write failed; using passthrough")
	}
	return nil
}

func (c *Client) MarkDedupeHash(ctx context.Context, contentHash []byte, dedupeKey string) error {
	if c == nil || c.Passthrough() || len(contentHash) == 0 {
		return nil
	}
	if err := c.r.Set(ctx, DedupeHashKey(contentHash), dedupeKey, 0).Err(); err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Msg("Content-hash cache write failed; using passthrough")
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if c == nil || c.Passthrough() || len(keys) == 0 {
		return nil
	}
	if err := c.r.Del(ctx, keys...).Err(); err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Strs("keys", keys).Msg("Cache delete failed; using passthrough")
	}
	return nil
}

func (c *Client) Epoch(ctx context.Context) int64 {
	if c == nil || c.Passthrough() {
		return 0
	}
	value, err := c.r.Get(ctx, EpochKey).Int64()
	if errors.Is(err, redis.Nil) {
		return 0
	}
	if err != nil {
		c.passthrough.Store(true)
		c.logger.Warn().Err(err).Msg("Cache epoch read failed; using passthrough")
		return 0
	}
	return value
}

func getEnvelope[T any](ctx context.Context, c *Client, key string, now time.Time) (T, bool, error) {
	var zero T
	raw, err := c.r.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, false, ErrMiss
	}
	if err != nil {
		return zero, false, err
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return zero, false, err
	}
	var value T
	if err := json.Unmarshal(env.Payload, &value); err != nil {
		return zero, false, err
	}
	return value, now.UnixNano() < env.ExpiresAt, nil
}

func setEnvelope[T any](ctx context.Context, c *Client, key string, value T, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	freshTTL := jitter(ttl)
	env := envelope{
		ExpiresAt: time.Now().Add(freshTTL).UnixNano(),
		Payload:   payload,
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return c.r.Set(ctx, key, raw, freshTTL+staleWindow).Err()
}

func releaseLock(ctx context.Context, c *Client, key string, token string) {
	script := redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)
	_ = script.Run(ctx, c.r, []string{lockKey(key)}, token).Err()
}

func jitter(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	factor := 0.9 + mathrand.Float64()*0.2
	return time.Duration(math.Round(float64(ttl) * factor))
}

func lockKey(key string) string {
	return key + ":lock"
}

func randomToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}

func CatalogKey(epoch int64, filters any, limit int, offset int) (string, error) {
	hash, err := FilterHash(filters)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%se%d:%s:%d:%d", CatalogPrefix, epoch, hash, limit, offset), nil
}

func FacetKey(epoch int64, name string, filters any) (string, error) {
	hash, err := FilterHash(filters)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%se%d:%s:%s", FacetPrefix, epoch, name, hash), nil
}

func PhotoKey(id uint64) string {
	return fmt.Sprintf("%s%d", PhotoPrefix, id)
}

func DedupeHashKey(contentHash []byte) string {
	return DedupePrefix + hex.EncodeToString(contentHash)
}

func SeenKey(provider string) string {
	return SeenPrefix + provider
}

func ThumbKey(id uint64, width int, height int) string {
	return fmt.Sprintf("%s%d:%dx%d", ThumbPrefix, id, width, height)
}

// FilterHash canonicalizes a filter struct as JSON and hashes the bytes. Pointer
// fields are intentional: nil, empty, and non-empty values produce distinct keys.
func FilterHash(filters any) (string, error) {
	payload, err := json.Marshal(filters)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func ValidKey(key string) bool {
	return strings.HasPrefix(key, Prefix)
}

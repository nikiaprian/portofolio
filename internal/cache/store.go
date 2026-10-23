package cache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultStoreNamespace = "tirtayasa"
	defaultStoreTTL       = 3 * time.Minute
	imageCacheTTL         = 7 * 24 * time.Hour
)

var (
	storeMu     sync.RWMutex
	globalStore *Store
)

// StoreConfig wires the shared Redis data cache used by repositories and media.
type StoreConfig struct {
	Client    *redis.Client
	Namespace string
	DefaultTTL time.Duration
}

// Store provides JSON and binary cache helpers namespaced under Redis.
type Store struct {
	client *redis.Client
	ns     string
	ttl    time.Duration
}

// InitStore sets the process-wide Redis data cache. Pass nil client to disable.
func InitStore(cfg StoreConfig) {
	storeMu.Lock()
	defer storeMu.Unlock()
	if cfg.Client == nil {
		globalStore = nil
		return
	}
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = defaultStoreTTL
	}
	ns := strings.TrimSpace(cfg.Namespace)
	if ns == "" {
		ns = defaultStoreNamespace
	}
	globalStore = &Store{client: cfg.Client, ns: ns, ttl: ttl}
}

// GlobalStore returns the configured Redis data cache, or nil when disabled.
func GlobalStore() *Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return globalStore
}

func (s *Store) redisKey(parts ...string) string {
	return s.ns + ":" + strings.Join(parts, ":")
}

func (s *Store) effectiveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	if s.ttl > 0 {
		return s.ttl
	}
	return defaultStoreTTL
}

// GetJSON loads a JSON value. ok is false on cache miss or when Redis is unavailable.
func (s *Store) GetJSON(ctx context.Context, key string, dest any) (ok bool, err error) {
	if s == nil || s.client == nil || key == "" || dest == nil {
		return false, nil
	}
	raw, err := s.client.Get(ctx, s.redisKey(key)).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

// SetJSON stores a JSON value with TTL.
func (s *Store) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	if s == nil || s.client == nil || key == "" || v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.redisKey(key), raw, s.effectiveTTL(ttl)).Err()
}

// Delete removes one or more logical cache keys.
func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if s == nil || s.client == nil || len(keys) == 0 {
		return nil
	}
	redisKeys := make([]string, len(keys))
	for i, key := range keys {
		redisKeys[i] = s.redisKey(key)
	}
	return s.client.Del(ctx, redisKeys...).Err()
}

// GetImage loads resized image bytes and content type from Redis.
func (s *Store) GetImage(ctx context.Context, key string) (data []byte, contentType string, ok bool, err error) {
	if s == nil || s.client == nil || key == "" {
		return nil, "", false, nil
	}
	ctKey := s.redisKey("data:img_resize", key, "ct")
	bodyKey := s.redisKey("data:img_resize", key, "body")
	ct, err := s.client.Get(ctx, ctKey).Result()
	if err == redis.Nil {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	data, err = s.client.Get(ctx, bodyKey).Bytes()
	if err == redis.Nil {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	if ct == "" || len(data) == 0 {
		return nil, "", false, nil
	}
	return data, ct, true, nil
}

// SetImage stores resized image bytes in Redis.
func (s *Store) SetImage(ctx context.Context, key string, data []byte, contentType string, ttl time.Duration) error {
	if s == nil || s.client == nil || key == "" || len(data) == 0 || strings.TrimSpace(contentType) == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = imageCacheTTL
	}
	bodyKey := s.redisKey("data:img_resize", key, "body")
	ctKey := s.redisKey("data:img_resize", key, "ct")
	pipe := s.client.Pipeline()
	pipe.Set(ctx, bodyKey, data, ttl)
	pipe.Set(ctx, ctKey, contentType, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

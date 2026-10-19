package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Bebii-Digital/bebii-blog/internal/cache"
)

const defaultResizeCacheDir = "data/cache/img-resize"

type resizeCacheMeta struct {
	ContentType string `json:"content_type"`
}

// ResizeCache stores resized image bytes (Redis primary, disk fallback).
type ResizeCache struct {
	dir string
	mu  sync.RWMutex
}

var (
	globalResizeCache   *ResizeCache
	globalResizeCacheMu sync.Once
)

// GlobalResizeCache returns the process-wide cache for /img delivery.
func GlobalResizeCache() *ResizeCache {
	globalResizeCacheMu.Do(func() {
		globalResizeCache = NewResizeCache(defaultResizeCacheDir)
	})
	return globalResizeCache
}

// NewResizeCache creates a cache rooted at dir (disk fallback when Redis is unavailable).
func NewResizeCache(dir string) *ResizeCache {
	return &ResizeCache{dir: dir}
}

// ResizeCacheKey builds a stable cache key from source identity and resize params.
func ResizeCacheKey(rel string, modTime int64, modSize int64, maxW, maxH, qual int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%d|%d|%d", rel, modTime, modSize, maxW, maxH, qual)))
	return hex.EncodeToString(sum[:])
}

// ETag returns a weak ETag header value for a cache key.
func ETag(cacheKey string) string {
	if len(cacheKey) < 16 {
		return `W/"` + cacheKey + `"`
	}
	return `W/"` + cacheKey[:16] + `"`
}

// Get returns cached bytes and content type, or ok=false on miss.
func (c *ResizeCache) Get(cacheKey string) (data []byte, contentType string, ok bool) {
	if cacheKey == "" {
		return nil, "", false
	}
	if store := cache.GlobalStore(); store != nil {
		data, contentType, ok, err := store.GetImage(context.Background(), cacheKey)
		if err == nil && ok {
			return data, contentType, true
		}
	}
	return c.getDisk(cacheKey)
}

// Set stores resized bytes; writes to Redis when available, always to disk as fallback.
func (c *ResizeCache) Set(cacheKey string, data []byte, contentType string) error {
	if cacheKey == "" || len(data) == 0 {
		return fmt.Errorf("invalid cache set")
	}
	if store := cache.GlobalStore(); store != nil {
		_ = store.SetImage(context.Background(), cacheKey, data, contentType, 0)
	}
	return c.setDisk(cacheKey, data, contentType)
}

func (c *ResizeCache) getDisk(cacheKey string) ([]byte, string, bool) {
	if c == nil || c.dir == "" {
		return nil, "", false
	}
	dataPath, metaPath := c.paths(cacheKey)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, "", false
	}
	var meta resizeCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, "", false
	}
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, "", false
	}
	return data, meta.ContentType, true
}

func (c *ResizeCache) setDisk(cacheKey string, data []byte, contentType string) error {
	if c == nil || c.dir == "" {
		return fmt.Errorf("invalid cache set")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	dataPath, metaPath := c.paths(cacheKey)
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o755); err != nil {
		return err
	}
	meta := resizeCacheMeta{ContentType: contentType}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	tmpMeta := metaPath + ".tmp"
	if err := os.WriteFile(tmpMeta, metaBytes, 0o644); err != nil {
		return err
	}
	tmpData := dataPath + ".tmp"
	if err := os.WriteFile(tmpData, data, 0o644); err != nil {
		_ = os.Remove(tmpMeta)
		return err
	}
	if err := os.Rename(tmpMeta, metaPath); err != nil {
		_ = os.Remove(tmpData)
		_ = os.Remove(tmpMeta)
		return err
	}
	if err := os.Rename(tmpData, dataPath); err != nil {
		_ = os.Remove(metaPath)
		return err
	}
	return nil
}

func (c *ResizeCache) paths(cacheKey string) (dataPath, metaPath string) {
	shard := cacheKey
	if len(shard) > 2 {
		shard = cacheKey[:2]
	}
	base := filepath.Join(c.dir, shard, cacheKey)
	return base + ".dat", base + ".json"
}

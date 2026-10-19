package cache

import (
	"sync"
	"time"
)

// TTLConfig holds per-feature cache durations. Zero values use built-in defaults.
type TTLConfig struct {
	SiteSetting   time.Duration
	NavCategories time.Duration
	PostViewRank  time.Duration
	ImageResize   time.Duration
}

const (
	defaultSiteSettingTTL   = 5 * time.Minute    // 5 minutes
	defaultNavCategoriesTTL = 60 * time.Minute   // 1 hour
	defaultPostViewRankTTL  = 60 * time.Second   // 1 minute
	defaultImageResizeTTL   = 7 * 24 * time.Hour // 7 days
)

var (
	ttlMu    sync.RWMutex
	ttlCfg   TTLConfig
	ttlReady bool
)

// InitTTLs configures per-feature cache durations. Safe to call once at startup.
func InitTTLs(cfg TTLConfig) {
	ttlMu.Lock()
	defer ttlMu.Unlock()
	ttlCfg = cfg
	ttlReady = true
}

func effectiveTTL(configured, fallback time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return fallback
}

func siteSettingTTL() time.Duration {
	ttlMu.RLock()
	defer ttlMu.RUnlock()
	if !ttlReady {
		return defaultSiteSettingTTL
	}
	return effectiveTTL(ttlCfg.SiteSetting, defaultSiteSettingTTL)
}

func navCategoriesTTL() time.Duration {
	ttlMu.RLock()
	defer ttlMu.RUnlock()
	if !ttlReady {
		return defaultNavCategoriesTTL
	}
	return effectiveTTL(ttlCfg.NavCategories, defaultNavCategoriesTTL)
}

func postViewRankTTL() time.Duration {
	ttlMu.RLock()
	defer ttlMu.RUnlock()
	if !ttlReady {
		return defaultPostViewRankTTL
	}
	return effectiveTTL(ttlCfg.PostViewRank, defaultPostViewRankTTL)
}

func imageResizeTTL() time.Duration {
	ttlMu.RLock()
	defer ttlMu.RUnlock()
	if !ttlReady {
		return defaultImageResizeTTL
	}
	return effectiveTTL(ttlCfg.ImageResize, defaultImageResizeTTL)
}

// SiteSettingTTL returns the cache duration for site_settings (L1 + Redis).
func SiteSettingTTL() time.Duration { return siteSettingTTL() }

// NavCategoriesTTL returns the cache duration for header nav categories.
func NavCategoriesTTL() time.Duration { return navCategoriesTTL() }

// PostViewRankTTL returns the cache duration for post view ranking queries.
func PostViewRankTTL() time.Duration { return postViewRankTTL() }

// ImageResizeTTL returns the cache duration for resized images in Redis.
func ImageResizeTTL() time.Duration { return imageResizeTTL() }

package repositories

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/cache"
	"github.com/nikiaprian/tirtayasa/internal/entity"
)

const (
	siteSettingCacheTTL = 5 * time.Minute
	siteSettingRedisKey = "data:site_setting"
)

var (
	siteSettingCacheMu sync.RWMutex
	siteSettingCached  *cachedSiteSettingRow
	siteSettingFlight  singleflight.Group
)

type cachedSiteSettingRow struct {
	row       entity.SiteSetting
	expiresAt time.Time
}

// InvalidateSiteSettingCache drops site_settings cache (in-process and Redis).
func InvalidateSiteSettingCache() {
	siteSettingCacheMu.Lock()
	siteSettingCached = nil
	siteSettingCacheMu.Unlock()
	if store := cache.GlobalStore(); store != nil {
		_ = store.Delete(context.Background(), siteSettingRedisKey)
	}
}

func getCachedSiteSettingL1() (entity.SiteSetting, bool) {
	siteSettingCacheMu.RLock()
	defer siteSettingCacheMu.RUnlock()
	if siteSettingCached == nil || time.Now().After(siteSettingCached.expiresAt) {
		return entity.SiteSetting{}, false
	}
	return siteSettingCached.row, true
}

func setCachedSiteSettingL1(row entity.SiteSetting) {
	siteSettingCacheMu.Lock()
	defer siteSettingCacheMu.Unlock()
	siteSettingCached = &cachedSiteSettingRow{
		row:       row,
		expiresAt: time.Now().Add(siteSettingCacheTTL),
	}
}

func getCachedSiteSettingFromRedis() (entity.SiteSetting, bool) {
	store := cache.GlobalStore()
	if store == nil {
		return entity.SiteSetting{}, false
	}
	var row entity.SiteSetting
	ok, err := store.GetJSON(context.Background(), siteSettingRedisKey, &row)
	if err != nil || !ok {
		return entity.SiteSetting{}, false
	}
	return row, true
}

func setCachedSiteSetting(row entity.SiteSetting) {
	setCachedSiteSettingL1(row)
	if store := cache.GlobalStore(); store != nil {
		_ = store.SetJSON(context.Background(), siteSettingRedisKey, row, siteSettingCacheTTL)
	}
}

func normalizeSiteSettingRow(s entity.SiteSetting) entity.SiteSetting {
	entity.UnpackSiteSettingJSON(&s)
	s.HomepageLayout = entity.NormalizeHomeLayout(strings.TrimSpace(s.HomepageLayout))
	s.ColorTheme = entity.NormalizeColorTheme(strings.TrimSpace(s.ColorTheme))
	s.NotFoundRedirectMode = entity.NormalizeNotFoundRedirectMode(strings.TrimSpace(s.NotFoundRedirectMode))
	s.ArticleSlugMode = entity.NormalizeArticleSlugMode(strings.TrimSpace(s.ArticleSlugMode))
	entity.NormalizeSidebarListLimits(&s)
	return s
}

func fetchSiteSettingFromDB(db *gorm.DB) (entity.SiteSetting, error) {
	if db == nil {
		return entity.DefaultSiteSetting(), nil
	}
	var s entity.SiteSetting
	err := db.Order("id asc").Limit(1).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return normalizeSiteSettingRow(entity.DefaultSiteSetting()), nil
		}
		return entity.SiteSetting{}, err
	}
	return normalizeSiteSettingRow(s), nil
}

// GetSiteSettingCached returns the singleton site_settings row with Redis + in-process caching.
func GetSiteSettingCached(db *gorm.DB) (entity.SiteSetting, error) {
	if row, ok := getCachedSiteSettingL1(); ok {
		return row, nil
	}
	if row, ok := getCachedSiteSettingFromRedis(); ok {
		setCachedSiteSettingL1(row)
		return row, nil
	}
	v, err, _ := siteSettingFlight.Do("site_setting", func() (any, error) {
		if row, ok := getCachedSiteSettingL1(); ok {
			return row, nil
		}
		if row, ok := getCachedSiteSettingFromRedis(); ok {
			setCachedSiteSettingL1(row)
			return row, nil
		}
		row, err := fetchSiteSettingFromDB(db)
		if err != nil {
			return entity.SiteSetting{}, err
		}
		setCachedSiteSetting(row)
		return row, nil
	})
	if err != nil {
		return entity.SiteSetting{}, err
	}
	return v.(entity.SiteSetting), nil
}

// SiteSettingOrDefault returns cached site settings or defaults when the read fails.
func SiteSettingOrDefault(db *gorm.DB) entity.SiteSetting {
	s, err := GetSiteSettingCached(db)
	if err != nil {
		return entity.DefaultSiteSetting()
	}
	return s
}

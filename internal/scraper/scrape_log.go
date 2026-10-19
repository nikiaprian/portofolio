package scraper

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"gorm.io/gorm"
)

const scrapeLogMessageMax = 8000

var (
	scrapeLogDBMu sync.RWMutex
	scrapeLogDB   *gorm.DB
)

// SetScraperLogDB wires the DB used for scraper_logs (call once at process startup).
func SetScraperLogDB(db *gorm.DB) {
	scrapeLogDBMu.Lock()
	defer scrapeLogDBMu.Unlock()
	scrapeLogDB = db
}

func dbForScrapeLog() *gorm.DB {
	scrapeLogDBMu.RLock()
	defer scrapeLogDBMu.RUnlock()
	return scrapeLogDB
}

func trimScrapeField(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// LogHTTPNotOK records a successful transport but non-2xx HTTP status (URL + status code).
func LogHTTPNotOK(pageURL string, statusCode int) {
	u := strings.TrimSpace(pageURL)
	if u == "" {
		return
	}
	if statusCode >= 200 && statusCode <= 299 {
		return
	}
	ScrapeLogf("warn", "", u, "[scraper] GET url=%s http_status=%d", u, statusCode)
}

func normalizeScrapeProviderCol(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) <= 16 {
		return s
	}
	return s[:16]
}

// ScrapeLogf writes to stderr and persists a row when SetScraperLogDB was called.
// level is typically info, warn, or error.
func ScrapeLogf(level, categorySlug, feedURL, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	d := dbForScrapeLog()
	if d == nil {
		return
	}
	lv := strings.ToLower(strings.TrimSpace(level))
	if lv == "" {
		lv = "info"
	}
	row := entity.ScraperLog{
		Level:        trimScrapeField(lv, 16),
		Message:      trimScrapeField(msg, scrapeLogMessageMax),
		CategorySlug: trimScrapeField(categorySlug, 255),
		FeedURL:      trimScrapeField(feedURL, 2000),
	}
	if err := d.Select("Level", "Message", "CategorySlug", "FeedURL", "ProviderUsed").Create(&row).Error; err != nil {
		log.Printf("[scraper-log] persist failed: %v", err)
	}
}

// ScrapeLogWithProviders persists provider_used for AI API audit lines (e.g. which backend served the request).
func ScrapeLogWithProviders(level, categorySlug, feedURL, providerUsed, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	d := dbForScrapeLog()
	if d == nil {
		return
	}
	lv := strings.ToLower(strings.TrimSpace(level))
	if lv == "" {
		lv = "info"
	}
	row := entity.ScraperLog{
		Level:        trimScrapeField(lv, 16),
		Message:      trimScrapeField(msg, scrapeLogMessageMax),
		CategorySlug: trimScrapeField(categorySlug, 255),
		FeedURL:      trimScrapeField(feedURL, 2000),
		ProviderUsed: trimScrapeField(normalizeScrapeProviderCol(providerUsed), 16),
	}
	if err := d.Select("Level", "Message", "CategorySlug", "FeedURL", "ProviderUsed").Create(&row).Error; err != nil {
		log.Printf("[scraper-log] persist failed: %v", err)
	}
}

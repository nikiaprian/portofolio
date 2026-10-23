package adminlogprune

import (
	"log"
	"sync"
	"time"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"gorm.io/gorm"
)

var (
	mu      sync.RWMutex
	pruneDB *gorm.DB
	retainH int // hours; <= 0 disables automatic deletion
)

// Init configures the database handle and rolling retention in hours.
// Pass retentionHours <= 0 to disable automatic pruning.
func Init(db *gorm.DB, retentionHours int) {
	mu.Lock()
	defer mu.Unlock()
	pruneDB = db
	retainH = retentionHours
}

// RetentionHours returns the configured window in hours (0 when disabled).
func RetentionHours() int {
	mu.RLock()
	defer mu.RUnlock()
	return retainH
}

// MaybePrune deletes rows with created_at older than the retention window from
// scraper_logs, login_logs, bulk_post_logs, and manual_post_ai_logs.
func MaybePrune() {
	mu.RLock()
	d := pruneDB
	h := retainH
	mu.RUnlock()
	if d == nil || h <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-time.Duration(h) * time.Hour)
	tasks := []struct {
		name string
		run  func() error
	}{
		{"scraper_logs", func() error { return d.Where("created_at < ?", cutoff).Delete(&entity.ScraperLog{}).Error }},
		{"login_logs", func() error { return d.Where("created_at < ?", cutoff).Delete(&entity.LoginLog{}).Error }},
		{"bulk_post_logs", func() error { return d.Where("created_at < ?", cutoff).Delete(&entity.BulkPostLog{}).Error }},
		{"manual_post_ai_logs", func() error { return d.Where("created_at < ?", cutoff).Delete(&entity.ManualPostAILog{}).Error }},
	}
	for _, t := range tasks {
		if err := t.run(); err != nil {
			log.Printf("[admin-log-prune] %s: %v", t.name, err)
		}
	}
}

// StartBackground runs MaybePrune on startup and then every tick while retention is enabled.
func StartBackground() {
	mu.RLock()
	h := retainH
	mu.RUnlock()
	if h <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(15 * time.Minute)
		defer t.Stop()
		MaybePrune()
		for range t.C {
			MaybePrune()
		}
	}()
}

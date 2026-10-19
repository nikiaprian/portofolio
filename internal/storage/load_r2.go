package storage

import (
	"context"
	"log"
	"strings"

	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// PostAssetStore returns R2 when configured (DB + env via LoadR2ObjectStore), otherwise a local
// store matching post image paths (for delete-by-URL when R2 is off).
func PostAssetStore(ctx context.Context, db *gorm.DB, cfg appconfig.Config) ObjectStore {
	if s := LoadR2ObjectStore(ctx, db, cfg); s != nil {
		return s
	}
	return NewLocalStore("public/static/uploads/posts", "/static/uploads/posts")
}

// LoadR2ObjectStore returns an R2-backed ObjectStore when either:
// 1) site_cloud_storage has all required fields (admin UI), or
// 2) environment variables in cfg are complete (legacy / server-only secrets).
// Database wins when both are usable so /admin/site-settings R2 fields take effect.
//
// Call this when serving a request (or each scraper run), not only at process startup, so admin
// changes to R2 settings take effect without restarting the server.
func LoadR2ObjectStore(ctx context.Context, db *gorm.DB, cfg appconfig.Config) ObjectStore {
	var row entity.SiteCloudStorage
	if err := db.First(&row).Error; err == nil {
		if c := r2ConfigFromSiteCloudRow(row); c != nil {
			s, errR2 := NewR2Store(ctx, *c)
			if errR2 == nil {
				log.Printf("storage: R2 enabled (site_cloud_storage)")
				return s
			}
			log.Printf("storage: R2 database config failed: %v", errR2)
		}
	}

	if c := r2ConfigFromEnv(cfg); c != nil {
		s, err := NewR2Store(ctx, *c)
		if err == nil {
			log.Printf("storage: R2 enabled (environment variables)")
			return s
		}
		log.Printf("storage: R2 environment config failed: %v", err)
	}

	return nil
}

func r2ConfigFromSiteCloudRow(s entity.SiteCloudStorage) *R2Config {
	account := strings.TrimSpace(s.R2AccountID)
	keyID := strings.TrimSpace(s.R2AccessKeyID)
	secret := strings.TrimSpace(s.R2SecretAccessKey)
	bucket := strings.TrimSpace(s.R2Bucket)
	if bucket == "" {
		bucket = "media"
	}
	pub := strings.TrimSpace(s.R2PublicBaseURL)
	if account == "" || keyID == "" || secret == "" || pub == "" {
		return nil
	}
	prefix := strings.TrimSpace(s.R2Prefix)
	if prefix == "" {
		prefix = "uploads"
	}
	return &R2Config{
		AccountID:       account,
		AccessKeyID:     keyID,
		SecretAccessKey: secret,
		Bucket:          bucket,
		PublicBaseURL:   pub,
		Prefix:          prefix,
	}
}

func r2ConfigFromEnv(cfg appconfig.Config) *R2Config {
	account := strings.TrimSpace(cfg.R2AccountID)
	keyID := strings.TrimSpace(cfg.R2AccessKeyID)
	secret := strings.TrimSpace(cfg.R2SecretAccessKey)
	bucket := strings.TrimSpace(cfg.R2Bucket)
	pub := strings.TrimSpace(cfg.R2PublicBaseURL)
	if account == "" || keyID == "" || secret == "" || bucket == "" || pub == "" {
		return nil
	}
	prefix := strings.TrimSpace(cfg.R2Prefix)
	if prefix == "" {
		prefix = "uploads"
	}
	return &R2Config{
		AccountID:       account,
		AccessKeyID:     keyID,
		SecretAccessKey: secret,
		Bucket:          bucket,
		PublicBaseURL:   pub,
		Prefix:          prefix,
	}
}

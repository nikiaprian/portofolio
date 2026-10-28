package migrations

import (
	"time"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// scraperCategoryFeedLegacyPause exists only so Migrator can drop pause_after_category_mins
// after the field was moved to scraper_settings.
type scraperCategoryFeedLegacyPause struct {
	PauseAfterCategoryMins int `gorm:"column:pause_after_category_mins"`
}

func (scraperCategoryFeedLegacyPause) TableName() string { return "scraper_category_feeds" }

// siteSettingLegacyRecycleURLs exists only so Migrator can drop post_date_url_recycle_urls
// after the URL list moved to an in-memory queue (no longer persisted).
type siteSettingLegacyRecycleURLs struct {
	PostDateUrlRecycleURLs string `gorm:"column:post_date_url_recycle_urls"`
}

func (siteSettingLegacyRecycleURLs) TableName() string { return "site_settings" }

// Shadow models below exist only for Migrator index management (not AutoMigrate targets).

type postsStatusUpdatedIndex struct {
	Status    string    `gorm:"column:status;index:idx_posts_status_updated,priority:1"`
	UpdatedAt time.Time `gorm:"column:updated_at;index:idx_posts_status_updated,priority:2"`
}

func (postsStatusUpdatedIndex) TableName() string { return "posts" }

type postsStatusPublishIndex struct {
	Status    string     `gorm:"column:status;index:idx_posts_status_publish,priority:1"`
	PublishAt *time.Time `gorm:"column:publish_at;index:idx_posts_status_publish,priority:2"`
}

func (postsStatusPublishIndex) TableName() string { return "posts" }

type postsCategoryStatusIndex struct {
	CategoryID uint   `gorm:"column:category_id;index:idx_posts_category_status,priority:1"`
	Status     string `gorm:"column:status;index:idx_posts_category_status,priority:2"`
}

func (postsCategoryStatusIndex) TableName() string { return "posts" }

type postViewsViewDateIndex struct {
	ViewDate string `gorm:"column:view_date;index:idx_post_views_view_date,priority:1"`
	PostID   uint   `gorm:"column:post_id;index:idx_post_views_view_date,priority:2"`
}

func (postViewsViewDateIndex) TableName() string { return "post_views" }

type postViewsCreatedIndex struct {
	CreatedAt time.Time `gorm:"column:created_at;index:idx_post_views_created,priority:1"`
	PostID    uint      `gorm:"column:post_id;index:idx_post_views_created,priority:2"`
}

func (postViewsCreatedIndex) TableName() string { return "post_views" }

type postViewsPostCreatedLegacy struct {
	PostID    uint      `gorm:"column:post_id;index:idx_post_views_post_created,priority:1"`
	CreatedAt time.Time `gorm:"column:created_at;index:idx_post_views_post_created,priority:2"`
}

func (postViewsPostCreatedLegacy) TableName() string { return "post_views" }

type postTagsTagIDIndex struct {
	TagID uint `gorm:"column:tag_id;index:idx_post_tags_tag_id"`
}

func (postTagsTagIDIndex) TableName() string { return "post_tags" }

type postTagsPostIDLegacy struct {
	PostID uint `gorm:"column:post_id;index:idx_post_tags_post_id"`
}

func (postTagsPostIDLegacy) TableName() string { return "post_tags" }

// Run applies database schema migrations using GORM AutoMigrate.
func Run(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&entity.User{},
		&entity.Category{},
		&entity.Tag{},
		&entity.Post{},
		&entity.PostView{},
		&entity.Page{},
		&entity.SiteSetting{},
		&entity.SiteCloudStorage{},
		&entity.SiteWebImageProvider{},
		&entity.SiteConnection{},
		&entity.ScraperSetting{},
		&entity.ScraperCategoryFeed{},
		&entity.ScraperLog{},
		&entity.LoginLog{},
		&entity.BulkPostLog{},
		&entity.ManualPostAILog{},
		&entity.AISetting{},
	); err != nil {
		return err
	}
	if db.Migrator().HasColumn(&entity.Post{}, "meta_title") {
		if err := db.Migrator().DropColumn(&entity.Post{}, "meta_title"); err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn(&entity.Post{}, "meta_image") {
		if err := db.Migrator().DropColumn(&entity.Post{}, "meta_image"); err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn(&scraperCategoryFeedLegacyPause{}, "PauseAfterCategoryMins") {
		if err := db.Migrator().DropColumn(&scraperCategoryFeedLegacyPause{}, "PauseAfterCategoryMins"); err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn(&siteSettingLegacyRecycleURLs{}, "PostDateUrlRecycleURLs") {
		if err := db.Migrator().DropColumn(&siteSettingLegacyRecycleURLs{}, "PostDateUrlRecycleURLs"); err != nil {
			return err
		}
	}
	if err := backfillSiteSettingImgResizeColumns(db); err != nil {
		return err
	}
	if err := ensureSiteSettingAutoHrefColumn(db); err != nil {
		return err
	}
	if err := ensureSiteSettingImgResizeEnabledColumn(db); err != nil {
		return err
	}
	if err := ensureIndexingColumns(db); err != nil {
		return err
	}
	return ensurePerformanceIndexes(db)
}

func ensureIndexingColumns(db *gorm.DB) error {
	m := db.Migrator()
	specs := []struct {
		model any
		cols  []string
	}{
		{&entity.Post{}, []string{"IndexingStatus", "IndexedAt", "IndexingMessage"}},
		{&entity.Page{}, []string{"IndexingStatus", "IndexedAt", "IndexingMessage"}},
		{&entity.Category{}, []string{"IndexingStatus", "IndexedAt", "IndexingMessage"}},
		{&entity.Tag{}, []string{"IndexingStatus", "IndexedAt", "IndexingMessage"}},
		{&entity.SiteConnection{}, []string{"GoogleIndexingEnabled", "GoogleServiceAccountJSON"}},
	}
	for _, spec := range specs {
		if !m.HasTable(spec.model) {
			continue
		}
		for _, col := range spec.cols {
			if m.HasColumn(spec.model, col) {
				continue
			}
			if err := m.AddColumn(spec.model, col); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensurePerformanceIndexes(db *gorm.DB) error {
	// Drop superseded indexes (wrong column order or redundant with the primary key).
	for _, drop := range []struct {
		model any
		name  string
	}{
		{&postViewsPostCreatedLegacy{}, "idx_post_views_post_created"},
		{&postTagsPostIDLegacy{}, "idx_post_tags_post_id"},
	} {
		if err := dropIndexIfExists(db, drop.model, drop.name); err != nil {
			return err
		}
	}

	for _, idx := range []struct {
		model any
		name  string
	}{
		{&postsStatusUpdatedIndex{}, "idx_posts_status_updated"},
		{&postsStatusPublishIndex{}, "idx_posts_status_publish"},
		{&postsCategoryStatusIndex{}, "idx_posts_category_status"},
		{&postViewsViewDateIndex{}, "idx_post_views_view_date"},
		{&postViewsCreatedIndex{}, "idx_post_views_created"},
		{&postTagsTagIDIndex{}, "idx_post_tags_tag_id"},
	} {
		if err := createIndexIfMissing(db, idx.model, idx.name); err != nil {
			return err
		}
	}
	return nil
}

func dropIndexIfExists(db *gorm.DB, model any, name string) error {
	m := db.Migrator()
	if !m.HasIndex(model, name) {
		return nil
	}
	return m.DropIndex(model, name)
}

func createIndexIfMissing(db *gorm.DB, model any, name string) error {
	m := db.Migrator()
	if m.HasIndex(model, name) {
		return nil
	}
	return m.CreateIndex(model, name)
}

// ensureSiteSettingAutoHrefColumn adds auto_href_enabled when missing (older deployments before the field existed).
func ensureSiteSettingAutoHrefColumn(db *gorm.DB) error {
	if db.Migrator().HasColumn(&entity.SiteSetting{}, "AutoHrefEnabled") {
		return nil
	}
	return db.Migrator().AddColumn(&entity.SiteSetting{}, "AutoHrefEnabled")
}

// ensureSiteSettingImgResizeEnabledColumn adds img_resize_enabled when missing (older deployments before the toggle existed).
func ensureSiteSettingImgResizeEnabledColumn(db *gorm.DB) error {
	if db.Migrator().HasColumn(&entity.SiteSetting{}, "ImgResizeEnabled") {
		return nil
	}
	return db.Migrator().AddColumn(&entity.SiteSetting{}, "ImgResizeEnabled")
}

// backfillSiteSettingImgResizeColumns seeds img_resize_* from legacy image_resize_* once after the column split.
func backfillSiteSettingImgResizeColumns(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&entity.SiteSetting{}, "ImgResizeWidth") {
		return nil
	}
	return db.Model(&entity.SiteSetting{}).
		Where("(img_resize_width IS NULL OR img_resize_width = '')").
		Where("(img_resize_quality IS NULL OR img_resize_quality = '')").
		Where("image_resize_width IS NOT NULL AND image_resize_width != ''").
		Updates(map[string]interface{}{
			"img_resize_width":   gorm.Expr("image_resize_width"),
			"img_resize_quality": gorm.Expr("image_resize_quality"),
		}).Error
}

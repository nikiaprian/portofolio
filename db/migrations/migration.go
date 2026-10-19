package migrations

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
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

// siteSettingLegacyFlat reads pre-JSON-merge flat columns for one-time backfill.
type siteSettingLegacyFlat struct {
	ID                             uint
	TextPrimary                    string `gorm:"column:text_primary"`
	TextSecondary                  string `gorm:"column:text_secondary"`
	BackgroundPrimary              string `gorm:"column:background_primary"`
	BackgroundSecondary            string `gorm:"column:background_secondary"`
	PrimaryButton                  string `gorm:"column:primary_button"`
	SecondaryButton                string `gorm:"column:secondary_button"`
	AccentColor                    string `gorm:"column:accent_color"`
	BorderColor                    string `gorm:"column:border_color"`
	HeaderScript                   string `gorm:"column:header_script"`
	FooterScript                   string `gorm:"column:footer_script"`
	AdsTop                         string `gorm:"column:ads_top"`
	AdsCenter                      string `gorm:"column:ads_center"`
	AdsBottom                      string `gorm:"column:ads_bottom"`
	AdsLazyLoadEnabled             bool   `gorm:"column:ads_lazy_load_enabled"`
	ArticleAdsJSON                 string `gorm:"column:article_ads_json"`
	ImgResizeEnabled               bool   `gorm:"column:img_resize_enabled"`
	ImgResizeWidth                 string `gorm:"column:img_resize_width"`
	ImgResizeHeight                string `gorm:"column:img_resize_height"`
	ImgResizeQuality               string `gorm:"column:img_resize_quality"`
	ImageResizeEnabled             bool   `gorm:"column:image_resize_enabled"`
	ImageResizeWidth               string `gorm:"column:image_resize_width"`
	ImageResizeQuality             string `gorm:"column:image_resize_quality"`
	StoredImageMaxWidth            string `gorm:"column:stored_image_max_width"`
	StoredImageQuality             string `gorm:"column:stored_image_quality"`
	AvatarResizeWidth              string `gorm:"column:avatar_resize_width"`
	AvatarResizeQuality            string `gorm:"column:avatar_resize_quality"`
	SocialFacebookURL              string `gorm:"column:social_facebook_url"`
	SocialXURL                     string `gorm:"column:social_x_url"`
	SocialInstagramURL             string `gorm:"column:social_instagram_url"`
	SocialYouTubeURL               string `gorm:"column:social_youtube_url"`
	SocialTikTokURL                string `gorm:"column:social_tiktok_url"`
	SocialRSSURL                   string `gorm:"column:social_rss_url"`
	FontFamily                     string `gorm:"column:font_family"`
	ContentFontSize                string `gorm:"column:content_font_size"`
	SidebarFontSize                string `gorm:"column:sidebar_font_size"`
	SidebarListLimit               int    `gorm:"column:sidebar_list_limit"`
	SidebarListLimitPopular        int    `gorm:"column:sidebar_list_limit_popular"`
	SidebarListLimitLatest         int    `gorm:"column:sidebar_list_limit_latest"`
	ImageCaptionEnabled            bool   `gorm:"column:image_caption_enabled"`
	ImageCaptionMode               string `gorm:"column:image_caption_mode"`
	ImageCaptionTemplate           string `gorm:"column:image_caption_template"`
	DefaultTitle                   string `gorm:"column:default_title"`
	DefaultDesc                    string `gorm:"column:default_description"`
	DefaultImage                   string `gorm:"column:default_image"`
	DefaultKeywords                string `gorm:"column:default_keywords"`
	SourceDisclaimerMode           string `gorm:"column:source_disclaimer_mode"`
	SourceDisclaimerTemplate       string `gorm:"column:source_disclaimer_template"`
	AlsoReadEnabled                bool   `gorm:"column:also_read_enabled"`
	AlsoReadMaxInserts             int    `gorm:"column:also_read_max_inserts"`
	PostDateRefreshEnabled         bool   `gorm:"column:post_date_refresh_enabled"`
	PostDateRefreshDays            int    `gorm:"column:post_date_refresh_days"`
	PostDateIntervalMins           int    `gorm:"column:post_date_interval_mins"`
	PostDateUrlRecycleEnabled      bool   `gorm:"column:post_date_url_recycle_enabled"`
	PostDateUrlRecycleIntervalMins int    `gorm:"column:post_date_url_recycle_interval_mins"`
	AdsTxt                         string `gorm:"column:ads_txt"`
	RobotsTxt                      string `gorm:"column:robots_txt"`
}

func (siteSettingLegacyFlat) TableName() string { return "site_settings" }

// siteSettingJSONColumns exists so Migrator can add JSON blob columns (GORM skips unexported fields on AutoMigrate).
type siteSettingJSONColumns struct {
	ColorTheme       string `gorm:"column:color_theme;type:text"`
	AdsScript        string `gorm:"column:ads_script;type:text"`
	ImgResizeSelf    string `gorm:"column:img_resize_self;type:text"`
	ImgResizeCDN     string `gorm:"column:img_resize_cdn;type:text"`
	SocialLinks      string `gorm:"column:social_links;type:text"`
	Typography       string `gorm:"column:typography;type:text"`
	SidebarConfig    string `gorm:"column:sidebar_config;type:text"`
	ImageCaption     string `gorm:"column:image_caption;type:text"`
	DefaultMeta      string `gorm:"column:default_meta;type:text"`
	SourceDisclaimer string `gorm:"column:source_disclaimer;type:text"`
	AlsoRead         string `gorm:"column:also_read;type:text"`
	PostDateConfig   string `gorm:"column:post_date_config;type:text"`
	SeoFiles         string `gorm:"column:seo_files;type:text"`
}

func (siteSettingJSONColumns) TableName() string { return "site_settings" }

// siteLayoutLegacyName exists only so Migrator can drop name after layout_name replaced it.
type siteLayoutLegacyName struct {
	Name string `gorm:"column:name;size:255"`
}

func (siteLayoutLegacyName) TableName() string { return "site_layouts" }

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
		&entity.SiteLayout{},
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
		&entity.Portfolio{},
		&entity.PortfolioOrder{},
		&entity.PortfolioAccount{},
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
	if err := dropSiteLayoutLegacyNameColumn(db); err != nil {
		return err
	}
	if err := ensureSiteSettingJSONColumns(db); err != nil {
		return err
	}
	if err := backfillSiteSettingJSONColumns(db); err != nil {
		return err
	}
	if err := repackSiteSettingJSONColumns(db); err != nil {
		return err
	}
	if err := verifySiteSettingJSONMigrated(db); err != nil {
		return err
	}
	if err := dropSiteSettingLegacyFlatColumns(db); err != nil {
		return err
	}
	if err := ensureSiteSettingAutoHrefColumn(db); err != nil {
		return err
	}
	if err := ensureSiteSettingAdminLoginFrontendDisabledColumn(db); err != nil {
		return err
	}
	if err := ensureAISettingSearchModeColumns(db); err != nil {
		return err
	}
	if err := ensureAISettingOpenCodeLocalJSON(db); err != nil {
		return err
	}
	if err := ensureCategoryEnabledColumn(db); err != nil {
		return err
	}
	if err := ensureIndexingColumns(db); err != nil {
		return err
	}
	if err := ensurePostContentMediumText(db); err != nil {
		return err
	}
	return ensurePerformanceIndexes(db)
}

// ensurePostContentMediumText widens posts.content so long HTML articles are not truncated.
func ensurePostContentMediumText(db *gorm.DB) error {
	if !db.Migrator().HasTable(&entity.Post{}) || !db.Migrator().HasColumn(&entity.Post{}, "Content") {
		return nil
	}
	return db.Exec("ALTER TABLE posts MODIFY COLUMN content MEDIUMTEXT NOT NULL").Error
}

func ensureIndexingColumns(db *gorm.DB) error {
	m := db.Migrator()

	_ = m.AddColumn(&entity.Category{}, "IndexingStatus")
	_ = m.AddColumn(&entity.Category{}, "IndexedAt")
	_ = m.AddColumn(&entity.Category{}, "IndexingMessage")

	_ = m.AddColumn(&entity.Tag{}, "IndexingStatus")
	_ = m.AddColumn(&entity.Tag{}, "IndexedAt")
	_ = m.AddColumn(&entity.Tag{}, "IndexingMessage")

	_ = m.AddColumn(&entity.Page{}, "IndexingStatus")
	_ = m.AddColumn(&entity.Page{}, "IndexedAt")
	_ = m.AddColumn(&entity.Page{}, "IndexingMessage")

	tables := []string{"categories", "tags", "pages"}
	for _, tbl := range tables {
		_ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN indexing_status VARCHAR(32) DEFAULT 'not_indexed'", tbl)).Error
		_ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN indexed_at DATETIME NULL", tbl)).Error
		_ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN indexing_message TEXT NULL", tbl)).Error
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

// dropSiteLayoutLegacyNameColumn removes the old name column after copying into layout_name.
func dropSiteLayoutLegacyNameColumn(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&siteLayoutLegacyName{}, "Name") {
		return nil
	}
	if db.Migrator().HasColumn(&entity.SiteLayout{}, "LayoutName") {
		if err := db.Model(&entity.SiteLayout{}).
			Where("(layout_name IS NULL OR layout_name = '')").
			Where("name IS NOT NULL AND name != ''").
			Updates(map[string]interface{}{
				"layout_name": gorm.Expr("name"),
			}).Error; err != nil {
			return err
		}
	}
	return db.Migrator().DropColumn(&siteLayoutLegacyName{}, "Name")
}

// ensureSiteSettingAutoHrefColumn adds auto_href_enabled when missing (older deployments before the field existed).
func ensureSiteSettingAutoHrefColumn(db *gorm.DB) error {
	if db.Migrator().HasColumn(&entity.SiteSetting{}, "AutoHrefEnabled") {
		return nil
	}
	return db.Migrator().AddColumn(&entity.SiteSetting{}, "AutoHrefEnabled")
}

func ensureSiteSettingAdminLoginFrontendDisabledColumn(db *gorm.DB) error {
	if db.Migrator().HasColumn(&entity.SiteSetting{}, "AdminLoginFrontendDisabled") {
		return nil
	}
	return db.Migrator().AddColumn(&entity.SiteSetting{}, "AdminLoginFrontendDisabled")
}

// ensureAISettingSearchModeColumns adds per-provider search mode toggle columns.
func ensureAISettingSearchModeColumns(db *gorm.DB) error {
	for _, col := range []string{
		"GeminiSearchMode",
		"OpenAISearchMode",
		"GroqSearchMode",
		"ClaudeSearchMode",
	} {
		if db.Migrator().HasColumn(&entity.AISetting{}, col) {
			continue
		}
		if err := db.Migrator().AddColumn(&entity.AISetting{}, col); err != nil {
			return err
		}
	}
	return nil
}

// aiSettingOpenCodeLocalLegacy holds old flat OpenCode local columns for one-time backfill.
type aiSettingOpenCodeLocalLegacy struct {
	OpenCodeLocalHost     string `gorm:"column:opencode_local_host"`
	OpenCodeLocalPort     string `gorm:"column:opencode_local_port"`
	OpenCodeLocalUsername string `gorm:"column:opencode_local_username"`
	OpenCodeLocalPassword string `gorm:"column:opencode_local_password"`
	OpenCodeLocalModel    string `gorm:"column:opencode_local_model"`
}

func (aiSettingOpenCodeLocalLegacy) TableName() string { return "ai_setting" }

// ensureAISettingOpenCodeLocalJSON consolidates flat OpenCode local fields into opencode_local JSON.
func ensureAISettingOpenCodeLocalJSON(db *gorm.DB) error {
	m := db.Migrator()
	if !m.HasColumn(&entity.AISetting{}, "OpenCodeLocal") {
		if err := m.AddColumn(&entity.AISetting{}, "OpenCodeLocal"); err != nil {
			return err
		}
	}

	legacy := aiSettingOpenCodeLocalLegacy{}
	hasLegacy := m.HasColumn(&legacy, "OpenCodeLocalHost") ||
		m.HasColumn(&legacy, "OpenCodeLocalPort") ||
		m.HasColumn(&legacy, "OpenCodeLocalUsername") ||
		m.HasColumn(&legacy, "OpenCodeLocalPassword") ||
		m.HasColumn(&legacy, "OpenCodeLocalModel")
	if hasLegacy {
		type legacyRow struct {
			ID                    uint
			OpenCodeLocalHost     string `gorm:"column:opencode_local_host"`
			OpenCodeLocalPort     string `gorm:"column:opencode_local_port"`
			OpenCodeLocalUsername string `gorm:"column:opencode_local_username"`
			OpenCodeLocalPassword string `gorm:"column:opencode_local_password"`
			OpenCodeLocalModel    string `gorm:"column:opencode_local_model"`
		}
		var rows []legacyRow
		q := db.Table("ai_setting")
		if err := q.Select("id, opencode_local_host, opencode_local_port, opencode_local_username, opencode_local_password, opencode_local_model").Find(&rows).Error; err != nil {
			// Columns may partially exist; ignore select failure and continue drop.
			rows = nil
		}
		for _, r := range rows {
			cfg := entity.OpenCodeLocalSetting{
				Host:     strings.TrimSpace(r.OpenCodeLocalHost),
				Port:     strings.TrimSpace(r.OpenCodeLocalPort),
				Username: strings.TrimSpace(r.OpenCodeLocalUsername),
				Password: strings.TrimSpace(r.OpenCodeLocalPassword),
				Model:    strings.TrimSpace(r.OpenCodeLocalModel),
			}
			cfg.Normalize()
			if !cfg.Configured() && cfg.Model == "auto" && cfg.Username == "" && cfg.Password == "" {
				continue
			}
			if err := db.Model(&entity.AISetting{}).Where("id = ?", r.ID).Update("opencode_local", cfg).Error; err != nil {
				return err
			}
		}
		for _, col := range []string{
			"OpenCodeLocalHost",
			"OpenCodeLocalPort",
			"OpenCodeLocalUsername",
			"OpenCodeLocalPassword",
			"OpenCodeLocalModel",
		} {
			if !m.HasColumn(&legacy, col) {
				continue
			}
			if err := m.DropColumn(&legacy, col); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureCategoryEnabledColumn adds the enabled column to categories if missing.
func ensureCategoryEnabledColumn(db *gorm.DB) error {
	if db.Migrator().HasColumn(&entity.Category{}, "Enabled") {
		return nil
	}
	if err := db.Migrator().AddColumn(&entity.Category{}, "Enabled"); err != nil {
		return err
	}
	// Set enabled=true for existing rows
	return db.Model(&entity.Category{}).Where("enabled IS NULL OR enabled = 0").Update("enabled", true).Error
}

func legacySiteSettingHasFlatColumns(db *gorm.DB) bool {
	legacy := siteSettingLegacyFlat{}
	for _, col := range siteSettingLegacyFlatColumnNames() {
		if db.Migrator().HasColumn(&legacy, col) {
			return true
		}
	}
	return false
}

func siteSettingLegacyFlatColumnNames() []string {
	return []string{
		"TextPrimary", "TextSecondary", "BackgroundPrimary", "BackgroundSecondary",
		"PrimaryButton", "SecondaryButton", "AccentColor", "BorderColor",
		"HeaderScript", "FooterScript", "AdsTop", "AdsCenter", "AdsBottom",
		"AdsLazyLoadEnabled", "ArticleAdsJSON",
		"ImgResizeEnabled", "ImgResizeWidth", "ImgResizeHeight", "ImgResizeQuality",
		"ImageResizeEnabled", "ImageResizeWidth", "ImageResizeQuality",
		"StoredImageMaxWidth", "StoredImageQuality", "AvatarResizeWidth", "AvatarResizeQuality",
		"SocialFacebookURL", "SocialXURL", "SocialInstagramURL", "SocialYouTubeURL", "SocialTikTokURL", "SocialRSSURL",
		"FontFamily", "ContentFontSize", "SidebarFontSize",
		"SidebarListLimit", "SidebarListLimitPopular", "SidebarListLimitLatest",
		"ImageCaptionEnabled", "ImageCaptionMode", "ImageCaptionTemplate",
		"DefaultTitle", "DefaultDesc", "DefaultImage", "DefaultKeywords",
		"SourceDisclaimerMode", "SourceDisclaimerTemplate",
		"AlsoReadEnabled", "AlsoReadMaxInserts",
		"PostDateRefreshEnabled", "PostDateRefreshDays", "PostDateIntervalMins",
		"PostDateUrlRecycleEnabled", "PostDateUrlRecycleIntervalMins",
		"AdsTxt", "RobotsTxt",
	}
}

// ensureSiteSettingJSONColumns adds JSON blob columns before backfill (unexported entity fields are not AutoMigrate targets).
func ensureSiteSettingJSONColumns(db *gorm.DB) error {
	m := db.Migrator()
	for _, col := range []string{
		"AdsScript", "ImgResizeSelf", "ImgResizeCDN", "SocialLinks", "Typography",
		"SidebarConfig", "ImageCaption", "DefaultMeta", "SourceDisclaimer",
		"AlsoRead", "PostDateConfig", "SeoFiles",
	} {
		if m.HasColumn(&siteSettingJSONColumns{}, col) {
			continue
		}
		if err := m.AddColumn(&siteSettingJSONColumns{}, col); err != nil {
			return err
		}
	}
	if !m.HasColumn(&siteSettingJSONColumns{}, "ColorTheme") {
		return nil
	}
	// Legacy color_theme was varchar(24) preset key; JSON payloads need TEXT.
	return db.Exec("ALTER TABLE site_settings MODIFY COLUMN color_theme TEXT").Error
}

// backfillSiteSettingJSONColumns copies legacy flat columns into JSON blobs when JSON is still empty.
func backfillSiteSettingJSONColumns(db *gorm.DB) error {
	if !legacySiteSettingHasFlatColumns(db) {
		return nil
	}
	var legacyRows []siteSettingLegacyFlat
	if err := db.Find(&legacyRows).Error; err != nil {
		return err
	}
	for _, leg := range legacyRows {
		var s entity.SiteSetting
		if err := db.First(&s, leg.ID).Error; err != nil {
			return err
		}
		mergeLegacyFlatIntoSiteSetting(&s, leg)
		if err := saveSiteSettingJSONColumns(db, &s); err != nil {
			return err
		}
	}
	return nil
}

// repackSiteSettingJSONColumns ensures every row stores JSON blobs (e.g. legacy color_theme preset strings).
func repackSiteSettingJSONColumns(db *gorm.DB) error {
	var rows []entity.SiteSetting
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	hasLegacy := legacySiteSettingHasFlatColumns(db)
	for i := range rows {
		entity.UnpackSiteSettingJSON(&rows[i])
		if rows[i].JSONColumnsComplete() {
			continue
		}
		if hasLegacy {
			var leg siteSettingLegacyFlat
			if err := db.First(&leg, rows[i].ID).Error; err != nil {
				return err
			}
			mergeLegacyFlatIntoSiteSetting(&rows[i], leg)
		}
		if err := saveSiteSettingJSONColumns(db, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}

func verifySiteSettingJSONMigrated(db *gorm.DB) error {
	var rows []entity.SiteSetting
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		entity.UnpackSiteSettingJSON(&rows[i])
		if !rows[i].JSONColumnsComplete() {
			return fmt.Errorf(
				"site_settings id=%d: JSON columns belum lengkap sebelum kolom legacy dihapus (color_theme=%q ads_script=%q)",
				rows[i].ID,
				truncateMigrationDebug(rows[i].StoredColorTheme),
				truncateMigrationDebug(rows[i].StoredAdsScript),
			)
		}
	}
	return nil
}

func truncateMigrationDebug(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 40 {
		return s
	}
	return s[:40] + "..."
}

func mergeLegacyFlatIntoSiteSetting(s *entity.SiteSetting, leg siteSettingLegacyFlat) {
	if s == nil {
		return
	}
	if entity.JSONBlobEmpty(s.StoredColorTheme) || !entity.IsJSONBlob(s.StoredColorTheme) {
		applyLegacyColorTheme(s, leg)
	}
	if entity.JSONBlobEmpty(s.StoredAdsScript) {
		s.HeaderScript = leg.HeaderScript
		s.FooterScript = leg.FooterScript
		s.AdsTop = leg.AdsTop
		s.AdsCenter = leg.AdsCenter
		s.AdsBottom = leg.AdsBottom
		s.AdsLazyLoadEnabled = leg.AdsLazyLoadEnabled
		s.ArticleAdsJSON = leg.ArticleAdsJSON
	}
	if entity.JSONBlobEmpty(s.StoredImgResizeSelf) {
		s.ImgResizeEnabled = leg.ImgResizeEnabled
		s.ImgResizeWidth = leg.ImgResizeWidth
		s.ImgResizeHeight = leg.ImgResizeHeight
		s.ImgResizeQuality = leg.ImgResizeQuality
		s.StoredImageMaxWidth = leg.StoredImageMaxWidth
		s.StoredImageQuality = leg.StoredImageQuality
		s.AvatarResizeWidth = leg.AvatarResizeWidth
		s.AvatarResizeQuality = leg.AvatarResizeQuality
	}
	if entity.JSONBlobEmpty(s.StoredImgResizeCDN) {
		s.ImageResizeEnabled = leg.ImageResizeEnabled
		s.ImageResizeWidth = leg.ImageResizeWidth
		s.ImageResizeQuality = leg.ImageResizeQuality
	}
	if entity.JSONBlobEmpty(s.StoredSocialLinks) {
		s.SocialFacebookURL = leg.SocialFacebookURL
		s.SocialXURL = leg.SocialXURL
		s.SocialInstagramURL = leg.SocialInstagramURL
		s.SocialYouTubeURL = leg.SocialYouTubeURL
		s.SocialTikTokURL = leg.SocialTikTokURL
		s.SocialRSSURL = leg.SocialRSSURL
	}
	if entity.JSONBlobEmpty(s.StoredTypography) {
		s.FontFamily = leg.FontFamily
		s.ContentFontSize = leg.ContentFontSize
		s.SidebarFontSize = leg.SidebarFontSize
	}
	if entity.JSONBlobEmpty(s.StoredSidebarConfig) {
		s.SidebarListLimit = leg.SidebarListLimit
		s.SidebarListLimitPopular = leg.SidebarListLimitPopular
		s.SidebarListLimitLatest = leg.SidebarListLimitLatest
	}
	if entity.JSONBlobEmpty(s.StoredImageCaption) {
		s.ImageCaptionEnabled = leg.ImageCaptionEnabled
		s.ImageCaptionMode = leg.ImageCaptionMode
		s.ImageCaptionTemplate = leg.ImageCaptionTemplate
	}
	if entity.JSONBlobEmpty(s.StoredDefaultMeta) {
		s.DefaultTitle = leg.DefaultTitle
		s.DefaultDesc = leg.DefaultDesc
		s.DefaultImage = leg.DefaultImage
		s.DefaultKeywords = leg.DefaultKeywords
	}
	if entity.JSONBlobEmpty(s.StoredSourceDisclaimer) {
		s.SourceDisclaimerMode = leg.SourceDisclaimerMode
		s.SourceDisclaimerTemplate = leg.SourceDisclaimerTemplate
	}
	if entity.JSONBlobEmpty(s.StoredAlsoRead) {
		s.AlsoReadEnabled = leg.AlsoReadEnabled
		s.AlsoReadMaxInserts = leg.AlsoReadMaxInserts
		s.AlsoReadParagraphInterval = entity.DefaultAlsoReadParagraphInterval
	}
	if entity.JSONBlobEmpty(s.StoredPostDateConfig) {
		s.PostDateRefreshEnabled = leg.PostDateRefreshEnabled
		s.PostDateRefreshDays = leg.PostDateRefreshDays
		s.PostDateIntervalMins = leg.PostDateIntervalMins
		s.PostDateUrlRecycleEnabled = leg.PostDateUrlRecycleEnabled
		s.PostDateUrlRecycleIntervalMins = leg.PostDateUrlRecycleIntervalMins
	}
	if entity.JSONBlobEmpty(s.StoredSeoFiles) {
		s.AdsTxt = leg.AdsTxt
		s.RobotsTxt = leg.RobotsTxt
	}
}

func applyLegacyColorTheme(s *entity.SiteSetting, leg siteSettingLegacyFlat) {
	presetRaw := strings.TrimSpace(s.StoredColorTheme)
	if presetRaw == "" || entity.IsJSONBlob(presetRaw) {
		presetRaw = entity.ColorThemeDefault
	}
	s.ColorTheme = entity.NormalizeColorTheme(presetRaw)
	if strings.TrimSpace(leg.TextPrimary) != "" {
		s.TextPrimary = leg.TextPrimary
		s.TextSecondary = leg.TextSecondary
		s.BackgroundPrimary = leg.BackgroundPrimary
		s.BackgroundSecondary = leg.BackgroundSecondary
		s.PrimaryButton = leg.PrimaryButton
		s.SecondaryButton = leg.SecondaryButton
		s.AccentColor = leg.AccentColor
		s.BorderColor = leg.BorderColor
		if s.ColorTheme != entity.ColorThemeCustom {
			s.ColorTheme = entity.ColorThemeCustom
		}
		return
	}
	if s.ColorTheme != entity.ColorThemeCustom {
		c := entity.SiteColorPreset(s.ColorTheme)
		s.TextPrimary = c.TextPrimary
		s.TextSecondary = c.TextSecondary
		s.BackgroundPrimary = c.BackgroundPrimary
		s.BackgroundSecondary = c.BackgroundSecondary
		s.PrimaryButton = c.PrimaryButton
		s.SecondaryButton = c.SecondaryButton
		s.AccentColor = c.AccentColor
		s.BorderColor = c.BorderColor
	}
}

func saveSiteSettingJSONColumns(db *gorm.DB, s *entity.SiteSetting) error {
	return db.Model(&entity.SiteSetting{}).Where("id = ?", s.ID).Updates(entity.SiteSettingJSONColumnUpdates(s)).Error
}

func dropSiteSettingLegacyFlatColumns(db *gorm.DB) error {
	if !legacySiteSettingHasFlatColumns(db) {
		return nil
	}
	legacy := siteSettingLegacyFlat{}
	for _, col := range siteSettingLegacyFlatColumnNames() {
		if !db.Migrator().HasColumn(&legacy, col) {
			continue
		}
		if err := db.Migrator().DropColumn(&legacy, col); err != nil {
			return err
		}
	}
	return nil
}

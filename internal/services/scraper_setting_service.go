package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
)

// ScraperSettingService reads and updates scraper settings.
type ScraperSettingService interface {
	Get() (*dto.ScraperSettingResponse, error)
	Update(req *dto.UpdateScraperSettingRequest) (*dto.ScraperSettingResponse, error)
}

type scraperSettingService struct {
	settingRepo repositories.ScraperSettingRepository
	feedRepo    repositories.ScraperCategoryFeedRepository
	db          *gorm.DB
}

// NewScraperSettingService constructs ScraperSettingService.
func NewScraperSettingService(
	settingRepo repositories.ScraperSettingRepository,
	feedRepo repositories.ScraperCategoryFeedRepository,
	db *gorm.DB,
) ScraperSettingService {
	return &scraperSettingService{
		settingRepo: settingRepo,
		feedRepo:    feedRepo,
		db:          db,
	}
}

func (s *scraperSettingService) Get() (*dto.ScraperSettingResponse, error) {
	if err := s.ensureFeedRowsForAllCategories(); err != nil {
		return nil, err
	}
	if err := s.backfillCategoryAuthorsFromLegacyGlobal(); err != nil {
		return nil, err
	}
	existing, err := s.settingRepo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Singleton row is required for API responses; without it the admin GET returned 404
			// and the per-category feed UI never rendered even when categories exist.
			created := &entity.ScraperSetting{
				Enabled:      false,
				IntervalMins: 5,
			}
			if err := s.settingRepo.Create(created); err != nil {
				return nil, err
			}
			existing = created
		} else {
			return nil, err
		}
	}
	aiSetting, err := s.getOrCreateAISetting()
	if err != nil {
		return nil, err
	}
	scraperAIProvider := strings.ToLower(strings.TrimSpace(aiSetting.ScraperAIProvider))
	if scraperAIProvider == "" {
		scraperAIProvider = "auto"
	}
	var cats []entity.Category
	if err := s.db.Order("name asc").Find(&cats).Error; err != nil {
		return nil, err
	}
	feedRows, err := s.feedRepo.List()
	if err != nil {
		return nil, err
	}
	type feedInfo struct {
		urls         string
		enabled      bool
		srcOn        bool
		articleStat  string
		imgMode           string
		webSearchSource   string
		imgToContent      bool
		authorEmail       string
	}
	feedBySlug := make(map[string]feedInfo, len(feedRows))
	for _, f := range feedRows {
		feedBySlug[strings.TrimSpace(f.CategorySlug)] = feedInfo{
			urls:         f.FeedURLs,
			enabled:      f.Enabled,
			srcOn:        f.IncludeSource,
			articleStat:  strings.ToLower(strings.TrimSpace(f.ArticleStatus)),
			imgMode:           strings.TrimSpace(f.ImageMode),
			webSearchSource:   strings.TrimSpace(f.WebSearchSource),
			imgToContent:      f.ImageToContent,
			authorEmail:       strings.TrimSpace(f.AuthorEmail),
		}
	}
	entries := make([]dto.CategoryFeedEntry, 0, len(cats))
	for _, c := range cats {
		info, ok := feedBySlug[c.Slug]
		en := true
		urls := ""
		srcOn := true
		articleStatus := "published"
		imgMode := "scrape"
		webSearchSource := "auto"
		imgToContent := false
		authEm := ""
		if ok {
			en = info.enabled
			urls = info.urls
			srcOn = info.srcOn
			if info.articleStat == "draft" {
				articleStatus = "draft"
			}
			authEm = info.authorEmail
			if info.imgMode != "" {
				imgMode = info.imgMode
			}
			if info.webSearchSource != "" {
				webSearchSource = info.webSearchSource
			}
			imgToContent = info.imgToContent
		}
		entries = append(entries, dto.CategoryFeedEntry{
			CategorySlug:      c.Slug,
			CategoryName:      c.Name,
			FeedURLs:          urls,
			Enabled:           en,
			IncludeSource:     srcOn,
			ArticleStatus:     articleStatus,
			ImageMode:         imgMode,
			WebSearchSource:   webSearchSource,
			ImageToContent:    imgToContent,
			AuthorEmail:       authEm,
		})
	}
	pauseBetween := existing.PauseBetweenCategoriesMins
	if pauseBetween < 0 {
		pauseBetween = 0
	}
	if pauseBetween > 1440 {
		pauseBetween = 1440
	}
	return &dto.ScraperSettingResponse{
		ID:                         existing.ID,
		Enabled:                    existing.Enabled,
		IntervalMins:               existing.IntervalMins,
		PauseBetweenCategoriesMins: pauseBetween,
		ScraperAIProvider:          scraperAIProvider,
		CategoryFeeds:              entries,
	}, nil
}

func (s *scraperSettingService) Update(req *dto.UpdateScraperSettingRequest) (*dto.ScraperSettingResponse, error) {
	interval := req.IntervalMins
	if interval <= 0 {
		interval = 5
	}
	scraperAIProvider := strings.ToLower(strings.TrimSpace(req.ScraperAIProvider))
	if scraperAIProvider == "" {
		scraperAIProvider = "auto"
	}
	pauseBetween := req.PauseBetweenCategoriesMins
	if pauseBetween < 0 {
		pauseBetween = 0
	}
	if pauseBetween > 1440 {
		pauseBetween = 1440
	}

	existing, err := s.settingRepo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := &entity.ScraperSetting{
				Enabled:                    req.Enabled,
				IntervalMins:               interval,
				PauseBetweenCategoriesMins: pauseBetween,
			}
			if err := s.settingRepo.Create(created); err != nil {
				return nil, err
			}
			aiSetting, err := s.getOrCreateAISetting()
			if err != nil {
				return nil, err
			}
			aiSetting.ScraperAIProvider = scraperAIProvider
			if err := s.db.Save(aiSetting).Error; err != nil {
				return nil, err
			}
			if err := s.syncCategoryFeedsIfRequested(req); err != nil {
				return nil, err
			}
			return s.Get()
		}
		return nil, err
	}

	existing.Enabled = req.Enabled
	existing.IntervalMins = interval
	existing.PauseBetweenCategoriesMins = pauseBetween

	if err := s.settingRepo.Save(existing); err != nil {
		return nil, err
	}
	aiSetting, err := s.getOrCreateAISetting()
	if err != nil {
		return nil, err
	}
	aiSetting.ScraperAIProvider = scraperAIProvider
	if err := s.db.Save(aiSetting).Error; err != nil {
		return nil, err
	}
	if err := s.syncCategoryFeedsIfRequested(req); err != nil {
		return nil, err
	}
	return s.Get()
}

func (s *scraperSettingService) getOrCreateAISetting() (*entity.AISetting, error) {
	var aiSetting entity.AISetting
	if err := s.db.First(&aiSetting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			aiSetting = entity.AISetting{ScraperAIProvider: "auto"}
			if err := s.db.Create(&aiSetting).Error; err != nil {
				return nil, err
			}
			return &aiSetting, nil
		}
		return nil, err
	}
	if strings.TrimSpace(aiSetting.ScraperAIProvider) == "" {
		aiSetting.ScraperAIProvider = "auto"
	}
	return &aiSetting, nil
}

func (s *scraperSettingService) syncCategoryFeedsIfRequested(req *dto.UpdateScraperSettingRequest) error {
	if req.CategoryFeeds == nil {
		return nil
	}
	if err := s.feedRepo.DedupeByCanonicalSlug(); err != nil {
		return err
	}
	var cats []entity.Category
	if err := s.db.Order("name asc").Find(&cats).Error; err != nil {
		return err
	}
	validSlug := make(map[string]bool, len(cats))
	for _, c := range cats {
		validSlug[entity.CanonicalCategorySlug(c.Slug)] = true
	}
	reqBySlug := make(map[string]string)
	reqEnabled := make(map[string]bool)
	reqIncludeSource := make(map[string]bool)
	reqArticleStatus := make(map[string]*string)
	reqImageMode := make(map[string]*string)
	reqWebSearchSource := make(map[string]*string)
	reqImageToContent := make(map[string]*bool)
	reqAuthorEmail := make(map[string]string)
	reqAuthorEmailSet := make(map[string]bool)
	hasReqCat := make(map[string]bool)
	for _, cf := range req.CategoryFeeds {
		slug := strings.TrimSpace(cf.CategorySlug)
		canon := entity.CanonicalCategorySlug(slug)
		if !validSlug[canon] {
			return fmt.Errorf("unknown category_slug: %s", slug)
		}
		hasReqCat[canon] = true
		reqBySlug[canon] = strings.TrimSpace(cf.FeedURLs)
		reqEnabled[canon] = cf.Enabled
		reqIncludeSource[canon] = cf.IncludeSource
		if cf.ArticleStatus != nil {
			v := strings.ToLower(strings.TrimSpace(*cf.ArticleStatus))
			reqArticleStatus[canon] = &v
		}
		if cf.AuthorEmail != nil {
			reqAuthorEmailSet[canon] = true
			reqAuthorEmail[canon] = strings.TrimSpace(*cf.AuthorEmail)
		}
		if cf.ImageMode != nil {
			v := strings.TrimSpace(*cf.ImageMode)
			reqImageMode[canon] = &v
		}
		if cf.WebSearchSource != nil {
			v := strings.TrimSpace(*cf.WebSearchSource)
			reqWebSearchSource[canon] = &v
		}
		if cf.ImageToContent != nil {
			v := *cf.ImageToContent
			reqImageToContent[canon] = &v
		}
	}
	existingRows, err := s.feedRepo.List()
	if err != nil {
		return err
	}
	existingMap := make(map[string]string, len(existingRows))
	existingEnabled := make(map[string]bool, len(existingRows))
	existingIncludeSource := make(map[string]bool, len(existingRows))
	existingArticleStatus := make(map[string]string, len(existingRows))
	existingImageMode := make(map[string]string, len(existingRows))
	existingWebSearchSource := make(map[string]string, len(existingRows))
	existingAuthorEmail := make(map[string]string, len(existingRows))
	existingImageToContent := make(map[string]bool, len(existingRows))
	for _, e := range existingRows {
		slugKey := entity.CanonicalCategorySlug(e.CategorySlug)
		existingMap[slugKey] = e.FeedURLs
		existingEnabled[slugKey] = e.Enabled
		existingIncludeSource[slugKey] = e.IncludeSource
		existingArticleStatus[slugKey] = strings.ToLower(strings.TrimSpace(e.ArticleStatus))
		existingImageMode[slugKey] = strings.TrimSpace(e.ImageMode)
		existingWebSearchSource[slugKey] = strings.TrimSpace(e.WebSearchSource)
		existingAuthorEmail[slugKey] = strings.TrimSpace(e.AuthorEmail)
		existingImageToContent[slugKey] = e.ImageToContent
	}
	out := make([]entity.ScraperCategoryFeed, 0, len(cats))
	for _, c := range cats {
		catSlug := strings.TrimSpace(c.Slug)
		catKey := entity.CanonicalCategorySlug(catSlug)
		urls := existingMap[catKey]
		if u, ok := reqBySlug[catKey]; ok {
			urls = u
		}
		en := true
		if v, ok := existingEnabled[catKey]; ok {
			en = v
		}
		srcOn := true
		if v, ok := existingIncludeSource[catKey]; ok {
			srcOn = v
		}
		articleStatus := "published"
		if v, ok := existingArticleStatus[catKey]; ok && v == "draft" {
			articleStatus = "draft"
		}
		imgMode := "scrape"
		webSearchSource := "auto"
		authEm := ""
		imgToContent := false
		autoTag := true
		if v, ok := existingAuthorEmail[catKey]; ok {
			authEm = v
		}
		if v, ok := existingImageToContent[catKey]; ok {
			imgToContent = v
		}
		if v, ok := existingImageMode[catKey]; ok && v != "" {
			imgMode = v
		}
		if v, ok := existingWebSearchSource[catKey]; ok && v != "" {
			webSearchSource = v
		}
		if hasReqCat[catKey] {
			en = reqEnabled[catKey]
			if v, ok := reqIncludeSource[catKey]; ok {
				srcOn = v
			}
			if v, ok := reqArticleStatus[catKey]; ok && v != nil && strings.TrimSpace(*v) != "" {
				if strings.ToLower(strings.TrimSpace(*v)) == "draft" {
					articleStatus = "draft"
				} else {
					articleStatus = "published"
				}
			}
			if reqAuthorEmailSet[catKey] {
				authEm = reqAuthorEmail[catKey]
			}
			if v, ok := reqImageMode[catKey]; ok && v != nil && strings.TrimSpace(*v) != "" {
				imgMode = strings.TrimSpace(*v)
			}
			if v, ok := reqWebSearchSource[catKey]; ok && v != nil && strings.TrimSpace(*v) != "" {
				webSearchSource = strings.TrimSpace(*v)
			}
			if v, ok := reqImageToContent[catKey]; ok && v != nil {
				imgToContent = *v
			}
		}
		out = append(out, entity.ScraperCategoryFeed{
			CategorySlug:      catSlug,
			FeedURLs:          urls,
			Enabled:           en,
			IncludeSource:     srcOn,
			ArticleStatus:     articleStatus,
			ImageMode:         imgMode,
			WebSearchSource:   webSearchSource,
			ImageToContent:    imgToContent,
			AutoTag:           autoTag,
			AuthorEmail:       authEm,
		})
	}
	return s.feedRepo.ReplaceAll(out)
}

// backfillCategoryAuthorsFromLegacyGlobal copies scraper_settings.author_email into any
// scraper_category_feeds row that still has an empty author_email (upgrade from pre-per-category storage).
func (s *scraperSettingService) backfillCategoryAuthorsFromLegacyGlobal() error {
	var global entity.ScraperSetting
	if err := s.db.Order("id desc").First(&global).Error; err != nil {
		return nil
	}
	leg := strings.TrimSpace(global.AuthorEmail)
	if leg == "" {
		return nil
	}
	return s.db.Model(&entity.ScraperCategoryFeed{}).
		Where("author_email IS NULL OR TRIM(author_email) = ?", "").
		Update("author_email", leg).Error
}

func (s *scraperSettingService) ensureFeedRowsForAllCategories() error {
	if err := s.feedRepo.DedupeByCanonicalSlug(); err != nil {
		return err
	}
	var cats []entity.Category
	if err := s.db.Find(&cats).Error; err != nil {
		return err
	}
	var legacyAuthor string
	var global entity.ScraperSetting
	if err := s.db.Order("id desc").First(&global).Error; err == nil {
		legacyAuthor = strings.TrimSpace(global.AuthorEmail)
	}
	if len(cats) == 0 {
		return s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.ScraperCategoryFeed{}).Error
	}
	slugs := make([]string, 0, len(cats))
	for _, c := range cats {
		slugs = append(slugs, strings.TrimSpace(c.Slug))
	}
	if err := s.db.Where("category_slug NOT IN ?", slugs).Delete(&entity.ScraperCategoryFeed{}).Error; err != nil {
		return err
	}
	rows, err := s.feedRepo.List()
	if err != nil {
		return err
	}
	have := make(map[string]bool, len(rows))
	for _, r := range rows {
		have[entity.CanonicalCategorySlug(r.CategorySlug)] = true
	}
	var missing []entity.ScraperCategoryFeed
	for _, c := range cats {
		catSlug := strings.TrimSpace(c.Slug)
		if !have[entity.CanonicalCategorySlug(catSlug)] {
			missing = append(missing, entity.ScraperCategoryFeed{
				CategorySlug:   catSlug,
				FeedURLs:       "",
				Enabled:        true,
				IncludeSource:  true,
				ArticleStatus:  "published",
				ImageMode:         "scrape",
				WebSearchSource:   "auto",
				ImageToContent: false,
				AutoTag:        true,
				AuthorEmail:    legacyAuthor,
			})
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return s.db.Create(&missing).Error
}

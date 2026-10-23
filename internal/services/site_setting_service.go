package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
)

// ErrInvalidSiteScript marks invalid header/footer script payloads.
var ErrInvalidSiteScript = errors.New("invalid site script")

// ErrInvalidArticleSlugMode marks an unsupported article_slug_mode value.
var ErrInvalidArticleSlugMode = errors.New("invalid article slug mode")
var cssFontSizeRe = regexp.MustCompile(`^(?:\d+(?:\.\d+)?)(?:px|rem|em|%)?$`)
var cssFontFamilyUnsafeRe = regexp.MustCompile(`[<>{};\n\r]`)

// SiteSettingService reads and updates site branding settings.
type SiteSettingService interface {
	Get() (*entity.SiteSetting, error)
	Update(req *dto.UpdateSiteSettingRequest) (*entity.SiteSetting, error)
	UpdateScripts(req *dto.UpdateSiteScriptsRequest) (*entity.SiteSetting, error)
}

type siteSettingService struct {
	repo repositories.SiteSettingRepository
}

// NewSiteSettingService constructs SiteSettingService.
func NewSiteSettingService(r repositories.SiteSettingRepository) SiteSettingService {
	return &siteSettingService{repo: r}
}

func (s *siteSettingService) Get() (*entity.SiteSetting, error) {
	row, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d := entity.DefaultSiteSetting()
			return &d, nil
		}
		return nil, err
	}
	row.ColorTheme = entity.NormalizeColorTheme(strings.TrimSpace(row.ColorTheme))
	row.FontFamily = normalizeFontFamilyToken(row.FontFamily, `system-ui, -apple-system, "Segoe UI", Roboto, Ubuntu, "Helvetica Neue", sans-serif`)
	row.ContentFontSize = normalizeFontSizeToken(row.ContentFontSize, "17px")
	row.SidebarFontSize = normalizeFontSizeToken(row.SidebarFontSize, "14px")
	entity.NormalizeSidebarListLimits(row)
	row.ImgResizeWidth = normalizeIntToken(row.ImgResizeWidth, 0, 0, 4000)
	row.ImgResizeHeight = normalizeIntToken(row.ImgResizeHeight, 0, 0, 4000)
	row.ImgResizeQuality = normalizeIntToken(row.ImgResizeQuality, 0, 1, 100)
	row.ImageResizeWidth = normalizeIntToken(row.ImageResizeWidth, 0, 0, 4000)
	row.ImageResizeQuality = normalizeIntToken(row.ImageResizeQuality, 0, 1, 100)
	backfillImgResizeFromLegacy(row)
	row.StoredImageMaxWidth = normalizeIntToken(row.StoredImageMaxWidth, 0, 0, 4000)
	row.StoredImageQuality = normalizeIntToken(row.StoredImageQuality, 0, 1, 100)
	row.AvatarResizeWidth = normalizeIntToken(row.AvatarResizeWidth, 0, 0, 4000)
	row.AvatarResizeQuality = normalizeIntToken(row.AvatarResizeQuality, 0, 1, 100)
	row.LoginURL = normalizeLoginURLInput(row.LoginURL)
	row.ArticleSlugMode = entity.NormalizeArticleSlugMode(row.ArticleSlugMode)
	row.ArticleSchemaType = entity.NormalizeArticleSchemaType(row.ArticleSchemaType)
	row.SourceDisclaimerMode = normalizeSourceDisclaimerMode(row.SourceDisclaimerMode)
	row.SourceDisclaimerTemplate = normalizeSourceDisclaimerTemplate(row.SourceDisclaimerTemplate)
	row.ImageCaptionMode = normalizeImageCaptionMode(row.ImageCaptionMode)
	row.ImageCaptionTemplate = normalizeImageCaptionTemplate(row.ImageCaptionTemplate)
	return row, nil
}

func normalizeFontSizeToken(raw, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return fallback
	}
	if !cssFontSizeRe.MatchString(s) {
		return fallback
	}
	if strings.HasSuffix(s, "px") || strings.HasSuffix(s, "rem") || strings.HasSuffix(s, "em") || strings.HasSuffix(s, "%") {
		return s
	}
	return s + "px"
}

func normalizeIntToken(raw string, fallback, min, max int) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		if fallback <= 0 {
			return ""
		}
		return strconv.Itoa(fallback)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		if fallback <= 0 {
			return ""
		}
		return strconv.Itoa(fallback)
	}
	n = int(math.Max(float64(min), math.Min(float64(max), float64(n))))
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func normalizeFontFamilyToken(raw, fallback string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fallback
	}
	if len(s) > 255 {
		return fallback
	}
	if cssFontFamilyUnsafeRe.MatchString(s) {
		return fallback
	}
	return s
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// backfillImgResizeFromLegacy copies Cloudflare-era values into img_resize_* when the new columns are empty.
func backfillImgResizeFromLegacy(row *entity.SiteSetting) {
	if row == nil {
		return
	}
	if strings.TrimSpace(row.ImgResizeWidth) == "" && strings.TrimSpace(row.ImageResizeWidth) != "" {
		row.ImgResizeWidth = row.ImageResizeWidth
	}
	if strings.TrimSpace(row.ImgResizeQuality) == "" && strings.TrimSpace(row.ImageResizeQuality) != "" {
		row.ImgResizeQuality = row.ImageResizeQuality
	}
}

func normalizeSourceDisclaimerMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case entity.SourceDisclaimerCustom:
		return entity.SourceDisclaimerCustom
	default:
		return entity.SourceDisclaimerDefault
	}
}

func normalizeSourceDisclaimerTemplate(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return entity.DefaultSourceDisclaimerTemplate
	}
	return s
}

func normalizeImageCaptionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case entity.ImageCaptionModeCustom:
		return entity.ImageCaptionModeCustom
	default:
		return entity.ImageCaptionModeDefault
	}
}

func normalizeImageCaptionTemplate(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return entity.DefaultImageCaptionTemplate
	}
	return s
}

func normalizeLoginURLInput(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "/login"
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	pathOnly := strings.TrimLeft(s, "/")
	if pathOnly == "" {
		return "/login"
	}
	return "/" + pathOnly
}

func applySiteColorsFromRequest(dst *entity.SiteSetting, req *dto.UpdateSiteSettingRequest) {
	theme := entity.NormalizeColorTheme(req.ColorTheme)
	dst.ColorTheme = theme
	if theme == entity.ColorThemeCustom {
		dst.TextPrimary = req.TextPrimary
		dst.TextSecondary = req.TextSecondary
		dst.BackgroundPrimary = req.BackgroundPrimary
		dst.BackgroundSecondary = req.BackgroundSecondary
		dst.PrimaryButton = req.PrimaryButton
		dst.SecondaryButton = req.SecondaryButton
		dst.AccentColor = req.AccentColor
		dst.BorderColor = req.BorderColor
		return
	}
	c := entity.SiteColorPreset(theme)
	dst.TextPrimary = c.TextPrimary
	dst.TextSecondary = c.TextSecondary
	dst.BackgroundPrimary = c.BackgroundPrimary
	dst.BackgroundSecondary = c.BackgroundSecondary
	dst.PrimaryButton = c.PrimaryButton
	dst.SecondaryButton = c.SecondaryButton
	dst.AccentColor = c.AccentColor
	dst.BorderColor = c.BorderColor
}

func sidebarLimitsFromRequest(req *dto.UpdateSiteSettingRequest) (popular, latest int) {
	popIn := req.SidebarListLimitPopular
	latIn := req.SidebarListLimitLatest
	if popIn <= 0 && req.SidebarListLimit > 0 {
		popIn = req.SidebarListLimit
	}
	if latIn <= 0 && req.SidebarListLimit > 0 {
		latIn = req.SidebarListLimit
	}
	return entity.EffectiveSidebarListLimit(popIn), entity.EffectiveSidebarListLimit(latIn)
}

func (s *siteSettingService) Update(req *dto.UpdateSiteSettingRequest) (*entity.SiteSetting, error) {
	if mode := strings.TrimSpace(req.ArticleSlugMode); mode != "" && !entity.IsValidArticleSlugMode(mode) {
		return nil, ErrInvalidArticleSlugMode
	}
	existing, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sidebarPop, sidebarLat := sidebarLimitsFromRequest(req)
			created := &entity.SiteSetting{
				SiteName:                       req.SiteName,
				FooterCopy:                     strings.TrimSpace(req.FooterCopy),
				Tagline:                        req.Tagline,
				LogoURL:                        req.LogoURL,
				FaviconURL:                     req.FaviconURL,
				LoginURL:                       normalizeLoginURLInput(req.LoginURL),
				NotFoundRedirectMode:           entity.NormalizeNotFoundRedirectMode(req.NotFoundRedirectMode),
				DefaultTitle:                   req.DefaultTitle,
				DefaultDesc:                    req.DefaultDesc,
				DefaultImage:                   req.DefaultImage,
				DefaultKeywords:                req.DefaultKeywords,
				HomepageLayout:                 entity.NormalizeHomeLayout(strings.TrimSpace(req.HomepageLayout)),
				FontFamily:                     normalizeFontFamilyToken(req.FontFamily, `system-ui, -apple-system, "Segoe UI", Roboto, Ubuntu, "Helvetica Neue", sans-serif`),
				ContentFontSize:                normalizeFontSizeToken(req.ContentFontSize, "17px"),
				SidebarFontSize:                normalizeFontSizeToken(req.SidebarFontSize, "14px"),
				SidebarListLimit:        sidebarPop,
				SidebarListLimitPopular: sidebarPop,
				SidebarListLimitLatest:  sidebarLat,
				ImageCaptionEnabled:            req.ImageCaptionEnabled,
				ImageCaptionMode:               normalizeImageCaptionMode(req.ImageCaptionMode),
				ImageCaptionTemplate:           normalizeImageCaptionTemplate(req.ImageCaptionTemplate),
				ImageResizeEnabled: true,
				ImgResizeEnabled:   false,
				ImgResizeWidth:     normalizeIntToken(req.ImgResizeWidth, 0, 0, 4000),
				ImgResizeHeight:    normalizeIntToken(req.ImgResizeHeight, 0, 0, 4000),
				ImgResizeQuality:   normalizeIntToken(req.ImgResizeQuality, 0, 1, 100),
				ImageResizeWidth:   normalizeIntToken(req.ImageResizeWidth, 0, 0, 4000),
				ImageResizeQuality: normalizeIntToken(req.ImageResizeQuality, 0, 1, 100),
				StoredImageMaxWidth:            normalizeIntToken(req.StoredImageMaxWidth, 0, 0, 4000),
				StoredImageQuality:             normalizeIntToken(req.StoredImageQuality, 0, 1, 100),
				AvatarResizeWidth:              normalizeIntToken(req.AvatarResizeWidth, 0, 0, 4000),
				AvatarResizeQuality:            normalizeIntToken(req.AvatarResizeQuality, 0, 1, 100),
				SocialFacebookURL:              strings.TrimSpace(req.SocialFacebookURL),
				SocialXURL:                     strings.TrimSpace(req.SocialXURL),
				SocialInstagramURL:             strings.TrimSpace(req.SocialInstagramURL),
				SocialYouTubeURL:               strings.TrimSpace(req.SocialYouTubeURL),
				SocialTikTokURL:                strings.TrimSpace(req.SocialTikTokURL),
				SocialRSSURL:                   strings.TrimSpace(req.SocialRSSURL),
				ArticleSlugMode:                entity.NormalizeArticleSlugMode(req.ArticleSlugMode),
				ArticleSchemaType:              entity.NormalizeArticleSchemaType(req.ArticleSchemaType),
				ScraperAutoTag:                 true,
				AlsoReadEnabled:                req.AlsoReadEnabled,
				AlsoReadMaxInserts:             entity.EffectiveAlsoReadMaxInserts(req.AlsoReadMaxInserts),
				PostDateRefreshEnabled:         req.PostDateRefreshEnabled,
				PostDateRefreshDays:            clampInt(req.PostDateRefreshDays, 0, 3650),
				PostDateIntervalMins:           clampInt(req.PostDateIntervalMins, 0, 1440),
				PostDateUrlRecycleEnabled:      req.PostDateUrlRecycleEnabled,
				PostDateUrlRecycleIntervalMins: clampInt(req.PostDateUrlRecycleIntervalMins, 0, 1440),
				SourceDisclaimerMode:           normalizeSourceDisclaimerMode(req.SourceDisclaimerMode),
				SourceDisclaimerTemplate:       normalizeSourceDisclaimerTemplate(req.SourceDisclaimerTemplate),
			}
			if req.ScraperAutoTag != nil {
				created.ScraperAutoTag = *req.ScraperAutoTag
			}
			if req.ImageResizeEnabled != nil {
				created.ImageResizeEnabled = *req.ImageResizeEnabled
			}
			created.ImgResizeEnabled = req.ImgResizeEnabled
			applySiteColorsFromRequest(created, req)
			if err := s.repo.Create(created); err != nil {
				return nil, err
			}
			return created, nil
		}
		return nil, err
	}

	existing.SiteName = req.SiteName
	existing.FooterCopy = strings.TrimSpace(req.FooterCopy)
	existing.Tagline = req.Tagline
	existing.LogoURL = req.LogoURL
	existing.FaviconURL = req.FaviconURL
	applySiteColorsFromRequest(existing, req)
	existing.LoginURL = normalizeLoginURLInput(req.LoginURL)
	existing.NotFoundRedirectMode = entity.NormalizeNotFoundRedirectMode(req.NotFoundRedirectMode)
	existing.DefaultTitle = req.DefaultTitle
	existing.DefaultDesc = req.DefaultDesc
	existing.DefaultImage = req.DefaultImage
	existing.DefaultKeywords = req.DefaultKeywords
	existing.HomepageLayout = entity.NormalizeHomeLayout(strings.TrimSpace(req.HomepageLayout))
	existing.FontFamily = normalizeFontFamilyToken(req.FontFamily, `system-ui, -apple-system, "Segoe UI", Roboto, Ubuntu, "Helvetica Neue", sans-serif`)
	existing.ContentFontSize = normalizeFontSizeToken(req.ContentFontSize, "17px")
	existing.SidebarFontSize = normalizeFontSizeToken(req.SidebarFontSize, "14px")
	existing.SidebarListLimitPopular, existing.SidebarListLimitLatest = sidebarLimitsFromRequest(req)
	existing.SidebarListLimit = existing.SidebarListLimitPopular
	existing.ImageCaptionEnabled = req.ImageCaptionEnabled
	existing.ImageCaptionMode = normalizeImageCaptionMode(req.ImageCaptionMode)
	existing.ImageCaptionTemplate = normalizeImageCaptionTemplate(req.ImageCaptionTemplate)
	if req.ImageResizeEnabled != nil {
		existing.ImageResizeEnabled = *req.ImageResizeEnabled
	}
	existing.ImgResizeEnabled = req.ImgResizeEnabled
	existing.ImgResizeWidth = normalizeIntToken(req.ImgResizeWidth, 0, 0, 4000)
	existing.ImgResizeHeight = normalizeIntToken(req.ImgResizeHeight, 0, 0, 4000)
	existing.ImgResizeQuality = normalizeIntToken(req.ImgResizeQuality, 0, 1, 100)
	existing.ImageResizeWidth = normalizeIntToken(req.ImageResizeWidth, 0, 0, 4000)
	existing.ImageResizeQuality = normalizeIntToken(req.ImageResizeQuality, 0, 1, 100)
	existing.StoredImageMaxWidth = normalizeIntToken(req.StoredImageMaxWidth, 0, 0, 4000)
	existing.StoredImageQuality = normalizeIntToken(req.StoredImageQuality, 0, 1, 100)
	existing.AvatarResizeWidth = normalizeIntToken(req.AvatarResizeWidth, 0, 0, 4000)
	existing.AvatarResizeQuality = normalizeIntToken(req.AvatarResizeQuality, 0, 1, 100)
	existing.SocialFacebookURL = strings.TrimSpace(req.SocialFacebookURL)
	existing.SocialXURL = strings.TrimSpace(req.SocialXURL)
	existing.SocialInstagramURL = strings.TrimSpace(req.SocialInstagramURL)
	existing.SocialYouTubeURL = strings.TrimSpace(req.SocialYouTubeURL)
	existing.SocialTikTokURL = strings.TrimSpace(req.SocialTikTokURL)
	existing.SocialRSSURL = strings.TrimSpace(req.SocialRSSURL)
	existing.ArticleSlugMode = entity.NormalizeArticleSlugMode(req.ArticleSlugMode)
	existing.ArticleSchemaType = entity.NormalizeArticleSchemaType(req.ArticleSchemaType)
	if req.ScraperAutoTag != nil {
		existing.ScraperAutoTag = *req.ScraperAutoTag
	}
	if req.AutoHrefEnabled != nil {
		existing.AutoHrefEnabled = *req.AutoHrefEnabled
	}
	existing.AlsoReadEnabled = req.AlsoReadEnabled
	existing.AlsoReadMaxInserts = entity.EffectiveAlsoReadMaxInserts(req.AlsoReadMaxInserts)
	existing.PostDateRefreshEnabled = req.PostDateRefreshEnabled
	existing.PostDateRefreshDays = clampInt(req.PostDateRefreshDays, 0, 3650)
	existing.PostDateIntervalMins = clampInt(req.PostDateIntervalMins, 0, 1440)
	existing.PostDateUrlRecycleEnabled = req.PostDateUrlRecycleEnabled
	existing.PostDateUrlRecycleIntervalMins = clampInt(req.PostDateUrlRecycleIntervalMins, 0, 1440)
	existing.SourceDisclaimerMode = normalizeSourceDisclaimerMode(req.SourceDisclaimerMode)
	existing.SourceDisclaimerTemplate = normalizeSourceDisclaimerTemplate(req.SourceDisclaimerTemplate)

	if err := s.repo.Save(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *siteSettingService) UpdateScripts(req *dto.UpdateSiteScriptsRequest) (*entity.SiteSetting, error) {
	headerScript, err := sanitizeSiteScript(req.HeaderScript, "header_script")
	if err != nil {
		return nil, err
	}
	footerScript, err := sanitizeSiteScript(req.FooterScript, "footer_script")
	if err != nil {
		return nil, err
	}
	adsTop, err := sanitizeAdSlotMarkup(req.AdsTop, "ads_top")
	if err != nil {
		return nil, err
	}
	adsCenter, err := sanitizeAdSlotMarkup(req.AdsCenter, "ads_center")
	if err != nil {
		return nil, err
	}
	adsBottom, err := sanitizeAdSlotMarkup(req.AdsBottom, "ads_bottom")
	if err != nil {
		return nil, err
	}
	articleAdsJSON, err := sanitizeArticleAds(req.ArticleAds)
	if err != nil {
		return nil, err
	}
	adsTxt := sanitizeSiteTextFile(req.AdsTxt)
	robotsTxt := sanitizeSiteTextFile(req.RobotsTxt)

	existing, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := entity.DefaultSiteSetting()
			created.HeaderScript = headerScript
			created.FooterScript = footerScript
			created.AdsTop = adsTop
			created.AdsCenter = adsCenter
			created.AdsBottom = adsBottom
			if req.AdsLazyLoadEnabled != nil {
				created.AdsLazyLoadEnabled = *req.AdsLazyLoadEnabled
			}
			created.AdsTxt = adsTxt
			created.RobotsTxt = robotsTxt
			created.ArticleAdsJSON = articleAdsJSON
			if err := s.repo.Create(&created); err != nil {
				return nil, err
			}
			return &created, nil
		}
		return nil, err
	}

	existing.HeaderScript = headerScript
	existing.FooterScript = footerScript
	existing.AdsTop = adsTop
	existing.AdsCenter = adsCenter
	existing.AdsBottom = adsBottom
	if req.AdsLazyLoadEnabled != nil {
		existing.AdsLazyLoadEnabled = *req.AdsLazyLoadEnabled
	}
	existing.AdsTxt = adsTxt
	existing.RobotsTxt = robotsTxt
	existing.ArticleAdsJSON = articleAdsJSON

	if err := s.repo.Save(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func sanitizeSiteScript(raw, fieldName string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<head") || strings.Contains(lower, "<body") {
		return "", fmt.Errorf("%w: %s must not contain html/head/body tags", ErrInvalidSiteScript, fieldName)
	}
	if !strings.Contains(lower, "<script") && !strings.Contains(lower, "<noscript") &&
		!strings.Contains(lower, "<link") && !strings.Contains(lower, "<style") {
		return "", fmt.Errorf("%w: %s must contain script, noscript, link, or style tag", ErrInvalidSiteScript, fieldName)
	}
	return s, nil
}

// sanitizeAdSlotMarkup validates header/footer-style snippets for ad placement fields.
// It allows common embeds (script, AdSense ins, iframes) without requiring every slot to include a script tag.
func sanitizeAdSlotMarkup(raw, fieldName string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<head") || strings.Contains(lower, "<body") {
		return "", fmt.Errorf("%w: %s must not contain html/head/body tags", ErrInvalidSiteScript, fieldName)
	}
	hasScript := strings.Contains(lower, "<script") || strings.Contains(lower, "<noscript")
	hasIns := strings.Contains(lower, "<ins")
	hasIFrame := strings.Contains(lower, "<iframe")
	if !hasScript && !hasIns && !hasIFrame {
		return "", fmt.Errorf("%w: %s must contain a script, noscript, ins, or iframe snippet", ErrInvalidSiteScript, fieldName)
	}
	return s, nil
}

func sanitizeSiteTextFile(raw string) string {
	return strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
}

// articleAdStored is the on-disk JSON shape for an in-article ad.
type articleAdStored struct {
	Position int    `json:"position"`
	Content  string `json:"content"`
}

// sanitizeArticleAds validates each in-article ad and returns the JSON encoding to persist.
// Returns "" when the input list is empty (no ads configured). Items keep their original order
// but are sorted by ascending position for deterministic storage and rendering.
func sanitizeArticleAds(items []dto.ArticleAdInput) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	if len(items) > 10 {
		return "", fmt.Errorf("%w: article_ads must contain at most 10 entries", ErrInvalidSiteScript)
	}
	cleaned := make([]articleAdStored, 0, len(items))
	for i, raw := range items {
		if raw.Position < 1 || raw.Position > 99 {
			return "", fmt.Errorf("%w: article_ads[%d].position must be between 1 and 99", ErrInvalidSiteScript, i)
		}
		content, err := sanitizeAdSlotMarkup(raw.Content, fmt.Sprintf("article_ads[%d].content", i))
		if err != nil {
			return "", err
		}
		if content == "" {
			return "", fmt.Errorf("%w: article_ads[%d].content must not be empty", ErrInvalidSiteScript, i)
		}
		cleaned = append(cleaned, articleAdStored{Position: raw.Position, Content: content})
	}
	sort.SliceStable(cleaned, func(i, j int) bool { return cleaned[i].Position < cleaned[j].Position })
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return "", fmt.Errorf("%w: article_ads encoding failed: %v", ErrInvalidSiteScript, err)
	}
	return string(encoded), nil
}

package seeds

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/scraper"
)

func newAISettingWithPromptDefaults(locale string) entity.AISetting {
	locale = normalizeSeedLocale(locale)
	scrapeTitle := scraper.DefaultScrapeTitlePrompt
	scrapeBody := scraper.DefaultScrapeBodyPrompt
	genTitle := defaultSeedBulkPromptTitle
	genContent := defaultSeedBulkPostPromptTemplate
	metaDesc := defaultSeedBulkPromptMetaDescription
	metaKW := defaultSeedBulkPromptMetaKeywords
	imgPrompt := defaultSeedGenerateImagePrompt
	imgToContentPrompt := defaultSeedGenerateImageToContentPrompt
	if locale == "en" {
		scrapeTitle = `Rewrite the article title in natural English. Keep it accurate and readable, avoid clickbait, and do not include source/site names. Output plain text only.`
		scrapeBody = `Rewrite the article body in natural English with clear structure and short paragraphs. Keep facts consistent, remove duplicated sentences, avoid plagiarism, and do not mention that you are an AI. Output HTML only.`
		genTitle = defaultSeedBulkPromptTitleEN
		genContent = defaultSeedBulkPostPromptTemplateEN
		metaDesc = defaultSeedBulkPromptMetaDescriptionEN
		metaKW = defaultSeedBulkPromptMetaKeywordsEN
		imgPrompt = defaultSeedGenerateImagePromptEN
		imgToContentPrompt = defaultSeedGenerateImageToContentPromptEN
	}
	return entity.AISetting{
		ScraperAIProvider:             "auto",
		GenerateAIProvider:            "auto",
		ScraperPromptTitle:            scrapeTitle,
		ScraperPromptContent:          scrapeBody,
		GeneratePromptTitle:           genTitle,
		GeneratePromptContent:         genContent,
		GeneratePromptContent2:        genContent,
		GeneratePromptContent3:        genContent,
		GeneratePromptMetaDescription: metaDesc,
		GeneratePromptMetaKeywords:    metaKW,
		GeneratePromptImage:           imgPrompt,
		GeneratePromptImageToContent:  imgToContentPrompt,
	}
}

func aiSettingPromptsBlank(a *entity.AISetting) bool {
	if a == nil {
		return true
	}
	return strings.TrimSpace(a.ScraperPromptTitle) == "" &&
		strings.TrimSpace(a.ScraperPromptContent) == "" &&
		strings.TrimSpace(a.GeneratePromptTitle) == "" &&
		strings.TrimSpace(a.GeneratePromptContent) == "" &&
		strings.TrimSpace(a.GeneratePromptContent2) == "" &&
		strings.TrimSpace(a.GeneratePromptContent3) == "" &&
		strings.TrimSpace(a.GeneratePromptMetaDescription) == "" &&
		strings.TrimSpace(a.GeneratePromptMetaKeywords) == "" &&
		strings.TrimSpace(a.GeneratePromptImage) == "" &&
		strings.TrimSpace(a.GeneratePromptImageToContent) == ""
}

// EnsureAISettingDefaults ensures the singleton ai_setting row exists and fills default
// prompts when every prompt field is empty. Use this when full Run() is skipped (site
// settings already exist) or after a row was created empty by GetOrCreate / new columns.
func EnsureAISettingDefaults(db *gorm.DB) error {
	return EnsureAISettingDefaultsForLocale(db, "id")
}

func EnsureAISettingDefaultsForLocale(db *gorm.DB, locale string) error {
	var row entity.AISetting
	err := db.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		def := newAISettingWithPromptDefaults(locale)
		return db.Create(&def).Error
	}
	if err != nil {
		return err
	}
	def := newAISettingWithPromptDefaults(locale)

	// Locale migration: if the row still uses seed defaults from the other locale,
	// switch it to the requested locale defaults. This avoids leaving /en with Indonesian defaults
	// when the DB was seeded before locale support existed.
	if normalizeSeedLocale(locale) == "en" {
		changed := false
		// Scraper prompts: old default is Indonesian constants from scraper package.
		if strings.TrimSpace(row.ScraperPromptTitle) == strings.TrimSpace(scraper.DefaultScrapeTitlePrompt) {
			row.ScraperPromptTitle = def.ScraperPromptTitle
			changed = true
		}
		if strings.TrimSpace(row.ScraperPromptContent) == strings.TrimSpace(scraper.DefaultScrapeBodyPrompt) {
			row.ScraperPromptContent = def.ScraperPromptContent
			changed = true
		}
		// Bulk/generate prompts: swap only when still equal to seed-ID defaults.
		if strings.TrimSpace(row.GeneratePromptTitle) == strings.TrimSpace(defaultSeedBulkPromptTitle) {
			row.GeneratePromptTitle = def.GeneratePromptTitle
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptContent) == strings.TrimSpace(defaultSeedBulkPostPromptTemplate) {
			row.GeneratePromptContent = def.GeneratePromptContent
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptContent2) == strings.TrimSpace(defaultSeedBulkPostPromptTemplate) {
			row.GeneratePromptContent2 = def.GeneratePromptContent2
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptContent3) == strings.TrimSpace(defaultSeedBulkPostPromptTemplate) {
			row.GeneratePromptContent3 = def.GeneratePromptContent3
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptMetaDescription) == strings.TrimSpace(defaultSeedBulkPromptMetaDescription) {
			row.GeneratePromptMetaDescription = def.GeneratePromptMetaDescription
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptMetaKeywords) == strings.TrimSpace(defaultSeedBulkPromptMetaKeywords) {
			row.GeneratePromptMetaKeywords = def.GeneratePromptMetaKeywords
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptImage) == strings.TrimSpace(defaultSeedGenerateImagePrompt) {
			row.GeneratePromptImage = def.GeneratePromptImage
			changed = true
		}
		if strings.TrimSpace(row.GeneratePromptImageToContent) == strings.TrimSpace(defaultSeedGenerateImageToContentPrompt) {
			row.GeneratePromptImageToContent = def.GeneratePromptImageToContent
			changed = true
		}
		if changed {
			return db.Save(&row).Error
		}
	}

	imagePromptsFixed := false
	contentPromptVariantsFixed := false
	baseContent := strings.TrimSpace(row.GeneratePromptContent)
	if baseContent == "" {
		baseContent = def.GeneratePromptContent
	}
	if strings.TrimSpace(row.GeneratePromptContent2) == "" {
		row.GeneratePromptContent2 = baseContent
		contentPromptVariantsFixed = true
	}
	if strings.TrimSpace(row.GeneratePromptContent3) == "" {
		row.GeneratePromptContent3 = baseContent
		contentPromptVariantsFixed = true
	}
	if strings.TrimSpace(row.GeneratePromptImage) == "" {
		row.GeneratePromptImage = def.GeneratePromptImage
		imagePromptsFixed = true
	}
	if strings.TrimSpace(row.GeneratePromptImageToContent) == "" {
		row.GeneratePromptImageToContent = def.GeneratePromptImageToContent
		imagePromptsFixed = true
	}
	if imagePromptsFixed || contentPromptVariantsFixed {
		return db.Save(&row).Error
	}
	if !aiSettingPromptsBlank(&row) {
		return nil
	}
	if strings.TrimSpace(row.ScraperAIProvider) == "" {
		row.ScraperAIProvider = def.ScraperAIProvider
	}
	if strings.TrimSpace(row.GenerateAIProvider) == "" {
		row.GenerateAIProvider = def.GenerateAIProvider
	}
	row.ScraperPromptTitle = def.ScraperPromptTitle
	row.ScraperPromptContent = def.ScraperPromptContent
	row.GeneratePromptTitle = def.GeneratePromptTitle
	row.GeneratePromptContent = def.GeneratePromptContent
	row.GeneratePromptContent2 = def.GeneratePromptContent2
	row.GeneratePromptContent3 = def.GeneratePromptContent3
	row.GeneratePromptMetaDescription = def.GeneratePromptMetaDescription
	row.GeneratePromptMetaKeywords = def.GeneratePromptMetaKeywords
	row.GeneratePromptImage = def.GeneratePromptImage
	row.GeneratePromptImageToContent = def.GeneratePromptImageToContent
	return db.Save(&row).Error
}

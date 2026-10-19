package seeds

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func newAISettingWithPromptDefaults(_ string) entity.AISetting {
	return entity.AISetting{
		ScraperAIProvider:             "auto",
		GenerateAIProvider:            "auto",
		ScraperPromptTitle:            DefaultScrapeTitlePrompt,
		ScraperPromptContent:          DefaultScrapeBodyPrompt,
		GeneratePromptTitle:           DefaultPromptTitle,
		GeneratePromptContent:         DefaultPromptContent,
		GeneratePromptContent2:        DefaultPromptContent,
		GeneratePromptContent3:        DefaultPromptContent,
		GeneratePromptMetaDescription: DefaultPromptMetaDescription,
		GeneratePromptMetaKeywords:    DefaultPromptMetaKeywords,
		GeneratePromptImage:           DefaultGenerateImagePrompt,
		GeneratePromptImageToContent:  DefaultGenerateImageToContentPrompt,
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

	changed := false
	if strings.TrimSpace(row.GeneratePromptTitle) == strings.TrimSpace(DefaultPromptTitle) {
		row.GeneratePromptTitle = def.GeneratePromptTitle
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptContent) == strings.TrimSpace(DefaultPromptContent) {
		row.GeneratePromptContent = def.GeneratePromptContent
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptContent2) == strings.TrimSpace(DefaultPromptContent) {
		row.GeneratePromptContent2 = def.GeneratePromptContent2
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptContent3) == strings.TrimSpace(DefaultPromptContent) {
		row.GeneratePromptContent3 = def.GeneratePromptContent3
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptMetaDescription) == strings.TrimSpace(DefaultPromptMetaDescription) {
		row.GeneratePromptMetaDescription = def.GeneratePromptMetaDescription
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptMetaKeywords) == strings.TrimSpace(DefaultPromptMetaKeywords) {
		row.GeneratePromptMetaKeywords = def.GeneratePromptMetaKeywords
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptImage) == strings.TrimSpace(DefaultGenerateImagePrompt) {
		row.GeneratePromptImage = def.GeneratePromptImage
		changed = true
	}
	if strings.TrimSpace(row.GeneratePromptImageToContent) == strings.TrimSpace(DefaultGenerateImageToContentPrompt) {
		row.GeneratePromptImageToContent = def.GeneratePromptImageToContent
		changed = true
	}
	if changed {
		return db.Save(&row).Error
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

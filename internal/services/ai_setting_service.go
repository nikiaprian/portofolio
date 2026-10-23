package services

import (
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
)

func normalizeBulkCoverBackgroundURLs(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	seen := make(map[string]struct{}, len(parts))
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.TrimSpace(p)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	return strings.Join(urls, "\n")
}

func normalizePromptContentName(raw string, fallback string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fallback
	}
	return s
}

// AISettingService reads and updates the singleton AI settings row.
type AISettingService interface {
	Get() (*entity.AISetting, error)
	Update(req *dto.UpdateAISettingRequest) (*entity.AISetting, error)
	UpdateBulkPostPrompts(req *dto.UpdateBulkPostPromptsRequest) (*entity.AISetting, error)
}

type aiSettingService struct {
	repo repositories.AISettingRepository
}

// NewAISettingService constructs AISettingService.
func NewAISettingService(r repositories.AISettingRepository) AISettingService {
	return &aiSettingService{repo: r}
}

func (s *aiSettingService) Get() (*entity.AISetting, error) {
	return s.repo.GetOrCreate()
}

func (s *aiSettingService) Update(req *dto.UpdateAISettingRequest) (*entity.AISetting, error) {
	row, err := s.repo.GetOrCreate()
	if err != nil {
		return nil, err
	}
	row.Gemini = strings.TrimSpace(req.Gemini)
	row.Groq = strings.TrimSpace(req.Groq)
	row.OpenAI = strings.TrimSpace(req.OpenAI)
	row.Claude = strings.TrimSpace(req.Claude)
	row.GeminiModel = strings.TrimSpace(req.GeminiModel)
	row.GroqModel = strings.TrimSpace(req.GroqModel)
	row.OpenAIModel = strings.TrimSpace(req.OpenAIModel)
	row.ClaudeModel = strings.TrimSpace(req.ClaudeModel)

	row.ScraperAIProvider = strings.TrimSpace(req.ScraperAIProvider)
	if row.ScraperAIProvider == "" {
		row.ScraperAIProvider = "auto"
	}
	row.GenerateAIProvider = strings.TrimSpace(req.GenerateAIProvider)
	if row.GenerateAIProvider == "" {
		row.GenerateAIProvider = "auto"
	}

	if req.ScraperPromptTitle != nil {
		row.ScraperPromptTitle = strings.TrimSpace(*req.ScraperPromptTitle)
	}
	if req.ScraperPromptContent != nil {
		row.ScraperPromptContent = strings.TrimSpace(*req.ScraperPromptContent)
	}
	if req.GeneratePromptTitle != nil {
		row.GeneratePromptTitle = strings.TrimSpace(*req.GeneratePromptTitle)
	}
	if req.GeneratePromptContentName != nil {
		row.GeneratePromptContentName = normalizePromptContentName(*req.GeneratePromptContentName, "Prompt content 1")
	}
	if req.GeneratePromptContent != nil {
		row.GeneratePromptContent = strings.TrimSpace(*req.GeneratePromptContent)
	}
	if req.GeneratePromptContentName2 != nil {
		row.GeneratePromptContentName2 = normalizePromptContentName(*req.GeneratePromptContentName2, "Prompt content 2")
	}
	if req.GeneratePromptContent2 != nil {
		row.GeneratePromptContent2 = strings.TrimSpace(*req.GeneratePromptContent2)
	}
	if req.GeneratePromptContentName3 != nil {
		row.GeneratePromptContentName3 = normalizePromptContentName(*req.GeneratePromptContentName3, "Prompt content 3")
	}
	if req.GeneratePromptContent3 != nil {
		row.GeneratePromptContent3 = strings.TrimSpace(*req.GeneratePromptContent3)
	}
	if req.GeneratePromptMetaDescription != nil {
		row.GeneratePromptMetaDescription = strings.TrimSpace(*req.GeneratePromptMetaDescription)
	}
	if req.GeneratePromptMetaKeywords != nil {
		row.GeneratePromptMetaKeywords = strings.TrimSpace(*req.GeneratePromptMetaKeywords)
	}
	if req.GeneratePromptImage != nil {
		row.GeneratePromptImage = strings.TrimSpace(*req.GeneratePromptImage)
	}
	if req.GeneratePromptImageToContent != nil {
		row.GeneratePromptImageToContent = strings.TrimSpace(*req.GeneratePromptImageToContent)
	}
	row.GeneratePromptContentName = normalizePromptContentName(row.GeneratePromptContentName, "Prompt content 1")
	row.GeneratePromptContentName2 = normalizePromptContentName(row.GeneratePromptContentName2, "Prompt content 2")
	row.GeneratePromptContentName3 = normalizePromptContentName(row.GeneratePromptContentName3, "Prompt content 3")
	row.BulkCoverBackgroundURL = normalizeBulkCoverBackgroundURLs(req.BulkCoverBackgroundURL)

	if err := s.repo.Save(row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *aiSettingService) UpdateBulkPostPrompts(req *dto.UpdateBulkPostPromptsRequest) (*entity.AISetting, error) {
	row, err := s.repo.GetOrCreate()
	if err != nil {
		return nil, err
	}
	if req.GeneratePromptTitle != nil {
		row.GeneratePromptTitle = strings.TrimSpace(*req.GeneratePromptTitle)
	}
	if req.GeneratePromptContentName != nil {
		row.GeneratePromptContentName = normalizePromptContentName(*req.GeneratePromptContentName, "Prompt content 1")
	}
	if req.GeneratePromptContent != nil {
		row.GeneratePromptContent = strings.TrimSpace(*req.GeneratePromptContent)
	}
	if req.GeneratePromptContentName2 != nil {
		row.GeneratePromptContentName2 = normalizePromptContentName(*req.GeneratePromptContentName2, "Prompt content 2")
	}
	if req.GeneratePromptContent2 != nil {
		row.GeneratePromptContent2 = strings.TrimSpace(*req.GeneratePromptContent2)
	}
	if req.GeneratePromptContentName3 != nil {
		row.GeneratePromptContentName3 = normalizePromptContentName(*req.GeneratePromptContentName3, "Prompt content 3")
	}
	if req.GeneratePromptContent3 != nil {
		row.GeneratePromptContent3 = strings.TrimSpace(*req.GeneratePromptContent3)
	}
	if req.GeneratePromptMetaDescription != nil {
		row.GeneratePromptMetaDescription = strings.TrimSpace(*req.GeneratePromptMetaDescription)
	}
	if req.GeneratePromptMetaKeywords != nil {
		row.GeneratePromptMetaKeywords = strings.TrimSpace(*req.GeneratePromptMetaKeywords)
	}
	if req.GeneratePromptImage != nil {
		row.GeneratePromptImage = strings.TrimSpace(*req.GeneratePromptImage)
	}
	if req.GeneratePromptImageToContent != nil {
		row.GeneratePromptImageToContent = strings.TrimSpace(*req.GeneratePromptImageToContent)
	}
	if req.BulkCoverBackgroundURL != nil {
		row.BulkCoverBackgroundURL = normalizeBulkCoverBackgroundURLs(*req.BulkCoverBackgroundURL)
	}
	row.GeneratePromptContentName = normalizePromptContentName(row.GeneratePromptContentName, "Prompt content 1")
	row.GeneratePromptContentName2 = normalizePromptContentName(row.GeneratePromptContentName2, "Prompt content 2")
	row.GeneratePromptContentName3 = normalizePromptContentName(row.GeneratePromptContentName3, "Prompt content 3")
	if err := s.repo.Save(row); err != nil {
		return nil, err
	}
	return row, nil
}

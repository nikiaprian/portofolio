package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
)

type SiteWebImageProviderService interface {
	List() ([]dto.SiteWebImageProviderResponse, error)
	Update(req *dto.UpdateSiteWebImageProvidersRequest) ([]dto.SiteWebImageProviderResponse, error)
}

type siteWebImageProviderService struct {
	repo repositories.SiteWebImageProviderRepository
}

func NewSiteWebImageProviderService(repo repositories.SiteWebImageProviderRepository) SiteWebImageProviderService {
	return &siteWebImageProviderService{repo: repo}
}

var webImageProviderCredentialFields = map[string][]string{
	entity.WebImageProviderUnsplash:  {"access_key"},
	entity.WebImageProviderPixabay:   {"api_key"},
	entity.WebImageProviderOpenverse: {"client_id", "client_secret"},
	entity.WebImageProviderWordPress: {},
	entity.WebImageProviderWikimedia: {},
	entity.WebImageProviderPexels:    {"api_key"},
}

var webImageProviderOrder = entity.WebImageProviderAutoOrder

func (s *siteWebImageProviderService) List() ([]dto.SiteWebImageProviderResponse, error) {
	rows, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	byProvider := map[string]entity.SiteWebImageProvider{}
	for _, row := range rows {
		byProvider[row.Provider] = row
	}

	resp := make([]dto.SiteWebImageProviderResponse, 0, len(webImageProviderOrder))
	for _, provider := range webImageProviderOrder {
		row, ok := byProvider[provider]
		if !ok {
			row = entity.SiteWebImageProvider{Provider: provider, Enabled: false, CredentialsJSON: "{}"}
		}
		resp = append(resp, dto.SiteWebImageProviderResponse{
			Provider:    provider,
			Enabled:     row.Enabled,
			Credentials: normalizeWebImageCredentials(provider, decodeCredentials(row.CredentialsJSON)),
		})
	}
	return resp, nil
}

func (s *siteWebImageProviderService) Update(req *dto.UpdateSiteWebImageProvidersRequest) ([]dto.SiteWebImageProviderResponse, error) {
	payloads := map[string]dto.SiteWebImageProviderPayload{}
	for _, item := range req.Providers {
		provider := strings.ToLower(strings.TrimSpace(item.Provider))
		if _, ok := webImageProviderCredentialFields[provider]; !ok {
			return nil, fmt.Errorf("unsupported web image provider: %s", item.Provider)
		}
		item.Provider = provider
		payloads[provider] = item
	}

	rows := make([]entity.SiteWebImageProvider, 0, len(webImageProviderOrder))
	for _, provider := range webImageProviderOrder {
		item, ok := payloads[provider]
		if !ok {
			continue
		}
		credentials := normalizeWebImageCredentials(provider, item.Credentials)
		raw, err := json.Marshal(credentials)
		if err != nil {
			return nil, err
		}
		rows = append(rows, entity.SiteWebImageProvider{
			Provider:        provider,
			Enabled:         item.Enabled,
			CredentialsJSON: string(raw),
		})
	}
	if err := s.repo.ReplaceAll(rows); err != nil {
		return nil, err
	}
	return s.List()
}

func normalizeWebImageCredentials(provider string, credentials map[string]string) map[string]string {
	normalized := map[string]string{}
	for _, field := range webImageProviderCredentialFields[provider] {
		normalized[field] = strings.TrimSpace(credentials[field])
	}
	return normalized
}

func decodeCredentials(raw string) map[string]string {
	credentials := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return credentials
	}
	_ = json.Unmarshal([]byte(raw), &credentials)
	return credentials
}

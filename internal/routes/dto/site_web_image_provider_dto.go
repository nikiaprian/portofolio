package dto

type SiteWebImageProviderPayload struct {
	Provider    string            `json:"provider" binding:"required,max=40"`
	Enabled     bool              `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
}

type UpdateSiteWebImageProvidersRequest struct {
	Providers []SiteWebImageProviderPayload `json:"providers" binding:"required"`
}

type SiteWebImageProviderResponse struct {
	Provider    string            `json:"provider"`
	Enabled     bool              `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
}

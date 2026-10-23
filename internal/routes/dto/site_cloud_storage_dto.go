package dto

// UpdateSiteCloudStorageRequest is the body for PUT /admin/site-cloud-storage.
type UpdateSiteCloudStorageRequest struct {
	R2AccountID       string `json:"r2_account_id" binding:"max=128"`
	R2AccessKeyID     string `json:"r2_access_key_id" binding:"max=128"`
	R2SecretAccessKey string `json:"r2_secret_access_key" binding:"max=256"`
	R2Bucket          string `json:"r2_bucket" binding:"max=120"`
	R2PublicBaseURL   string `json:"r2_public_base_url" binding:"max=500"`
	R2Prefix          string `json:"r2_prefix" binding:"max=255"`
}

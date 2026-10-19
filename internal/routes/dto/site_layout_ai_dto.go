package dto

// GenerateSiteLayoutFragmentRequest is the body for POST /admin/site-layouts/generate-fragment.
type GenerateSiteLayoutFragmentRequest struct {
	Fragment      string   `json:"fragment" binding:"required"`
	CurrentPrompt string   `json:"current_prompt"`
	LayoutName    string   `json:"layout_name"`
	Provider      string   `json:"provider"`
	ImageURL      string   `json:"image_url"`
	ImageRefs     []string `json:"image_refs"`
}

// EnqueueLayoutGenerateRequest starts a background layout AI job.
type EnqueueLayoutGenerateRequest struct {
	LayoutID      uint     `json:"layout_id" binding:"required"`
	Mode          string   `json:"mode" binding:"required"`
	Fragment      string   `json:"fragment"`
	CurrentPrompt string   `json:"current_prompt"`
	LayoutName    string   `json:"layout_name"`
	Provider      string   `json:"provider"`
	ImageURL      string   `json:"image_url"`
	ImageRefs     []string `json:"image_refs"`
}

// ClassifySiteLayoutImageRefsRequest is the body for POST /admin/site-layouts/classify-image-refs.
type ClassifySiteLayoutImageRefsRequest struct {
	ImageRefs []string `json:"image_refs" binding:"required,min=1"`
	Provider  string   `json:"provider"`
}

// ApplySiteLayoutFragmentRequest is the body for POST /admin/site-layouts/apply-fragment.
type ApplySiteLayoutFragmentRequest struct {
	Fragment string `json:"fragment" binding:"required"`
	Content  string `json:"content"`
}

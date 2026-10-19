package dto

// CreateSiteLayoutRequest is the body for POST /admin/site-layouts.
type CreateSiteLayoutRequest struct {
	LayoutName    string `json:"layout_name" binding:"required,min=1,max=255"`
	Header        string `json:"header" binding:"max=65535"`
	Footer        string `json:"footer" binding:"max=65535"`
	HomeBody      string `json:"home_body" binding:"max=65535"`
	ArticleBody   string `json:"article_body" binding:"max=65535"`
	Sidebar       string `json:"sidebar" binding:"max=65535"`
	RSS           string `json:"rss" binding:"max=65535"`
	AuthorProfile string `json:"author_profile" binding:"max=65535"`
	Tag           string `json:"tag" binding:"max=65535"`
	Kategori      string `json:"kategori" binding:"max=65535"`
	SearchPage    string `json:"search_page" binding:"max=65535"`
	Misc          string `json:"misc" binding:"max=65535"`
	LatestPrompt  string `json:"latest_prompt" binding:"max=65535"`
	CurrentPrompt string `json:"current_prompt" binding:"max=65535"`
	ImageRef      string `json:"image_ref" binding:"max=65535"`
	CSSData       string `json:"css_data" binding:"max=65535"`
	JSData        string `json:"js_data" binding:"max=65535"`
}

// DuplicateSiteLayoutRequest is the body for POST /admin/site-layouts/:id/duplicate.
type DuplicateSiteLayoutRequest struct {
	LayoutName string `json:"layout_name" binding:"required,min=1,max=255"`
}

// UpdateSiteLayoutRequest is the body for PUT /admin/site-layouts/:id.
type UpdateSiteLayoutRequest struct {
	LayoutName    *string `json:"layout_name" binding:"omitempty,min=1,max=255"`
	Header        *string `json:"header" binding:"omitempty,max=65535"`
	Footer        *string `json:"footer" binding:"omitempty,max=65535"`
	HomeBody      *string `json:"home_body" binding:"omitempty,max=65535"`
	ArticleBody   *string `json:"article_body" binding:"omitempty,max=65535"`
	Sidebar       *string `json:"sidebar" binding:"omitempty,max=65535"`
	RSS           *string `json:"rss" binding:"omitempty,max=65535"`
	AuthorProfile *string `json:"author_profile" binding:"omitempty,max=65535"`
	Tag           *string `json:"tag" binding:"omitempty,max=65535"`
	Kategori      *string `json:"kategori" binding:"omitempty,max=65535"`
	SearchPage    *string `json:"search_page" binding:"omitempty,max=65535"`
	Misc          *string `json:"misc" binding:"omitempty,max=65535"`
	LatestPrompt  *string `json:"latest_prompt" binding:"omitempty,max=65535"`
	CurrentPrompt *string `json:"current_prompt" binding:"omitempty,max=65535"`
	ImageRef      *string `json:"image_ref" binding:"omitempty,max=65535"`
	CSSData       *string `json:"css_data" binding:"omitempty,max=65535"`
	JSData        *string `json:"js_data" binding:"omitempty,max=65535"`
}

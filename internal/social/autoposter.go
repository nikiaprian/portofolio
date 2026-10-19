package social

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

type AutoPoster struct {
	db     *gorm.DB
	client *http.Client
}

func NewAutoPoster(db *gorm.DB) *AutoPoster {
	if db == nil {
		return nil
	}
	return &AutoPoster{
		db: db,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

type publishPayload struct {
	Platform string `json:"platform"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet"`
	Message  string `json:"message"`
	Image    string `json:"image,omitempty"`
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type xTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type tiktokTokenResponse struct {
	Data struct {
		AccessToken string `json:"access_token"`
	} `json:"data"`
	AccessToken string `json:"access_token"`
}

type oauthPlatformDefaults struct {
	tokenURL   string
	publishURL string
	scope      string
}

var hardcodedOAuthDefaults = map[string]oauthPlatformDefaults{
	"facebook": {
		tokenURL:   "https://graph.facebook.com/v24.0/oauth/access_token",
		publishURL: "https://graph.facebook.com/v24.0/{account_id}/feed",
		scope:      "pages_manage_posts pages_read_engagement",
	},
	"instagram": {
		tokenURL:   "https://graph.facebook.com/v24.0/oauth/access_token",
		publishURL: "https://graph.facebook.com/v24.0/{account_id}/media",
		scope:      "instagram_basic instagram_content_publish pages_show_list",
	},
	"x": {
		tokenURL:   "https://api.twitter.com/2/oauth2/token",
		publishURL: "https://api.twitter.com/2/tweets",
		scope:      "tweet.write users.read offline.access",
	},
	"tiktok": {
		tokenURL:   "https://open.tiktokapis.com/v2/oauth/token/",
		publishURL: "https://open.tiktokapis.com/v2/post/publish/content/init/",
		scope:      "user.info.basic video.publish",
	},
}

func (a *AutoPoster) OnPublishedPost(ctx context.Context, p *entity.Post) error {
	if a == nil || a.db == nil || p == nil || p.ID == 0 {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(p.Status), "published") {
		return nil
	}

	var sc entity.SiteConnection
	if err := a.db.First(&sc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if !sc.AutoPostEnabled {
		return nil
	}

	postURL := a.postURL(p)
	msg := renderTemplate(sc.AutoPostTemplate, p, postURL)
	snippet := textSnippet(p.Content, 180)
	img := strings.TrimSpace(stringOrEmpty(p.Image))

	platforms := []struct {
		name         string
		enabled      bool
		clientID     string
		clientSecret string
		tokenURL     string
		publishURL   string
		scope        string
		accountID    string
	}{
		{
			name:         "facebook",
			enabled:      sc.FacebookEnabled,
			clientID:     sc.FacebookClientID,
			clientSecret: sc.FacebookClientSecret,
			tokenURL:     hardcodedOAuthDefaults["facebook"].tokenURL,
			publishURL:   hardcodedOAuthDefaults["facebook"].publishURL,
			scope:        hardcodedOAuthDefaults["facebook"].scope,
			accountID:    sc.FacebookAccountID,
		},
		{
			name:         "instagram",
			enabled:      sc.InstagramEnabled,
			clientID:     sc.InstagramClientID,
			clientSecret: sc.InstagramClientSecret,
			tokenURL:     hardcodedOAuthDefaults["instagram"].tokenURL,
			publishURL:   hardcodedOAuthDefaults["instagram"].publishURL,
			scope:        hardcodedOAuthDefaults["instagram"].scope,
			accountID:    sc.InstagramAccountID,
		},
		{
			name:         "x",
			enabled:      sc.XEnabled,
			clientID:     sc.XClientID,
			clientSecret: sc.XClientSecret,
			tokenURL:     hardcodedOAuthDefaults["x"].tokenURL,
			publishURL:   hardcodedOAuthDefaults["x"].publishURL,
			scope:        hardcodedOAuthDefaults["x"].scope,
			accountID:    sc.XAccountID,
		},
		{
			name:         "tiktok",
			enabled:      sc.TiktokEnabled,
			clientID:     sc.TiktokClientID,
			clientSecret: sc.TiktokClientSecret,
			tokenURL:     hardcodedOAuthDefaults["tiktok"].tokenURL,
			publishURL:   hardcodedOAuthDefaults["tiktok"].publishURL,
			scope:        hardcodedOAuthDefaults["tiktok"].scope,
			accountID:    sc.TiktokAccountID,
		},
	}

	var firstErr error
	for _, pl := range platforms {
		if !pl.enabled {
			continue
		}
		if !isPlatformSupportedForCurrentOAuth(pl.name) {
			log.Printf("autopost %s skipped post_id=%d: unsupported oauth flow (client_credentials)", pl.name, p.ID)
			continue
		}
		token, err := a.fetchOAuthToken(ctx, pl.tokenURL, pl.clientID, pl.clientSecret, pl.scope)
		if err != nil {
			log.Printf("autopost %s oauth failed post_id=%d: %v", pl.name, p.ID, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		body := publishPayload{
			Platform: pl.name,
			Title:    p.Title,
			URL:      postURL,
			Snippet:  snippet,
			Message:  msg,
			Image:    img,
		}
		if err := a.postPublishEndpoint(ctx, pl.publishURL, pl.accountID, token, body); err != nil {
			log.Printf("autopost %s failed post_id=%d: %v", pl.name, p.ID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (a *AutoPoster) postPublishEndpoint(ctx context.Context, endpoint, accountID, token string, payload publishPayload) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return errors.New("publish url is required")
	}
	endpoint = strings.ReplaceAll(endpoint, "{account_id}", url.PathEscape(strings.TrimSpace(accountID)))
	endpoint = strings.ReplaceAll(endpoint, "{{account_id}}", url.PathEscape(strings.TrimSpace(accountID)))

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(endpoint), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			return fmt.Errorf("webhook status: %s", resp.Status)
		}
		return fmt.Errorf("webhook status: %s body=%s", resp.Status, msg)
	}
	return nil
}

func (a *AutoPoster) fetchOAuthToken(ctx context.Context, tokenURL, clientID, clientSecret, scope string) (string, error) {
	tokenURL = strings.TrimSpace(tokenURL)
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	scope = strings.TrimSpace(scope)
	if tokenURL == "" {
		return "", errors.New("token url is required")
	}
	if clientID == "" || clientSecret == "" {
		return "", errors.New("client id and client secret are required")
	}
	provider := detectProviderByTokenURL(tokenURL)
	var req *http.Request
	var err error
	switch provider {
	case "x":
		req, err = buildXTokenRequest(ctx, tokenURL, clientID, clientSecret, scope)
	case "tiktok":
		req, err = buildTikTokTokenRequest(ctx, tokenURL, clientID, clientSecret, scope)
	default:
		req, err = buildDefaultTokenRequest(ctx, tokenURL, clientID, clientSecret, scope)
	}
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			return "", fmt.Errorf("token endpoint status: %s", resp.Status)
		}
		return "", fmt.Errorf("token endpoint status: %s body=%s", resp.Status, msg)
	}
	token, err := parseTokenFromResponse(provider, respBody)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("token response did not include access_token")
	}
	return token, nil
}

func (a *AutoPoster) postURL(p *entity.Post) string {
	base := strings.TrimRight(seo.DefaultLocalOrigin(), "/")
	if p == nil {
		return base + "/"
	}
	mode := repositories.ArticleSlugModeFromDB(a.db)
	catPath := ""
	if p.Category.ID != 0 {
		catPath = entity.CategoryWebPathPrefix(p.Category)
	} else if p.CategoryID != 0 {
		var cat entity.Category
		if err := a.db.Preload("Parent").First(&cat, p.CategoryID).Error; err == nil {
			catPath = entity.CategoryWebPathPrefix(cat)
		}
	}
	rel := entity.PublicArticleWebPath(mode, catPath, p.Slug)
	return strings.TrimRight(base, "/") + rel
}

func renderTemplate(tpl string, p *entity.Post, postURL string) string {
	tpl = strings.TrimSpace(tpl)
	if tpl == "" {
		tpl = "{{title}}\n{{url}}"
	}
	out := strings.ReplaceAll(tpl, "{{title}}", strings.TrimSpace(p.Title))
	out = strings.ReplaceAll(out, "{{url}}", postURL)
	out = strings.ReplaceAll(out, "{{snippet}}", textSnippet(p.Content, 220))
	return out
}

func textSnippet(html string, max int) string {
	s := strings.TrimSpace(html)
	if max <= 0 || s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "..."
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func detectProviderByTokenURL(tokenURL string) string {
	u := strings.ToLower(strings.TrimSpace(tokenURL))
	switch {
	case strings.Contains(u, "twitter.com"):
		return "x"
	case strings.Contains(u, "tiktok"):
		return "tiktok"
	default:
		return "default"
	}
}

func buildDefaultTokenRequest(ctx context.Context, tokenURL, clientID, clientSecret, scope string) (*http.Request, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	if scope != "" {
		form.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func buildXTokenRequest(ctx context.Context, tokenURL, clientID, clientSecret, scope string) (*http.Request, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_type", "confidential")
	form.Set("client_secret", clientSecret)
	if scope != "" {
		form.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func buildTikTokTokenRequest(ctx context.Context, tokenURL, clientID, clientSecret, scope string) (*http.Request, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_key", clientID)
	form.Set("client_secret", clientSecret)
	if scope != "" {
		form.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func parseTokenFromResponse(provider string, body []byte) (string, error) {
	switch provider {
	case "x":
		var tr xTokenResponse
		if err := json.Unmarshal(body, &tr); err != nil {
			return "", err
		}
		return strings.TrimSpace(tr.AccessToken), nil
	case "tiktok":
		var tr tiktokTokenResponse
		if err := json.Unmarshal(body, &tr); err != nil {
			return "", err
		}
		return strings.TrimSpace(firstNonEmptyString(tr.Data.AccessToken, tr.AccessToken)), nil
	default:
		var tr oauthTokenResponse
		if err := json.Unmarshal(body, &tr); err != nil {
			return "", err
		}
		return strings.TrimSpace(tr.AccessToken), nil
	}
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isPlatformSupportedForCurrentOAuth(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "x":
		return true
	default:
		return false
	}
}

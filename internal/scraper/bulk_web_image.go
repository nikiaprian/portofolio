package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
)

var (
	nonWordWebImageRe    = regexp.MustCompile(`[^a-z0-9]+`)
	webImageURLInHTMLRe  = regexp.MustCompile(`https?:\\?/\\?/[^"'\\\s<>]+?\.(?:jpe?g|png|webp)(?:\?[^"'\\\s<>]*)?`)
	webImageEscapedSlash = strings.NewReplacer(`\/`, `/`, `\u0026`, `&`)
)

type WebImageResult struct {
	StoredURL string
	SourceURL string
	Provider  string // web image provider id when from API/search (pexels, unsplash, …)
}

func newWebImageResult(storedURL, sourceURL, provider string) *WebImageResult {
	return &WebImageResult{
		StoredURL: strings.TrimSpace(storedURL),
		SourceURL: strings.TrimSpace(sourceURL),
		Provider:  strings.ToLower(strings.TrimSpace(provider)),
	}
}

type webImageProviderConfig struct {
	Provider    string
	Enabled     bool
	Credentials map[string]string
}

// BulkCoverImageFromWebSources tries public web sources in entity.WebImageProviderAutoOrder
// (Pexels, Unsplash, Google, Bing via SerpAPI).
// to obtain a relevant image for a generated post title.
// db may be nil; stored_image_* caps are applied when db is non-nil.
func BulkCoverImageFromWebSources(ctx context.Context, db *gorm.DB, store storage.ObjectStore, title, postSlug string) *string {
	if res := BulkCoverImageResultFromWebSourcesAttemptSource(ctx, db, store, title, postSlug, 0, ""); res != nil {
		return &res.StoredURL
	}
	return nil
}

// BulkCoverImageFromWebSourcesAttempt is like BulkCoverImageFromWebSources, but skips
// earlier usable candidates. This lets manual users click Search repeatedly for alternatives.
func BulkCoverImageFromWebSourcesAttempt(ctx context.Context, db *gorm.DB, store storage.ObjectStore, title, postSlug string, attempt int) *string {
	if res := BulkCoverImageResultFromWebSourcesAttemptSource(ctx, db, store, title, postSlug, attempt, ""); res != nil {
		return &res.StoredURL
	}
	return nil
}

// BulkCoverImageFromWebSourcesAttemptSource limits the search to one source when source is set.
func BulkCoverImageFromWebSourcesAttemptSource(ctx context.Context, db *gorm.DB, store storage.ObjectStore, title, postSlug string, attempt int, source string) *string {
	if res := BulkCoverImageResultFromWebSourcesAttemptSource(ctx, db, store, title, postSlug, attempt, source); res != nil {
		return &res.StoredURL
	}
	return nil
}

func BulkCoverImageResultFromWebSourcesAttemptSource(ctx context.Context, db *gorm.DB, store storage.ObjectStore, title, postSlug string, attempt int, source string) *WebImageResult {
	title = strings.TrimSpace(title)
	postSlug = strings.TrimSpace(postSlug)
	if title == "" || postSlug == "" {
		return nil
	}
	if attempt < 0 {
		attempt = 0
	}

	client := &http.Client{Timeout: 24 * time.Second}
	storeMaxEdge, storeWebPQ := repositories.PostImageStoreLimitsFromDB(db)
	skip := attempt
	source = strings.ToLower(strings.TrimSpace(source))
	providers := loadWebImageProviderConfigs(db)

	tryOrder := entity.WebImageProviderAutoOrder
	if source != "" && source != "auto" {
		tryOrder = []string{source}
	}
	for _, provider := range tryOrder {
		if !webImageProviderAllowed(source, providers, provider) {
			continue
		}
		img, exhausted := tryWebImageProviderAPI(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, &skip, source, provider, providers)
		if img != nil {
			return img
		}
		if exhausted {
			return nil
		}
	}

	// Last fallback: scrape public search pages when API access is unavailable.
	searchSlug := slugForWebSearch(title)
	for _, page := range webImageSearchPageURLs(searchSlug) {
		if source != "" && source != "auto" && source != page.source {
			continue
		}
		if !webImageProviderAllowed(source, providers, page.source) {
			continue
		}
		if img := trySearchPageImageResultAt(ctx, client, store, page.url, page.source, postSlug, storeMaxEdge, storeWebPQ, &skip); img != nil {
			return img
		}
	}
	return nil
}

// tryWebImageProviderAPI attempts one provider's official API (or equivalent). exhausted is true
// when source targets this provider and no further providers should be tried.
func tryWebImageProviderAPI(
	ctx context.Context,
	client *http.Client,
	store storage.ObjectStore,
	title, postSlug string,
	storeMaxEdge, storeWebPQ int,
	skip *int,
	source, provider string,
	providers map[string]webImageProviderConfig,
) (img *WebImageResult, exhausted bool) {
	switch provider {
	case entity.WebImageProviderPexels:
		credentials := providers[entity.WebImageProviderPexels].Credentials
		pexelsKey := webImageProviderCredentialValue(credentials, "api_key", "PEXELS_API_KEY")
		if pexelsKey != "" {
			if img := tryPexelsAPIImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip, credentials); img != nil {
				return img, false
			}
		} else if source == entity.WebImageProviderPexels {
			return nil, true
		}
	case entity.WebImageProviderUnsplash:
		credentials := providers[entity.WebImageProviderUnsplash].Credentials
		unsplashKey := webImageProviderCredentialValue(credentials, "access_key", "UNSPLASH_ACCESS_KEY")
		if unsplashKey != "" {
			if img := tryUnsplashAPIImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip, credentials); img != nil {
				return img, false
			}
			if img := tryUnsplashSourceResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ); img != nil {
				if skip == nil || *skip <= 0 {
					return img, false
				}
				*skip = *skip - 1
			}
		} else if source == entity.WebImageProviderUnsplash {
			return nil, true
		}
	case entity.WebImageProviderGoogle, entity.WebImageProviderBing:
		credentials := providers[entity.WebImageProviderSerpAPI].Credentials
		serpAPIKey := webImageProviderCredentialValue(credentials, "api_key", "SERPAPI_API_KEY")
		if serpAPIKey != "" {
			if img := trySerpAPIImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip, credentials, provider); img != nil {
				return img, false
			}
		} else if source == provider {
			return nil, true
		}
	case entity.WebImageProviderBingNet:
		if img := tryBingNetResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip); img != nil {
			return img, false
		} else if source == entity.WebImageProviderBingNet {
			return nil, true
		}
	}
	return nil, false
}

func webImageSearchPageURLs(searchSlug string) []struct {
	source string
	url    string
} {
	return []struct {
		source string
		url    string
	}{
		{entity.WebImageProviderPexels, fmt.Sprintf("https://www.pexels.com/search/%s/", searchSlug)},
		{entity.WebImageProviderUnsplash, fmt.Sprintf("https://unsplash.com/s/photos/%s", searchSlug)},
	}
}

func loadWebImageProviderConfigs(db *gorm.DB) map[string]webImageProviderConfig {
	configs := map[string]webImageProviderConfig{}
	for _, row := range entity.DefaultSiteWebImageProviders() {
		configs[row.Provider] = webImageProviderConfig{
			Provider:    row.Provider,
			Enabled:     row.Enabled,
			Credentials: map[string]string{},
		}
	}
	if db == nil {
		return configs
	}
	var rows []entity.SiteWebImageProvider
	if err := db.Find(&rows).Error; err != nil {
		return configs
	}
	for _, row := range rows {
		provider := strings.ToLower(strings.TrimSpace(row.Provider))
		if provider == "" {
			continue
		}
		cfg := configs[provider]
		cfg.Provider = provider
		cfg.Enabled = row.Enabled
		cfg.Credentials = decodeWebImageCredentials(row.CredentialsJSON)
		configs[provider] = cfg
	}
	return configs
}

func decodeWebImageCredentials(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func serpAPICredentials(configs map[string]webImageProviderConfig) map[string]string {
	cfg, ok := configs[entity.WebImageProviderSerpAPI]
	if !ok {
		return map[string]string{}
	}
	return cfg.Credentials
}

func serpAPIKeyConfigured(configs map[string]webImageProviderConfig) bool {
	return webImageProviderCredentialValue(serpAPICredentials(configs), "api_key", "SERPAPI_API_KEY") != ""
}

func webImageProviderConfigKey(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == entity.WebImageProviderGoogle || provider == entity.WebImageProviderBing {
		return entity.WebImageProviderSerpAPI
	}
	return provider
}

func webImageProviderAllowed(source string, configs map[string]webImageProviderConfig, provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return false
	}
	if source != "" && source != "auto" && source != provider {
		return false
	}
	if provider == entity.WebImageProviderGoogle || provider == entity.WebImageProviderBing {
		if !serpAPIKeyConfigured(configs) {
			return false
		}
		// Explicit Google/Bing selection works whenever a SerpAPI key is configured.
		if source == provider {
			return true
		}
		cfg, ok := configs[entity.WebImageProviderSerpAPI]
		if ok && !cfg.Enabled {
			return false
		}
		return true
	}
	cfgKey := webImageProviderConfigKey(provider)
	cfg, ok := configs[cfgKey]
	if !ok {
		return true
	}
	return cfg.Enabled
}

func webImageProviderCredentialValue(credentials map[string]string, field, fallbackEnv string) string {
	value := strings.TrimSpace(credentials[field])
	if value == "" && fallbackEnv != "" {
		value = strings.TrimSpace(os.Getenv(fallbackEnv))
	}
	return value
}

type unsplashSearchResponse struct {
	Results []struct {
		Links struct {
			HTML string `json:"html"`
		} `json:"links"`
		URLs struct {
			Raw     string `json:"raw"`
			Full    string `json:"full"`
			Regular string `json:"regular"`
		} `json:"urls"`
	} `json:"results"`
}

func tryUnsplashAPIImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int, credentials map[string]string) *WebImageResult {
	key := strings.TrimSpace(credentials["access_key"])
	if key == "" {
		key = strings.TrimSpace(os.Getenv("UNSPLASH_ACCESS_KEY"))
	}
	if key == "" {
		return nil
	}
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://api.unsplash.com/search/photos?per_page=20&orientation=landscape&query=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Client-ID "+key)
	req.Header.Set("Accept-Version", "v1")
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}
	var payload unsplashSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil
	}
	for _, row := range payload.Results {
		for _, rawURL := range []string{row.URLs.Regular, row.URLs.Full, row.URLs.Raw} {
			imgURL := normalizeCandidateImageURL(rawURL)
			if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
				continue
			}
			if local, err := downloadRemotePostImage(ctx, client, store, imgURL, row.Links.HTML, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
				if skip != nil && *skip > 0 {
					*skip = *skip - 1
					continue
				}
				return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderUnsplash)
			}
		}
	}
	return nil
}

type pexelsSearchResponse struct {
	Photos []struct {
		URL string `json:"url"`
		Src struct {
			Large2x  string `json:"large2x"`
			Large    string `json:"large"`
			Medium   string `json:"medium"`
			Original string `json:"original"`
		} `json:"src"`
	} `json:"photos"`
}

func tryPexelsAPIImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int, credentials map[string]string) *WebImageResult {
	key := webImageProviderCredentialValue(credentials, "api_key", "PEXELS_API_KEY")
	if key == "" {
		return nil
	}
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://api.pexels.com/v1/search?" + url.Values{
		"query":    {q},
		"per_page": {"20"},
		"locale":   {"id-ID"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", key)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bebii-blog-web-image/1.0; +https://bebii.blog)")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}
	var payload pexelsSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil
	}
	for _, photo := range payload.Photos {
		pageURL := strings.TrimSpace(photo.URL)
		for _, rawURL := range []string{photo.Src.Large2x, photo.Src.Large, photo.Src.Medium, photo.Src.Original} {
			imgURL := normalizeCandidateImageURL(rawURL)
			if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
				continue
			}
			if local, err := downloadRemotePostImage(ctx, client, store, imgURL, pageURL, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
				if skip != nil && *skip > 0 {
					*skip = *skip - 1
					continue
				}
				return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderPexels)
			}
		}
	}
	return nil
}

type serpAPIImagesSearchResponse struct {
	ImagesResults []struct {
		Original  string `json:"original"`
		Thumbnail string `json:"thumbnail"`
		Link      string `json:"link"`
		Source    string `json:"source"`
	} `json:"images_results"`
	Error string `json:"error"`
}

func serpAPISearchEngine(provider string) string {
	if strings.EqualFold(strings.TrimSpace(provider), entity.WebImageProviderBing) {
		return "bing_images"
	}
	return "google_images"
}

func serpAPIPageReferer(link, source string) string {
	link = strings.TrimSpace(link)
	if link != "" {
		if u, err := url.Parse(link); err == nil && u.Scheme != "" && u.Host != "" {
			return link
		}
	}
	source = strings.TrimSpace(source)
	if source != "" {
		if u, err := url.Parse(source); err == nil && u.Scheme != "" && u.Host != "" {
			return source
		}
	}
	return ""
}

func serpAPIImageCandidateURLs(row struct {
	Original  string `json:"original"`
	Thumbnail string `json:"thumbnail"`
	Link      string `json:"link"`
	Source    string `json:"source"`
}) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(raw string) {
		imgURL := normalizeCandidateImageURL(raw)
		if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
			return
		}
		if _, dup := seen[imgURL]; dup {
			return
		}
		seen[imgURL] = struct{}{}
		out = append(out, imgURL)
	}
	add(row.Original)
	if u, err := url.Parse(strings.TrimSpace(row.Link)); err == nil {
		add(u.Query().Get("mediaurl"))
	}
	add(row.Thumbnail)
	return out
}

func trySerpAPIImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int, credentials map[string]string, provider string) *WebImageResult {
	key := webImageProviderCredentialValue(credentials, "api_key", "SERPAPI_API_KEY")
	if key == "" {
		return nil
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != entity.WebImageProviderGoogle && provider != entity.WebImageProviderBing {
		provider = entity.WebImageProviderGoogle
	}
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://serpapi.com/search.json?" + url.Values{
		"engine":  {serpAPISearchEngine(provider)},
		"q":       {q},
		"api_key": {key},
		"ijn":     {"0"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bebii-blog-web-image/1.0; +https://bebii.blog)")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		ScrapeLogf("warn", "", u, "[web-image] serpapi HTTP %d body=%s", res.StatusCode, strings.TrimSpace(string(rawBody)))
		return nil
	}
	var payload serpAPIImagesSearchResponse
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil
	}
	if strings.TrimSpace(payload.Error) != "" {
		ScrapeLogf("warn", "", u, "[web-image] serpapi error: %s", strings.TrimSpace(payload.Error))
		return nil
	}
	if len(payload.ImagesResults) == 0 {
		ScrapeLogf("warn", "", u, "[web-image] serpapi returned no images for q=%q engine=%s", q, serpAPISearchEngine(provider))
		return nil
	}
	for _, row := range payload.ImagesResults {
		pageURL := serpAPIPageReferer(row.Link, row.Source)
		for _, imgURL := range serpAPIImageCandidateURLs(row) {
			if local, err := downloadRemotePostImage(ctx, client, store, imgURL, pageURL, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
				if skip != nil && *skip > 0 {
					*skip = *skip - 1
					continue
				}
				return newWebImageResult(strings.TrimSpace(*local), imgURL, provider)
			}
		}
	}
	return nil
}

func tryUnsplashSource(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int) *string {
	if res := tryUnsplashSourceResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ); res != nil {
		return &res.StoredURL
	}
	return nil
}

func tryUnsplashSourceResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int) *WebImageResult {
	q := url.QueryEscape(strings.TrimSpace(title))
	if q == "" {
		return nil
	}
	u := "https://source.unsplash.com/1600x900/?" + q
	if local, err := downloadRemotePostImage(ctx, client, store, u, "https://unsplash.com/", postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
		return newWebImageResult(strings.TrimSpace(*local), u, entity.WebImageProviderUnsplash)
	}
	return nil
}

func tryBingNetResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int) *WebImageResult {
	q := url.QueryEscape(strings.TrimSpace(title))
	if q == "" {
		return nil
	}

	// 1. If skip == 0 (first attempt), try direct Bing.net thumbnail endpoint
	if skip == nil || *skip == 0 {
		u := "https://tes1.mm.bing.net/th?q=" + q
		if local, err := downloadRemotePostImage(ctx, client, store, u, "https://www.bing.com/", postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
			return newWebImageResult(strings.TrimSpace(*local), u, entity.WebImageProviderBingNet)
		}
	}

	// 2. If direct thumbnail failed or skip > 0 (alternative search attempt), scrape Bing Images search page
	searchURL := "https://www.bing.com/images/search?q=" + q
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")

	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}

	rawBody, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(rawBody))
	if err != nil {
		return nil
	}

	type bingMedia struct {
		Murl string `json:"murl"`
		Turl string `json:"turl"`
	}

	seen := map[string]struct{}{}
	var candidates []string

	// Extract murl/turl from a.iusc data attribute
	doc.Find("a.iusc").Each(func(_ int, s *goquery.Selection) {
		mAttr := s.AttrOr("m", "")
		if mAttr != "" {
			var mData bingMedia
			if err := json.Unmarshal([]byte(mAttr), &mData); err == nil {
				if mData.Murl != "" {
					if _, ok := seen[mData.Murl]; !ok {
						seen[mData.Murl] = struct{}{}
						candidates = append(candidates, mData.Murl)
					}
				}
				if mData.Turl != "" {
					if _, ok := seen[mData.Turl]; !ok {
						seen[mData.Turl] = struct{}{}
						candidates = append(candidates, mData.Turl)
					}
				}
			}
		}
	})

	// Extract img.mimg src/data-src
	doc.Find("img.mimg, img.rms_img").Each(func(_ int, s *goquery.Selection) {
		for _, attr := range []string{"src", "data-src"} {
			v := strings.TrimSpace(s.AttrOr(attr, ""))
			if strings.HasPrefix(v, "http") {
				if _, ok := seen[v]; !ok {
					seen[v] = struct{}{}
					candidates = append(candidates, v)
				}
			}
		}
	})

	if len(candidates) == 0 {
		return nil
	}

	for _, imgURL := range candidates {
		if local, err := downloadRemotePostImage(ctx, client, store, imgURL, searchURL, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
			if skip != nil && *skip > 0 {
				*skip = *skip - 1
				continue
			}
			return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderBingNet)
		}
	}

	return nil
}

func trySearchPageFirstImage(ctx context.Context, client *http.Client, store storage.ObjectStore, pageURL, postSlug string, storeMaxEdge, storeWebPQ int) *string {
	skip := 0
	return trySearchPageImageAt(ctx, client, store, pageURL, postSlug, storeMaxEdge, storeWebPQ, &skip)
}

func trySearchPageImageAt(ctx context.Context, client *http.Client, store storage.ObjectStore, pageURL, postSlug string, storeMaxEdge, storeWebPQ int, skip *int) *string {
	if res := trySearchPageImageResultAt(ctx, client, store, pageURL, "", postSlug, storeMaxEdge, storeWebPQ, skip); res != nil {
		return &res.StoredURL
	}
	return nil
}

func trySearchPageImageResultAt(ctx context.Context, client *http.Client, store storage.ObjectStore, pageURL, provider, postSlug string, storeMaxEdge, storeWebPQ int, skip *int) *WebImageResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bebii-blog-web-image/1.0; +https://bebii.blog)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}

	rawBody, err := io.ReadAll(io.LimitReader(res.Body, maxFeedBodyBytes))
	if err != nil {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(rawBody)))
	if err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	cands := make([]string, 0, 8)
	doc.Find("img,source,meta[property='og:image'],meta[name='twitter:image']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, attr := range []string{"content", "src", "data-src", "data-lazy-src", "srcset"} {
			v, ok := s.Attr(attr)
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			if attr == "srcset" {
				v = firstURLFromSrcset(v)
			}
			v = resolveAgainstArticleURL(pageURL, normalizeCandidateImageURL(v))
			if v == "" || !strings.HasPrefix(strings.ToLower(v), "http") {
				continue
			}
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			cands = append(cands, v)
			if len(cands) >= 8 {
				return false
			}
		}
		return true
	})
	for _, m := range webImageURLInHTMLRe.FindAllString(string(rawBody), 24) {
		v := webImageEscapedSlash.Replace(strings.TrimSpace(m))
		v = resolveAgainstArticleURL(pageURL, normalizeCandidateImageURL(v))
		if v == "" || !strings.HasPrefix(strings.ToLower(v), "http") {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		cands = append(cands, v)
		if len(cands) >= 16 {
			break
		}
	}

	for _, imgURL := range cands {
		if local, err := downloadRemotePostImage(ctx, client, store, imgURL, pageURL, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
			if skip != nil && *skip > 0 {
				*skip = *skip - 1
				continue
			}
			return newWebImageResult(strings.TrimSpace(*local), imgURL, provider)
		}
	}
	return nil
}

func slugForWebSearch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = nonWordWebImageRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 90 {
		s = strings.Trim(s[:90], "-")
	}
	return s
}

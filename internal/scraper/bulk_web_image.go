package scraper

import (
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

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/storage"
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
// (Pexels, Unsplash, Pixabay, Openverse, Wikimedia, WordPress).
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
	case entity.WebImageProviderPixabay:
		credentials := providers[entity.WebImageProviderPixabay].Credentials
		pixabayKey := webImageProviderCredentialValue(credentials, "api_key", "PIXABAY_API_KEY")
		if pixabayKey != "" {
			if img := tryPixabayAPIImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip, credentials); img != nil {
				return img, false
			}
		} else if source == entity.WebImageProviderPixabay {
			return nil, true
		}
	case entity.WebImageProviderOpenverse:
		if img := tryOpenverseImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip); img != nil {
			return img, false
		}
		if source == entity.WebImageProviderOpenverse {
			return nil, true
		}
	case entity.WebImageProviderWikimedia:
		if img := tryWikimediaImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip); img != nil {
			return img, false
		}
		if source == entity.WebImageProviderWikimedia {
			return nil, true
		}
	case entity.WebImageProviderWordPress:
		if img := tryWordPressRESTImageResult(ctx, client, store, title, postSlug, storeMaxEdge, storeWebPQ, skip); img != nil {
			return img, false
		}
		if source == entity.WebImageProviderWordPress {
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
		{entity.WebImageProviderPixabay, fmt.Sprintf("https://pixabay.com/images/search/%s/", searchSlug)},
		{entity.WebImageProviderWordPress, fmt.Sprintf("https://wordpress.com/tag/%s/", searchSlug)},
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

func webImageProviderAllowed(source string, configs map[string]webImageProviderConfig, provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return false
	}
	if source != "" && source != "auto" && source != provider {
		return false
	}
	cfg, ok := configs[provider]
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

type openverseImageSearchResponse struct {
	Results []struct {
		URL       string `json:"url"`
		Thumbnail string `json:"thumbnail"`
		Foreign   string `json:"foreign_landing_url"`
	} `json:"results"`
}

func tryOpenverseImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int) *WebImageResult {
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://api.openverse.engineering/v1/images/?page_size=20&q=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-web-image/1.0; +https://tirtayasa.id)")
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}
	var payload openverseImageSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, row := range payload.Results {
		for _, rawURL := range []string{row.Thumbnail, row.URL} {
			imgURL := normalizeCandidateImageURL(rawURL)
			if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
				continue
			}
			if _, dup := seen[imgURL]; dup {
				continue
			}
			seen[imgURL] = struct{}{}
			if local, err := downloadRemotePostImage(ctx, client, store, imgURL, row.Foreign, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
				if skip != nil && *skip > 0 {
					*skip = *skip - 1
					continue
				}
				return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderOpenverse)
			}
		}
	}
	return nil
}

type wikimediaImageSearchResponse struct {
	Query struct {
		Pages map[string]struct {
			ImageInfo []struct {
				URL      string `json:"url"`
				ThumbURL string `json:"thumburl"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

func tryWikimediaImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int) *WebImageResult {
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://commons.wikimedia.org/w/api.php?action=query&format=json&generator=search&gsrnamespace=6&gsrlimit=20&prop=imageinfo&iiprop=url&iiurlwidth=1600&gsrsearch=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-web-image/1.0; +https://tirtayasa.id)")
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}
	var payload wikimediaImageSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil
	}
	for _, page := range payload.Query.Pages {
		for _, info := range page.ImageInfo {
			for _, rawURL := range []string{info.ThumbURL, info.URL} {
				imgURL := normalizeCandidateImageURL(rawURL)
				if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
					continue
				}
				if local, err := downloadRemotePostImage(ctx, client, store, imgURL, "https://commons.wikimedia.org/", postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
					if skip != nil && *skip > 0 {
						*skip = *skip - 1
						continue
					}
					return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderWikimedia)
				}
			}
		}
	}
	return nil
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

type pixabaySearchResponse struct {
	Hits []struct {
		PageURL      string `json:"pageURL"`
		LargeURL     string `json:"largeImageURL"`
		WebformatURL string `json:"webformatURL"`
		PreviewURL   string `json:"previewURL"`
	} `json:"hits"`
}

func tryPixabayAPIImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int, credentials map[string]string) *WebImageResult {
	key := strings.TrimSpace(credentials["api_key"])
	if key == "" {
		key = strings.TrimSpace(os.Getenv("PIXABAY_API_KEY"))
	}
	if key == "" {
		return nil
	}
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://pixabay.com/api/?image_type=photo&orientation=horizontal&per_page=20&safesearch=true&key=" + url.QueryEscape(key) + "&q=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-web-image/1.0; +https://tirtayasa.id)")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}
	var payload pixabaySearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil
	}
	for _, row := range payload.Hits {
		for _, rawURL := range []string{row.LargeURL, row.WebformatURL, row.PreviewURL} {
			imgURL := normalizeCandidateImageURL(rawURL)
			if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
				continue
			}
			if local, err := downloadRemotePostImage(ctx, client, store, imgURL, row.PageURL, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
				if skip != nil && *skip > 0 {
					*skip = *skip - 1
					continue
				}
				return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderPixabay)
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-web-image/1.0; +https://tirtayasa.id)")
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

type wordpressRESTSearchResponse []struct {
	URL           string `json:"URL"`
	Featured      string `json:"featured_image"`
	PostThumbnail struct {
		URL string `json:"URL"`
	} `json:"post_thumbnail"`
	Attachments map[string]struct {
		URL string `json:"URL"`
	} `json:"attachments"`
}

func tryWordPressRESTImageResult(ctx context.Context, client *http.Client, store storage.ObjectStore, title, postSlug string, storeMaxEdge, storeWebPQ int, skip *int) *WebImageResult {
	q := strings.TrimSpace(title)
	if q == "" {
		return nil
	}
	u := "https://public-api.wordpress.com/rest/v1.1/read/search?q=" + url.QueryEscape(q) + "&number=20"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-web-image/1.0; +https://tirtayasa.id)")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil
	}
	var payload wordpressRESTSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil
	}
	for _, row := range payload {
		candidates := []string{row.Featured, row.PostThumbnail.URL}
		for _, att := range row.Attachments {
			candidates = append(candidates, att.URL)
		}
		for _, rawURL := range candidates {
			imgURL := normalizeCandidateImageURL(rawURL)
			if imgURL == "" || !strings.HasPrefix(strings.ToLower(imgURL), "http") {
				continue
			}
			if local, err := downloadRemotePostImage(ctx, client, store, imgURL, row.URL, postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
				if skip != nil && *skip > 0 {
					*skip = *skip - 1
					continue
				}
				return newWebImageResult(strings.TrimSpace(*local), imgURL, entity.WebImageProviderWordPress)
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-web-image/1.0; +https://tirtayasa.id)")
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

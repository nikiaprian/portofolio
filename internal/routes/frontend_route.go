package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/internal/cache"
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/autohref"
	"github.com/nikiaprian/tirtayasa/internal/helper/imagecaption"
	"github.com/nikiaprian/tirtayasa/internal/helper/posthtml"
	"github.com/nikiaprian/tirtayasa/internal/helper/publicshelf"
	"github.com/nikiaprian/tirtayasa/internal/helper/publishedpost"
	"github.com/nikiaprian/tirtayasa/internal/helper/scraperguard"
	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/middleware"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/handler"
)

var htmlTagStripper = regexp.MustCompile(`<[^>]*>`)
var slugifyNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
var firstImgSrcRe = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
var htmlAttrURLRe = regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*["'](/[^"']*)["']`)
var clientPublicTimezone = "Asia/Jakarta"

// clientAdSlotHTML renders ad slot markup as stored (snippet AdSense utuh, termasuk push script).
func clientAdSlotHTML(s string) template.HTML {
	return template.HTML(s)
}

func clientSiteScriptHTML(s string) template.HTML {
	return template.HTML(s)
}

// BLANK_IMG is a 1x1 transparent GIF data URL used as a safe fallback.
// Keep this in sync with the frontend constant in `public/static/js/app.js`.
const BLANK_IMG = "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw=="

// adminAssetQueryVersion cache-busts admin static URLs from file mtimes so deploys
// pick up new JS/CSS without relying on process restart (see staticCacheHeaders immutable policy).
func adminAssetQueryVersion() string {
	paths := []string{
		"public/static/css/adminlte.min.css",
		"public/static/css/admin/crud.min.css",
		"public/static/js/adminlte.min.js",
		"public/static/js/api.min.js",
		"public/static/js/admin/crud.min.js",
		"public/static/js/admin/name_slug_crud.js",
		"public/static/js/admin/category.min.js",
		"public/static/js/admin/tag.min.js",
		"public/static/js/admin/post.min.js",
		"public/static/js/admin/page.min.js",
		"public/static/js/admin/page_form.min.js",
		"public/static/js/admin/user.min.js",
		"public/static/js/admin/profile.min.js",
		"public/static/js/admin/bulk_post.min.js",
		"public/static/js/admin/bulk_post.js",
		"public/static/js/admin/post_form.min.js",
		"public/static/js/admin/ai_setting.min.js",
		"public/static/js/admin/backup.min.js",
		"public/static/js/admin/site_setting.js",
		"public/static/js/admin/site_setting.min.js",
		"public/static/js/admin/site_connections.min.js",
		"public/static/js/admin/scraper_setting.min.js",
		"public/static/js/admin/scraper_logs.min.js",
		"public/static/js/admin/login_logs.min.js",
		"public/static/js/admin/bulk_post_logs.min.js",
		"public/static/js/admin/manual_post_ai_logs.min.js",
		"public/static/js/admin/log_timestamp.min.js",
		"public/static/js/admin/tinymce.min.js",
	}
	var max int64
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if t := fi.ModTime().Unix(); t > max {
			max = t
		}
	}
	if max == 0 {
		return strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}
	return strconv.FormatInt(max, 10)
}

// cache-bust client static assets (css/js) on each deploy/restart
var clientStaticVersion = strconv.FormatInt(time.Now().Unix(), 10)

func buildRandomRecommendationRows(popularRows, latestRows []gin.H, limit int) []gin.H {
	if limit <= 0 {
		return nil
	}
	seen := map[string]struct{}{}
	candidates := make([]gin.H, 0, len(popularRows)+len(latestRows))
	appendUnique := func(rows []gin.H) {
		for _, r := range rows {
			href := strings.TrimSpace(fmt.Sprint(r["href"]))
			if href == "" {
				continue
			}
			if _, ok := seen[href]; ok {
				continue
			}
			seen[href] = struct{}{}
			candidates = append(candidates, r)
		}
	}
	appendUnique(popularRows)
	appendUnique(latestRows)
	if len(candidates) == 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func textPreview(s string, max int) string {
	t := strings.TrimSpace(htmlTagStripper.ReplaceAllString(s, " "))
	t = strings.Join(strings.Fields(t), " ")
	if max <= 0 || len(t) <= max {
		return t
	}
	return t[:max] + "…"
}

func postExcerptForCard(p entity.Post, max int) string {
	if p.MetaDesc != nil {
		md := strings.TrimSpace(*p.MetaDesc)
		if md != "" {
			return textPreview(md, max)
		}
	}
	return textPreview(p.Content, max)
}

func authorDisplayName(u entity.User) string {
	n := strings.TrimSpace(u.Name)
	if n != "" {
		return n
	}
	return strings.TrimSpace(u.Username)
}

func formatPostDateCalendar(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	v := *t
	if loc, err := time.LoadLocation(clientPublicTimezone); err == nil {
		v = v.In(loc)
	}
	m := int(v.Month())
	if m < 1 || m > 12 {
		m = 1
	}
	return fmt.Sprintf("%d %s %d", v.Day(), indonesianMonths[m], v.Year())
}

func postIDsFromPosts(posts []entity.Post) []uint {
	ids := make([]uint, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	return ids
}

func postToNeoMagRow(siteSetting entity.SiteSetting, p entity.Post) gin.H {
	cn := strings.TrimSpace(p.Category.Name)
	catHref := ""
	if p.Category.ID != 0 {
		catHref = entity.CategoryPublicWebPath(p.Category)
	}
	vlab := formatViewCountID(p.View)
	return gin.H{
		"title":        strings.TrimSpace(p.Title),
		"href":         clientPostPublicPath(siteSetting, p),
		"thumb":        postThumbURL(siteSetting, p.Image, p.Content),
		"rel_time":     formatRelativeTimeID(postDisplayTimeRef(p)),
		"date_label":   formatPostDateCalendar(postDisplayTimeRef(p)),
		"excerpt":      postExcerptForCard(p, 200),
		"author_name":  authorDisplayName(p.Author),
		"cat_name":     cn,
		"has_cat_slug": catHref != "" && catHref != "/",
		"cat_href":     catHref,
		"view_label":   vlab,
		"has_view":     vlab != "",
	}
}

func postsToNeoMagRows(siteSetting entity.SiteSetting, posts []entity.Post) []gin.H {
	out := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		out = append(out, postToNeoMagRow(siteSetting, p))
	}
	return out
}

// categoriesFromDistinctRecentPosts returns up to maxCats categories taken from recent published posts
// (distinct by category_id, ordered by earliest appearance in the post list = recency-first).
func categoriesFromDistinctRecentPosts(db *gorm.DB, scanLimit int, maxCats int) []entity.Category {
	if scanLimit <= 0 {
		scanLimit = 64
	}
	if maxCats <= 0 {
		maxCats = 3
	}
	var posts []entity.Post
	q := db.Preload("Category").Preload("Category.Parent").
		Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).
		Order("COALESCE(updated_at, created_at) DESC")
	if scanLimit > 0 {
		q = q.Limit(scanLimit)
	}
	if err := q.Find(&posts).Error; err != nil {
		return nil
	}
	seen := make(map[uint]struct{})
	out := make([]entity.Category, 0, maxCats)
	for _, p := range posts {
		c := p.Category
		if c.ID == 0 {
			continue
		}
		if _, ok := seen[c.ID]; ok {
			continue
		}
		name := strings.TrimSpace(c.Name)
		slug := strings.TrimSpace(c.Slug)
		if name == "" || slug == "" {
			continue
		}
		seen[c.ID] = struct{}{}
		out = append(out, c)
		if len(out) >= maxCats {
			break
		}
	}
	return out
}

func buildNeoFeaturedCategoryTabs(db *gorm.DB, siteSetting entity.SiteSetting, maxTabs int, postsPerCat int) []gin.H {
	if maxTabs <= 0 {
		maxTabs = 3
	}
	if postsPerCat < 2 {
		postsPerCat = 2
	}
	seenCat := make(map[uint]struct{})
	tabs := make([]gin.H, 0, maxTabs)

	tryAppendTab := func(catID uint, catName, tabHref string) {
		if len(tabs) >= maxTabs || catID == 0 {
			return
		}
		if _, ok := seenCat[catID]; ok {
			return
		}
		name := strings.TrimSpace(catName)
		tabHref = strings.TrimSpace(tabHref)
		if tabHref == "" || tabHref == "/" {
			return
		}
		var tabPosts []entity.Post
		if err := db.Preload("Category").Preload("Category.Parent").Preload("Author").
			Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).
			Where("category_id = ?", catID).
			Order("COALESCE(updated_at, created_at) DESC").
			Limit(postsPerCat).
			Find(&tabPosts).Error; err != nil || len(tabPosts) == 0 {
			return
		}
		if viewCounts, err := repositories.NewPostViewRepository(db).CountForPosts(postIDsFromPosts(tabPosts)); err == nil {
			for i := range tabPosts {
				tabPosts[i].View = viewCounts[tabPosts[i].ID]
			}
		}
		seenCat[catID] = struct{}{}
		side := tabPosts[1:]
		if len(side) > 4 {
			side = side[:4]
		}
		tabs = append(tabs, gin.H{
			"name":       name,
			"href":       tabHref,
			"featured":   postToNeoMagRow(siteSetting, tabPosts[0]),
			"side_posts": postsToNeoMagRows(siteSetting, side),
		})
	}

	for _, cat := range loadNavCategories(db, siteSetting.HomepageLayout) {
		if len(tabs) >= maxTabs {
			break
		}
		rootEnt := entity.Category{ID: cat.ID, Slug: cat.Slug}
		tryAppendTab(cat.ID, cat.Name, entity.CategoryPublicWebPath(rootEnt))
		for _, ch := range cat.Children {
			if len(tabs) >= maxTabs {
				break
			}
			pid := cat.ID
			subEnt := entity.Category{
				ID:       ch.ID,
				Slug:     ch.Slug,
				ParentID: &pid,
				Parent:   &rootEnt,
			}
			tryAppendTab(ch.ID, ch.Name, entity.CategoryPublicWebPath(subEnt))
		}
	}
	if len(tabs) < maxTabs {
		fb := categoriesFromDistinctRecentPosts(db, 96, maxTabs*8)
		for _, cat := range fb {
			if len(tabs) >= maxTabs {
				break
			}
			tryAppendTab(cat.ID, cat.Name, entity.CategoryPublicWebPath(cat))
		}
	}
	if len(tabs) == 0 {
		return nil
	}
	return tabs
}

func loadNeoTickerPostsForHeader(db *gorm.DB, siteSetting entity.SiteSetting, limit int) []gin.H {
	if limit <= 0 {
		limit = 8
	}
	var posts []entity.Post
	if err := db.Preload("Category").
		Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).
		Order("updated_at DESC").
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil
	}
	return buildNeoTickerRows(posts, siteSetting, limit)
}

func injectNeoHeaderTicker(db *gorm.DB, siteSetting entity.SiteSetting, data gin.H) {
	layout, _ := data["homepage_layout"].(string)
	if layout != entity.HomeLayoutNeo {
		return
	}
	if existing, ok := data["neo_ticker_posts"].([]gin.H); ok && len(existing) > 0 {
		return
	}
	data["neo_ticker_posts"] = loadNeoTickerPostsForHeader(db, siteSetting, 8)
}

func buildNeoTickerRows(bootstrap []entity.Post, siteSetting entity.SiteSetting, limit int) []gin.H {
	if limit <= 0 {
		limit = 8
	}
	out := make([]gin.H, 0, limit)
	for _, p := range bootstrap {
		if len(out) >= limit {
			break
		}
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}
		cn := strings.TrimSpace(p.Category.Name)
		cs := strings.TrimSpace(p.Category.Slug)
		ch := ""
		if cs != "" {
			ch = "/" + url.PathEscape(cs)
		}
		out = append(out, gin.H{
			"title":    title,
			"href":     clientPostPublicPath(siteSetting, p),
			"cat_name": cn,
			"cat_href": ch,
			"has_cat":  cn != "",
		})
	}
	return out
}

func clientCanonicalURL(c *gin.Context) string {
	path := c.Request.URL.Path
	proto := "http"
	if c.Request.TLS != nil {
		proto = "https"
	} else if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		proto = "https"
	}
	return proto + "://" + c.Request.Host + path
}

func categoryPathForPublishedPost(mode string, p entity.Post) string {
	if p.Category.ID == 0 {
		return ""
	}
	return entity.CategoryPathPrefixForArticleURL(mode, p.Category)
}

func preloadCategoryParentIfNeeded(db *gorm.DB, cat *entity.Category) {
	if cat == nil || cat.ID == 0 || cat.IsRoot() || cat.ParentID == nil {
		return
	}
	if cat.Parent != nil && cat.Parent.ID != 0 {
		return
	}
	var parent entity.Category
	if err := db.Select("id", "name", "slug").First(&parent, *cat.ParentID).Error; err == nil {
		cat.Parent = &parent
	}
}

// findSubcategoryBySlugs resolves a subcategory by parent slug + child slug (max depth 1).
func findSubcategoryBySlugs(db *gorm.DB, parentSlug, childSlug string) (*entity.Category, error) {
	parentSlug = strings.TrimSpace(parentSlug)
	childSlug = strings.TrimSpace(childSlug)
	if parentSlug == "" || childSlug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var parent entity.Category
	if err := db.Where("slug = ?", parentSlug).First(&parent).Error; err != nil {
		return nil, err
	}
	if !parent.IsRoot() {
		return nil, gorm.ErrRecordNotFound
	}
	var child entity.Category
	if err := db.Preload("Parent").Where("slug = ? AND parent_id = ?", childSlug, parent.ID).First(&child).Error; err != nil {
		return nil, err
	}
	return &child, nil
}

// clientPostPublicPath returns the root-relative canonical path for a published post (site article_slug_mode).
func clientPostPublicPath(siteSetting entity.SiteSetting, p entity.Post) string {
	return entity.PublishedArticleWebPath(siteSetting.ArticleSlugMode, categoryPathForPublishedPost(siteSetting.ArticleSlugMode, p), p.Slug)
}

func clientReservedPublicPathSegment(low string) bool {
	switch low {
	case "admin", "api", "static", "health", "posts", "rss-feeds", "login", "rss", "halaman", "tag", "author",
		"favicon.ico", "robots.txt", "ads.txt":
		return true
	default:
		return false
	}
}

func normalizePublicTimezone(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "Asia/Jakarta"
	}
	if _, err := time.LoadLocation(v); err != nil {
		return "Asia/Jakarta"
	}
	return v
}

// clientSocialLinksH returns template keys for header/footer social icons (used by client layout).
func clientSocialLinksH(siteSetting entity.SiteSetting) gin.H {
	return gin.H{
		"social_facebook_url":  strings.TrimSpace(siteSetting.SocialFacebookURL),
		"social_x_url":         strings.TrimSpace(siteSetting.SocialXURL),
		"social_instagram_url": strings.TrimSpace(siteSetting.SocialInstagramURL),
		"social_youtube_url":   strings.TrimSpace(siteSetting.SocialYouTubeURL),
		"social_tiktok_url":    strings.TrimSpace(siteSetting.SocialTikTokURL),
		"social_rss_url":       strings.TrimSpace(siteSetting.SocialRSSURL),
	}
}

// injectClientVerificationMeta adds search-engine verification meta values to the template data map.
func injectClientVerificationMeta(db *gorm.DB, data gin.H) {
	var sc entity.SiteConnection
	if err := db.First(&sc).Error; err != nil {
		return
	}
	if v := strings.TrimSpace(sc.Ahrefs); v != "" {
		data["verify_ahrefs"] = v
	}
	if v := strings.TrimSpace(sc.Google); v != "" {
		data["verify_google"] = v
	}
	if v := strings.TrimSpace(sc.Bing); v != "" {
		data["verify_bing"] = v
	}
	if v := strings.TrimSpace(sc.Yandex); v != "" {
		data["verify_yandex"] = v
	}
}

// clientBrowserTitleSuffix returns a short " - SiteName" fragment for listing/archive <title> tags
// (not the long default SEO title string).
func clientBrowserTitleSuffix(siteSetting entity.SiteSetting) string {
	if s := strings.TrimSpace(siteSetting.SiteName); s != "" {
		return " - " + s
	}
	if s := strings.TrimSpace(siteSetting.DefaultTitle); s != "" {
		return " - " + s
	}
	return ""
}

// injectClientAbsoluteAssetURLs sets public_base (origin, no trailing slash) for full static URLs
// and rewrites favicon_url to an absolute URL when it is host-relative or path-only.
func injectClientAbsoluteAssetURLs(c *gin.Context, siteSetting entity.SiteSetting, data gin.H) {
	if data == nil {
		return
	}
	data["scraper_guard_meta"] = template.HTML(scraperguard.MetaTagHTML())
	data["current_year"] = time.Now().Year()
	base := canonicalBaseURL(c)
	data["public_base"] = base
	data["article_slug_mode"] = entity.NormalizeArticleSlugMode(siteSetting.ArticleSlugMode)
	if fav := strings.TrimSpace(siteSetting.FaviconURL); fav != "" {
		data["favicon_url"] = toAbsoluteURL(base, fav)
	}
	normalizeClientSocialImageURLs(base, data)
	absolutizeClientDataURLs(base, data)
	injectClientJSONLDURLLiterals(data)
	injectClientJSONLDHTML(data)
	injectAdsCenterScriptLiteral(data)
}

func jsonStringLiteral(raw string) template.JS {
	b, err := json.Marshal(strings.TrimSpace(raw))
	if err != nil {
		return template.JS(`""`)
	}
	return template.JS(string(b))
}

func jsStringContent(raw string) template.JSStr {
	return template.JSStr(strings.TrimSpace(raw))
}

func injectClientJSONLDURLLiterals(data gin.H) {
	if data == nil {
		return
	}
	strValue := func(key string) string {
		if v, ok := data[key]; ok && v != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return ""
	}
	base := strings.TrimRight(strValue("public_base"), "/")
	current := strValue("current_url")
	metaImage := strValue("meta_image")
	logoURL := strValue("logo_url")
	authorPath := strValue("author_path")
	authorImg := strValue("author_img")
	catSlug := strings.Trim(strings.TrimSpace(strValue("cat_slug")), "/")

	data["public_base_json"] = jsonStringLiteral(base)
	data["public_base_root_json"] = jsonStringLiteral(base + "/")
	data["organization_id_json"] = jsonStringLiteral(base + "/#organization")
	data["tag_index_url_json"] = jsonStringLiteral(base + "/tag")
	data["author_index_url_json"] = jsonStringLiteral(base + "/author")
	data["current_url_json"] = jsonStringLiteral(current)
	data["current_webpage_id_json"] = jsonStringLiteral(current + "#webpage")
	data["current_article_id_json"] = jsonStringLiteral(current + "#article")
	data["current_collection_id_json"] = jsonStringLiteral(current + "#collection")
	data["current_profile_id_json"] = jsonStringLiteral(current + "#profile")
	data["meta_image_json"] = jsonStringLiteral(metaImage)
	data["logo_url_json"] = jsonStringLiteral(logoURL)
	data["author_path_json"] = jsonStringLiteral(authorPath)
	data["author_img_json"] = jsonStringLiteral(authorImg)
	data["public_base_jsstr"] = jsStringContent(base)
	data["public_base_root_jsstr"] = jsStringContent(base + "/")
	data["organization_id_jsstr"] = jsStringContent(base + "/#organization")
	data["tag_index_url_jsstr"] = jsStringContent(base + "/tag")
	data["author_index_url_jsstr"] = jsStringContent(base + "/author")
	data["current_url_jsstr"] = jsStringContent(current)
	data["current_webpage_id_jsstr"] = jsStringContent(current + "#webpage")
	data["current_article_id_jsstr"] = jsStringContent(current + "#article")
	data["current_collection_id_jsstr"] = jsStringContent(current + "#collection")
	data["current_profile_id_jsstr"] = jsStringContent(current + "#profile")
	data["meta_image_jsstr"] = jsStringContent(metaImage)
	data["logo_url_jsstr"] = jsStringContent(logoURL)
	data["author_path_jsstr"] = jsStringContent(authorPath)
	data["author_img_jsstr"] = jsStringContent(authorImg)
	if catSlug != "" {
		data["cat_url_json"] = jsonStringLiteral(base + "/" + url.PathEscape(catSlug))
		data["cat_url_jsstr"] = jsStringContent(base + "/" + url.PathEscape(catSlug))
	}
}

func jsonLDScriptHTML(v any) template.HTML {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return template.HTML(`<script type="application/ld+json">` + "\n" + strings.TrimSpace(b.String()) + "\n" + `</script>`)
}

func appendJSONLDScript(parts []string, v any) []string {
	h := strings.TrimSpace(string(jsonLDScriptHTML(v)))
	if h == "" {
		return parts
	}
	return append(parts, h)
}

func jsonLDOrganizationNode(base, siteName, imageURL string, data gin.H) gin.H {
	org := gin.H{
		"@id":   base + "/#organization",
		"@type": "Organization",
		"name":  siteName,
		"url":   base,
	}
	if imageURL != "" {
		org["logo"] = gin.H{
			"@type":  "ImageObject",
			"@id":    base + "/#logo",
			"url":    imageURL,
			"width":  512,
			"height": 512,
		}
	}
	if data != nil {
		sameAs := make([]string, 0, 5)
		for _, key := range []string{"social_facebook_url", "social_x_url", "social_instagram_url", "social_youtube_url", "social_tiktok_url"} {
			v := strings.TrimSpace(fmt.Sprint(data[key]))
			low := strings.ToLower(v)
			if v != "" && v != "<nil>" && (strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "http://")) {
				sameAs = append(sameAs, v)
			}
		}
		if len(sameAs) > 0 {
			org["sameAs"] = sameAs
		}
	}
	return org
}

func jsonLDWebsiteNode(base, siteName string) gin.H {
	return gin.H{
		"@id":        base + "/#website",
		"@type":      "WebSite",
		"name":       siteName,
		"url":        base,
		"inLanguage": "id-ID",
		"publisher":  gin.H{"@id": base + "/#organization"},
		"potentialAction": gin.H{
			"@type":       "SearchAction",
			"target":      base + "/search?q={search_term_string}",
			"query-input": "required name=search_term_string",
		},
	}
}

func splitMetaKeywords(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

func injectClientJSONLDHTML(data gin.H) {
	if data == nil {
		return
	}
	strValue := func(key string) string {
		if v, ok := data[key]; ok && v != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return ""
	}
	boolValue := func(key string) bool {
		if v, ok := data[key]; ok && v != nil {
			if b, ok := v.(bool); ok {
				return b
			}
			return strings.EqualFold(strings.TrimSpace(fmt.Sprint(v)), "true")
		}
		return false
	}

	base := strings.TrimRight(strValue("public_base"), "/")
	current := strValue("current_url")
	siteName := strValue("site_name")
	metaDesc := strValue("meta_description")
	metaImage := strValue("meta_image")
	publisherImage := strValue("publisher_image")
	if publisherImage == "" {
		publisherImage = metaImage
	}
	var scripts []string

	if siteName != "" && boolValue("is_homepage") {
		scripts = appendJSONLDScript(scripts, gin.H{
			"@context": "https://schema.org",
			"@graph": []gin.H{
				jsonLDOrganizationNode(base, siteName, publisherImage, data),
				jsonLDWebsiteNode(base, siteName),
				{
					"@id":         base + "/#webpage",
					"@type":       "WebPage",
					"url":         base,
					"name":        strValue("meta_title"),
					"description": metaDesc,
					"inLanguage":  "id-ID",
					"isPartOf":    gin.H{"@id": base + "/#website"},
					"about":       gin.H{"@id": base + "/#organization"},
				},
			},
		})
	}

	if current != "" && strValue("post_title") != "" && strValue("og_type") == "article" {
		postTitle := strValue("post_title")
		articleDesc := metaDesc
		if articleDesc == "" {
			articleDesc = postTitle
		}
		articleType := strValue("article_schema_type")
		if articleType == "" {
			articleType = entity.ArticleSchemaTypeArticle
		}
		primaryImageID := current + "#primaryimage"
		authorID := strings.TrimRight(strValue("author_path"), "/") + "#author"
		if strings.TrimSpace(strValue("author_path")) == "" {
			authorID = current + "#author"
		}
		webPage := gin.H{
			"@id":         current + "#webpage",
			"@type":       "WebPage",
			"url":         current,
			"name":        postTitle + " - " + siteName,
			"description": articleDesc,
			"inLanguage":  "id-ID",
			"isPartOf":    gin.H{"@id": base + "/#website"},
		}
		if metaImage != "" {
			webPage["primaryImageOfPage"] = gin.H{"@id": primaryImageID}
		}
		imageNode := gin.H{
			"@id":    primaryImageID,
			"@type":  "ImageObject",
			"url":    metaImage,
			"width":  1200,
			"height": 675,
		}
		authorNode := gin.H{
			"@id":   authorID,
			"@type": "Person",
			"name":  strValue("author_name"),
		}
		if authorPath := strValue("author_path"); authorPath != "" {
			authorNode["url"] = authorPath
		}
		article := gin.H{
			"@type":               articleType,
			"@id":                 current + "#article",
			"mainEntityOfPage":    gin.H{"@id": current + "#webpage"},
			"headline":            postTitle,
			"description":         articleDesc,
			"inLanguage":          "id-ID",
			"isAccessibleForFree": true,
			"url":                 current,
			"datePublished":       strValue("post_created_at_iso"),
			"dateModified":        strValue("post_updated_at_iso"),
			"author":              gin.H{"@id": authorID},
			"publisher":           gin.H{"@id": base + "/#organization"},
		}
		if metaImage != "" {
			article["image"] = []gin.H{{"@id": primaryImageID}}
			article["thumbnailUrl"] = metaImage
		}
		if catName := strValue("cat_name"); catName != "" {
			article["articleSection"] = catName
		}
		if kws := splitMetaKeywords(strValue("meta_keywords")); len(kws) > 0 {
			article["keywords"] = kws
		}

		crumbs := []gin.H{{"@type": "ListItem", "position": 1, "name": "Beranda", "item": base}}
		pos := 2
		if pn := strValue("category_parent_name"); pn != "" {
			ph := strings.TrimSpace(strValue("category_parent_href"))
			if ph == "" {
				ph = "/"
			} else if !strings.HasPrefix(ph, "/") {
				ph = "/" + ph
			}
			crumbs = append(crumbs, gin.H{"@type": "ListItem", "position": pos, "name": pn, "item": base + ph})
			pos++
		}
		catName := strValue("cat_name")
		catHref := strings.TrimSpace(strValue("cat_href"))
		if catHref == "" {
			if catSlug := strings.Trim(strings.TrimSpace(strValue("cat_slug")), "/"); catSlug != "" {
				catHref = "/" + url.PathEscape(catSlug)
			}
		} else if !strings.HasPrefix(catHref, "/") {
			catHref = "/" + catHref
		}
		if catName != "" && catHref != "" {
			crumbs = append(crumbs, gin.H{"@type": "ListItem", "position": pos, "name": catName, "item": base + catHref})
			pos++
		}
		crumbs = append(crumbs, gin.H{"@type": "ListItem", "position": pos, "name": postTitle, "item": current})
		breadcrumb := gin.H{"@id": current + "#breadcrumb", "@type": "BreadcrumbList", "itemListElement": crumbs}
		graph := []gin.H{
			jsonLDOrganizationNode(base, siteName, publisherImage, data),
			jsonLDWebsiteNode(base, siteName),
			webPage,
		}
		if metaImage != "" {
			graph = append(graph, imageNode)
		}
		graph = append(graph, authorNode, article, breadcrumb)
		scripts = appendJSONLDScript(scripts, gin.H{"@context": "https://schema.org", "@graph": graph})
	}

	if current != "" {
		switch strValue("schema_page_type") {
		case "category":
			categoryTitle := strValue("schema_category_title")
			if categoryTitle == "" {
				categoryTitle = strValue("category_name")
			}
			desc := metaDesc
			if desc == "" {
				desc = "Kumpulan artikel kategori " + categoryTitle
			}
			graph := []gin.H{
				jsonLDOrganizationNode(base, siteName, publisherImage, data),
				jsonLDWebsiteNode(base, siteName),
				{
					"@id":         current + "#collection",
					"@type":       "CollectionPage",
					"url":         current,
					"name":        categoryTitle + " - " + siteName,
					"description": desc,
					"inLanguage":  "id-ID",
					"isPartOf":    gin.H{"@id": base + "/#website"},
					"about":       gin.H{"@type": "Thing", "name": categoryTitle},
				},
				{
					"@id":   current + "#breadcrumb",
					"@type": "BreadcrumbList",
					"itemListElement": []gin.H{
						{"@type": "ListItem", "position": 1, "name": siteName, "item": base},
						{"@type": "ListItem", "position": 2, "name": categoryTitle, "item": current},
					},
				},
			}
			scripts = appendJSONLDScript(scripts, gin.H{
				"@context": "https://schema.org",
				"@graph":   graph,
			})
		case "tag":
			tagName := strValue("tag_name")
			desc := metaDesc
			if desc == "" {
				desc = "Kumpulan artikel dengan tag " + tagName
			}
			graph := []gin.H{
				jsonLDOrganizationNode(base, siteName, publisherImage, data),
				jsonLDWebsiteNode(base, siteName),
				{
					"@id":         current + "#collection",
					"@type":       "CollectionPage",
					"url":         current,
					"name":        "Tag: " + tagName + " - " + siteName,
					"description": desc,
					"inLanguage":  "id-ID",
					"isPartOf":    gin.H{"@id": base + "/#website"},
					"about":       gin.H{"@type": "Thing", "name": tagName},
				},
				{
					"@id":   current + "#breadcrumb",
					"@type": "BreadcrumbList",
					"itemListElement": []gin.H{
						{"@type": "ListItem", "position": 1, "name": siteName, "item": base},
						{"@type": "ListItem", "position": 2, "name": "Tag", "item": base + "/tag"},
						{"@type": "ListItem", "position": 3, "name": tagName, "item": current},
					},
				},
			}
			scripts = appendJSONLDScript(scripts, gin.H{
				"@context": "https://schema.org",
				"@graph":   graph,
			})
		case "author":
			authorName := strValue("author_name")
			desc := metaDesc
			if desc == "" {
				desc = "Kumpulan artikel dari author " + authorName
			}
			person := gin.H{"@id": current + "#author", "@type": "Person", "name": authorName, "url": current}
			if bio := strValue("author_bio"); bio != "" {
				person["description"] = bio
			}
			if img := strValue("author_img"); img != "" {
				person["image"] = img
			}
			graph := []gin.H{
				jsonLDOrganizationNode(base, siteName, publisherImage, data),
				jsonLDWebsiteNode(base, siteName),
				{
					"@type": "ProfilePage", "@id": current + "#profile", "url": current,
					"name": "Author: " + authorName + " - " + siteName, "description": desc, "inLanguage": "id-ID",
					"isPartOf":   gin.H{"@id": base + "/#website"},
					"mainEntity": gin.H{"@id": current + "#author"},
				},
				person,
				{"@id": current + "#breadcrumb", "@type": "BreadcrumbList", "itemListElement": []gin.H{
					{"@type": "ListItem", "position": 1, "name": siteName, "item": base},
					{"@type": "ListItem", "position": 2, "name": "Author", "item": base + "/author"},
					{"@type": "ListItem", "position": 3, "name": authorName, "item": current},
				}},
			}
			scripts = appendJSONLDScript(scripts, gin.H{"@context": "https://schema.org", "@graph": graph})
		case "posts":
			desc := metaDesc
			if desc == "" {
				desc = "Daftar semua artikel yang dipublikasikan."
			}
			title := strValue("title")
			graph := []gin.H{
				jsonLDOrganizationNode(base, siteName, publisherImage, data),
				jsonLDWebsiteNode(base, siteName),
				{"@type": "CollectionPage", "@id": current + "#collection", "url": current, "name": title + " - " + siteName, "description": desc, "inLanguage": "id-ID", "isPartOf": gin.H{"@id": base + "/#website"}},
				{"@id": current + "#breadcrumb", "@type": "BreadcrumbList", "itemListElement": []gin.H{
					{"@type": "ListItem", "position": 1, "name": siteName, "item": base},
					{"@type": "ListItem", "position": 2, "name": title, "item": current},
				}},
			}
			scripts = appendJSONLDScript(scripts, gin.H{"@context": "https://schema.org", "@graph": graph})
		case "page":
			pageTitle := strValue("page_title")
			desc := metaDesc
			if desc == "" {
				desc = pageTitle
			}
			graph := []gin.H{
				jsonLDOrganizationNode(base, siteName, publisherImage, data),
				jsonLDWebsiteNode(base, siteName),
				{"@type": "WebPage", "@id": current + "#webpage", "url": current, "name": pageTitle + " - " + siteName, "description": desc, "inLanguage": "id-ID", "isPartOf": gin.H{"@id": base + "/#website"}},
				{"@id": current + "#breadcrumb", "@type": "BreadcrumbList", "itemListElement": []gin.H{
					{"@type": "ListItem", "position": 1, "name": siteName, "item": base},
					{"@type": "ListItem", "position": 2, "name": pageTitle, "item": current},
				}},
			}
			scripts = appendJSONLDScript(scripts, gin.H{"@context": "https://schema.org", "@graph": graph})
		}
	}

	if len(scripts) > 0 {
		data["json_ld_html"] = template.HTML(strings.Join(scripts, "\n"))
	}
}

func rebuildClientJSONLD(data gin.H) {
	if data == nil {
		return
	}
	delete(data, "json_ld_html")
	injectClientJSONLDHTML(data)
}

// injectAdsCenterScriptLiteral exposes ads_center as a JSON string for inline <script>
// (client-side article render and late AdSense init).
func injectAdsCenterScriptLiteral(data gin.H) {
	if data == nil {
		return
	}
	var raw string
	switch v := data["ads_center"].(type) {
	case template.HTML:
		raw = string(v)
	case string:
		raw = v
	default:
		data["ads_center_json"] = template.JS("null")
		return
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		data["ads_center_json"] = template.JS("null")
		return
	}
	b, err := json.Marshal(raw)
	if err != nil {
		data["ads_center_json"] = template.JS("null")
		return
	}
	data["ads_center_json"] = template.JS(string(b))
}

func injectAdminAbsoluteBaseURL(c *gin.Context, data gin.H) {
	if c == nil || data == nil {
		return
	}
	base := canonicalBaseURL(c)
	data["public_base"] = base
	if o := normalizedPublicSiteOrigin(); o != "" {
		data["admin_public_site_origin"] = o
	}
}

// absolutizeClientDataURLs rewrites common client-facing URL fields to absolute URLs using baseURL.
// This keeps SSR HTML stable for crawlers and for clients that need full links (href/src) including domain.
func absolutizeClientDataURLs(baseURL string, data gin.H) {
	if strings.TrimSpace(baseURL) == "" || data == nil {
		return
	}
	// Keys that commonly hold links or image URLs used by templates.
	keySet := map[string]struct{}{
		"href":            {},
		"src":             {},
		"thumb":           {},
		"image":           {},
		"image_url":       {},
		"hero_url":        {},
		"logo_url":        {},
		"meta_image":      {},
		"publisher_image": {},
		"author_img":      {},
		"favicon_url":     {},
		"cat_href":        {},
		"tag_href":        {},
		"author_path":     {},
		"source_url":      {},
		"current_url":     {}, // should already be absolute but safe
	}
	// Keys that contain HTML fragments where href/src may be present inside markup.
	htmlKeySet := map[string]struct{}{
		"post_body":    {},
		"page_content": {},
	}
	var walkAny func(v any) any
	var walkMap func(m map[string]any) map[string]any
	var walkSlice func(s []any) []any

	walkMap = func(m map[string]any) map[string]any {
		if m == nil {
			return m
		}
		for k, v := range m {
			if k == "public_base" {
				continue
			}
			if _, ok := htmlKeySet[k]; ok {
				switch vv := v.(type) {
				case template.HTML:
					m[k] = template.HTML(absolutizeHTMLAttrURLs(baseURL, string(vv)))
					continue
				case string:
					m[k] = absolutizeHTMLAttrURLs(baseURL, vv)
					continue
				}
			}
			if _, ok := keySet[k]; ok {
				if s, ok := v.(string); ok {
					m[k] = toClientAbsoluteURL(baseURL, s)
					continue
				}
			}
			// Safety net: any string that looks like a root-relative URL should be absolutized,
			// even if its key is not listed in keySet (e.g. newly added view-model fields).
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
					m[k] = toClientAbsoluteURL(baseURL, s)
					continue
				}
			}
			m[k] = walkAny(v)
		}
		return m
	}
	walkSlice = func(s []any) []any {
		for i := range s {
			s[i] = walkAny(s[i])
		}
		return s
	}
	walkAny = func(v any) any {
		switch vv := v.(type) {
		case gin.H:
			return walkMap(map[string]any(vv))
		case map[string]any:
			return walkMap(vv)
		case []gin.H:
			for i := range vv {
				_ = walkMap(map[string]any(vv[i]))
			}
			return vv
		case []any:
			return walkSlice(vv)
		default:
			return v
		}
	}

	_ = walkMap(map[string]any(data))
}

func absolutizeHTMLAttrURLs(baseURL, html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	return htmlAttrURLRe.ReplaceAllStringFunc(html, func(m string) string {
		// m looks like: href="/path" OR src='/path'
		// Extract the quoted path portion and rebuild using toAbsoluteURL.
		// Keep the original attribute name and quote type.
		lower := strings.ToLower(m)
		attr := "href"
		if strings.Contains(lower, "src") && !strings.Contains(lower, "href") {
			attr = "src"
		}
		q := `"`
		iq := strings.IndexAny(m, `"'`)
		if iq >= 0 {
			q = m[iq : iq+1]
		}
		parts := strings.Split(m, q)
		if len(parts) < 3 {
			return m
		}
		path := parts[1]
		abs := toClientAbsoluteURL(baseURL, path)
		return attr + "=" + q + abs + q
	})
}

func loadClientMenuPages(db *gorm.DB, limit int) []entity.Page {
	var pages []entity.Page
	q := db.Model(&entity.Page{}).
		Select("title, slug, created_at, updated_at").
		Where("status = ?", "published").
		Order("COALESCE(updated_at, created_at) DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&pages).Error
	if err != nil {
		return []entity.Page{}
	}
	return pages
}

// loadNavCategories returns categories for the client header (ordered by admin sort, then name).
func loadNavCategories(db *gorm.DB, layout string) []entity.NavCategoryNode {
	// declare cache param
	store := cache.GlobalStore()
	var cats []entity.Category
	const (
		loadNavCategoriesCacheTTL = 60 * time.Minute
		loadNavCategoriesCacheKey = "nav_categories"
	)

	// get nav_categories
	if store != nil {
		if ok, _ := store.GetJSON(context.Background(), loadNavCategoriesCacheKey, &cats); ok {
			return navTreeForLayout(db, layout, entity.BuildNavCategoryTree(cats))
		}
	}

	if err := db.Model(&entity.Category{}).
		Select("id", "sort", "name", "slug", "parent_id").
		Where("TRIM(COALESCE(name, '')) <> ''").
		Where("TRIM(COALESCE(slug, '')) <> ''").
		Order("CASE WHEN sort < 1 THEN 2147483647 ELSE sort END ASC").
		Order("LOWER(name) ASC").
		Order("id ASC").
		Find(&cats).Error; err != nil {
		return nil
	}

	// cache nav_categories
	if store != nil {
		store.SetJSON(context.Background(), loadNavCategoriesCacheKey, &cats, loadNavCategoriesCacheTTL)
	}
	return navTreeForLayout(db, layout, entity.BuildNavCategoryTree(cats))
}

func navTreeForLayout(db *gorm.DB, layout string, tree []entity.NavCategoryNode) []entity.NavCategoryNode {
	if !publicshelf.Active(layout) {
		return tree
	}
	return filterNavToShelf(tree, shelfCategoryIDs(db))
}

func visiblePublicScopes(layout string) []func(*gorm.DB) *gorm.DB {
	scopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
	if publicshelf.Active(layout) {
		scopes = append(scopes, publicshelf.Scope(""))
	}
	return scopes
}

func shelfCategoryIDs(db *gorm.DB) map[uint]struct{} {
	var ids []uint
	_ = db.Model(&entity.Post{}).
		Scopes(publishedpost.ScopeVisiblePublished("", time.Now()), publicshelf.Scope("")).
		Distinct("category_id").
		Pluck("category_id", &ids).Error
	out := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id != 0 {
			out[id] = struct{}{}
		}
	}
	return out
}

func filterNavToShelf(nodes []entity.NavCategoryNode, allowed map[uint]struct{}) []entity.NavCategoryNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]entity.NavCategoryNode, 0, len(nodes))
	for _, node := range nodes {
		children := make([]entity.NavCategoryChild, 0, len(node.Children))
		for _, child := range node.Children {
			if _, ok := allowed[child.ID]; ok {
				children = append(children, child)
			}
		}
		_, keep := allowed[node.ID]
		if !keep && len(children) == 0 {
			continue
		}
		node.Children = children
		out = append(out, node)
	}
	return out
}

// serveClientStaticPage renders a published static Page (e.g. /kebijakan-privasi).
func serveClientStaticPage(c *gin.Context, db *gorm.DB, page *entity.Page) error {
	tmpl, err := template.ParseFiles(
		"public/templates/layouts/client/app.html",
		"public/templates/layouts/client/header.html",
		"public/templates/layouts/client/footer.html",
		"public/templates/pages/client/page.html",
	)
	if err != nil {
		return err
	}

	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := strings.TrimSpace(page.Title)
	if title == "" {
		title = "Halaman"
	}
	metaDesc := textPreview(page.Content, 180)
	if metaDesc == "" {
		metaDesc = strings.TrimSpace(siteMeta.DefaultDesc)
	}
	data := gin.H{
		"title":                 title,
		"meta_title":            title + clientBrowserTitleSuffix(siteSetting),
		"meta_description":      metaDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"tagline":               strings.TrimSpace(siteSetting.Tagline),
		"homepage_layout":       entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
		"footer_copy":           siteSetting.FooterCopy,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"accent_color":          siteSetting.AccentColor,
		"background_primary":    siteSetting.BackgroundPrimary,
		"background_secondary":  siteSetting.BackgroundSecondary,
		"text_primary":          siteSetting.TextPrimary,
		"text_secondary":        siteSetting.TextSecondary,
		"border_color":          siteSetting.BorderColor,
		"static_version":        clientStaticVersion,
		"header_script":         clientSiteScriptHTML(siteSetting.HeaderScript),
		"footer_script":         clientSiteScriptHTML(siteSetting.FooterScript),
		"ads_top":               clientAdSlotHTML(siteSetting.AdsTop),
		"ads_center":            clientAdSlotHTML(siteSetting.AdsCenter),
		"ads_bottom":            clientAdSlotHTML(siteSetting.AdsBottom),
		"ads_lazy_load_enabled": siteSetting.AdsLazyLoadEnabled,
		"social_facebook_url":   strings.TrimSpace(siteSetting.SocialFacebookURL),
		"social_x_url":          strings.TrimSpace(siteSetting.SocialXURL),
		"social_instagram_url":  strings.TrimSpace(siteSetting.SocialInstagramURL),
		"social_youtube_url":    strings.TrimSpace(siteSetting.SocialYouTubeURL),
		"social_tiktok_url":     strings.TrimSpace(siteSetting.SocialTikTokURL),
		"social_rss_url":        strings.TrimSpace(siteSetting.SocialRSSURL),
		"current_url":           clientCanonicalURL(c),
		"local_timezone":        clientPublicTimezone,
		"schema_page_type":      "page",
		"page_title":            title,
		"page_content":          template.HTML(posthtml.EnsureExternalAnchorsNofollowBlank(page.Content, canonicalBaseURL(c))),
		"client_pages":          loadClientMenuPages(db, 0),
		"nav_categories":        loadNavCategories(db, siteSetting.HomepageLayout),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
	injectClientVerificationMeta(db, data)
	injectClientAbsoluteAssetURLs(c, siteSetting, data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

func serveClientErrorPage(c *gin.Context, db *gorm.DB, status int, heading, description string) {
	tmpl, err := template.ParseFiles(
		"public/templates/layouts/client/app.html",
		"public/templates/layouts/client/header.html",
		"public/templates/layouts/client/footer.html",
		"public/templates/pages/client/error.html",
	)
	if err != nil {
		c.String(status, "error: %s", http.StatusText(status))
		return
	}
	siteSetting := loadSiteSetting(db)
	codeTitle := strconv.Itoa(status)
	if heading == "" {
		heading = http.StatusText(status)
	}
	if description == "" {
		description = "Terjadi kesalahan saat memuat halaman."
	}
	data := baseClientData(
		c,
		db,
		codeTitle+" "+heading,
		codeTitle+" "+heading+clientBrowserTitleSuffix(siteSetting),
		description,
	)
	data["error_code"] = status
	data["error_title"] = heading
	data["error_description"] = description
	c.Status(status)
	_ = tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

// staticCacheHeaders sets long-lived caching for versioned assets under /static/ (CSS/JS use ?v=).
func staticCacheHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Next()
	}
}

// RegisterFrontendRoutes configures routes for the public client pages.
func RegisterFrontendRoutes(r *gin.Engine, db *gorm.DB, cfg config.Config) {
	clientPublicTimezone = normalizePublicTimezone(cfg.LocalTimezone)
	jwtMgr := middleware.NewJWTManager(cfg.JWTSecret, cfg.JWTExpiresInHours)
	redirectHome := func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, notFoundRedirectTarget(db))
	}

	r.Use(staticCacheHeaders())
	r.GET(media.ResizeProxyPath, handler.ServeResizedImage)
	r.GET(media.ResizeProxyPath+"/*filepath", handler.ServeResizedImage)
	r.Static("/static", "./public/static")
	loadAllHTMLTemplates(r, "public/templates")

	serveClientIndex := func(c *gin.Context) {
		tmpl, err := clientHomeTemplate()
		if err != nil {
			c.String(http.StatusInternalServerError, "template error: %v", err)
			return
		}

		siteSetting := loadSiteSetting(db)
		siteMeta := siteMetaFromSetting(siteSetting)
		mt := strings.TrimSpace(siteMeta.DefaultTitle)
		if mt == "" {
			mt = strings.TrimSpace(siteSetting.SiteName)
		}
		if mt == "" {
			mt = "Tirtayasa"
		}
		metaDesc := strings.TrimSpace(siteMeta.DefaultDesc)
		if metaDesc == "" {
			metaDesc = strings.TrimSpace(siteSetting.Tagline)
		}

		var pageTitle, metaTitle string
		siteName := strings.TrimSpace(siteSetting.SiteName)
		tagline := strings.TrimSpace(siteSetting.Tagline)
		if siteName == "" {
			siteName = mt
		}
		switch p := c.Request.URL.Path; p {
		case "/":
			pageTitle = "Home"
			if tagline != "" {
				metaTitle = siteName + " - " + tagline
			} else {
				metaTitle = siteName
			}
		case "/author":
			pageTitle = "Author"
			metaTitle = "Author" + clientBrowserTitleSuffix(siteSetting)
		default:
			pageTitle = "Artikel"
			metaTitle = "Artikel" + clientBrowserTitleSuffix(siteSetting)
			if strings.HasPrefix(p, "/profile/") {
				metaTitle = "Author" + clientBrowserTitleSuffix(siteSetting)
				break
			}
			trimmed := strings.Trim(p, "/")
			if trimmed != "" && !strings.Contains(trimmed, "/") {
				var post entity.Post
				if err := db.Select("title, content, meta_desc").Where("slug = ?", trimmed).Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
					t := strings.TrimSpace(post.Title)
					if t != "" {
						pageTitle = t
						metaTitle = t
					}
					// Halaman artikel: deskripsi meta mengikuti metadata/konten artikel, bukan default homepage.
					if post.MetaDesc != nil {
						if md := strings.TrimSpace(*post.MetaDesc); md != "" {
							metaDesc = md
						}
					}
					if strings.TrimSpace(metaDesc) == "" {
						if md := textPreview(post.Content, 180); md != "" {
							metaDesc = md
						}
					}
				}
			}
		}

		entity.NormalizeSidebarListLimits(&siteSetting)
		homeVM, homeBootstrap := buildHomePageViewModel(db, siteSetting)
		popularRows, latestRows := loadSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitPopular, siteSetting.SidebarListLimitLatest, homeBootstrap)
		data := gin.H{
			"title":                  pageTitle,
			"meta_title":             metaTitle,
			"meta_description":       metaDesc,
			"meta_keywords":          siteMeta.DefaultKeywords,
			"meta_image":             siteMeta.DefaultImage,
			"og_type":                "website",
			"site_name":              siteSetting.SiteName,
			"tagline":                strings.TrimSpace(siteSetting.Tagline),
			"footer_copy":            siteSetting.FooterCopy,
			"logo_url":               siteSetting.LogoURL,
			"favicon_url":            siteSetting.FaviconURL,
			"theme_color":            themeColorMeta(siteSetting),
			"accent_color":           siteSetting.AccentColor,
			"background_primary":     siteSetting.BackgroundPrimary,
			"background_secondary":   siteSetting.BackgroundSecondary,
			"text_primary":           siteSetting.TextPrimary,
			"text_secondary":         siteSetting.TextSecondary,
			"border_color":           siteSetting.BorderColor,
			"font_family":            siteSetting.FontFamily,
			"content_font_size":      siteSetting.ContentFontSize,
			"sidebar_font_size":      siteSetting.SidebarFontSize,
			"image_caption_enabled":  siteSetting.ImageCaptionEnabled,
			"image_caption_mode":     siteSetting.ImageCaptionMode,
			"image_caption_template": siteSetting.ImageCaptionTemplate,
			"also_read_enabled":      siteSetting.AlsoReadEnabled,
			"also_read_max_inserts":  entity.EffectiveAlsoReadMaxInserts(siteSetting.AlsoReadMaxInserts),
			"image_resize_enabled":   siteSetting.ImageResizeEnabled,
			"img_resize_enabled":     siteSetting.ImgResizeEnabled,
			"img_resize_width":       siteSetting.ImgResizeWidth,
			"img_resize_height":      siteSetting.ImgResizeHeight,
			"img_resize_quality":     siteSetting.ImgResizeQuality,
			"image_resize_width":     siteSetting.ImageResizeWidth,
			"image_resize_quality":   siteSetting.ImageResizeQuality,
			"ads_lazy_load_enabled":  siteSetting.AdsLazyLoadEnabled,
			"avatar_resize_width":    siteSetting.AvatarResizeWidth,
			"avatar_resize_quality":  siteSetting.AvatarResizeQuality,
			"homepage_layout":        entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
			"static_version":         clientStaticVersion,
			"header_script":          clientSiteScriptHTML(siteSetting.HeaderScript),
			"footer_script":          clientSiteScriptHTML(siteSetting.FooterScript),
			"ads_top":                clientAdSlotHTML(siteSetting.AdsTop),
			"ads_center":             clientAdSlotHTML(siteSetting.AdsCenter),
			"ads_bottom":             clientAdSlotHTML(siteSetting.AdsBottom),
			"current_url":            clientCanonicalURL(c),
			"local_timezone":         clientPublicTimezone,
			"client_pages":           loadClientMenuPages(db, 0),
			"nav_categories":         loadNavCategories(db, siteSetting.HomepageLayout),
			"popular_posts":          popularRows,
			"latest_posts":           latestRows,
			"detik_recommend_posts":  buildRandomRecommendationRows(popularRows, latestRows, siteSetting.SidebarListLimitLatest),
			"is_homepage":            true,
			"load_app_js":            true,
			"load_post_js":           false,
			"login_href":             entity.EffectiveLoginHref(siteSetting.LoginURL),
		}
		mergeSidebarListLimitData(data, siteSetting)
		for k, v := range clientSocialLinksH(siteSetting) {
			data[k] = v
		}
		for k, v := range homeVM {
			data[k] = v
		}
		injectNeoHeaderTicker(db, siteSetting, data)
		injectClientVerificationMeta(db, data)
		injectClientAbsoluteAssetURLs(c, siteSetting, data)

		if err := tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
			return
		}
	}

	r.GET("/", middleware.PublicSearchLimit(), serveClientIndex)

	// Kanonik artikel: /slug atau /kategori/slug; redirect permanen dari URL lama /post/slug.
	r.GET("/post/:slug", func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" {
			redirectHome(c)
			return
		}
		ss := loadSiteSetting(db)
		var post entity.Post
		if err := db.Preload("Category").Where("slug = ?", slug).
			Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
			c.Redirect(http.StatusMovedPermanently, clientPostPublicPath(ss, post))
			return
		}
		c.Redirect(http.StatusMovedPermanently, "/"+url.PathEscape(slug))
	})

	// Daftar semua artikel (SSR, sama pola dengan tag/kategori)
	r.GET("/posts", func(c *gin.Context) {
		if err := serveClientPostsPage(c, db); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
		}
	})

	// Halaman tag kanonik: /tag/:slug (SSR)
	r.GET("/tag/:slug", func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" {
			redirectHome(c)
			return
		}
		tag, err := repositories.NewTagRepository(db).FindBySlug(slug)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				redirectHome(c)
				return
			}
			c.String(http.StatusInternalServerError, "render error: %v", err)
			return
		}
		if err := serveClientTagPage(c, db, tag); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
		}
	})

	// Halaman author kanonik: /profile/:slug (SSR)
	r.GET("/profile/:slug", func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" {
			redirectHome(c)
			return
		}
		if err := serveClientAuthorPage(c, db, slug); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				redirectHome(c)
				return
			}
			serveClientErrorPage(c, db, http.StatusInternalServerError, "Terjadi kesalahan", "Gagal memuat halaman author.")
		}
	})

	// RSS FEEDS LIST (derived from sitemap/category list)
	r.GET("/rss-feeds", func(c *gin.Context) {
		tmpl, err := template.ParseFiles(
			"public/templates/layouts/client/app.html",
			"public/templates/layouts/client/header.html",
			"public/templates/layouts/client/footer.html",
			"public/templates/pages/client/rss_feeds.html",
		)
		if err != nil {
			c.String(http.StatusInternalServerError, "template error: %v", err)
			return
		}

		siteMeta := loadSiteMeta(db)
		siteSetting := loadSiteSetting(db)
		baseURL := canonicalBaseURL(c)

		type rssFeedRow struct {
			Type string
			Name string
			URL  string
		}

		rows := make([]rssFeedRow, 0, 32)
		rows = append(rows, rssFeedRow{
			Type: "Page",
			Name: "News Sitemap (48h)",
			URL:  baseURL + "/rss/sitemap-news.xml",
		})
		rows = append(rows, rssFeedRow{
			Type: "Page",
			Name: "Latest Posts",
			URL:  baseURL + "/rss/latest-posts.xml",
		})

		var cats []entity.Category
		if err := db.Order("slug ASC").Find(&cats).Error; err != nil {
			c.String(http.StatusInternalServerError, "rss feeds error: %v", err)
			return
		}
		for _, cat := range cats {
			slug := strings.TrimSpace(cat.Slug)
			name := strings.TrimSpace(cat.Name)
			if slug == "" || name == "" {
				continue
			}
			rows = append(rows, rssFeedRow{
				Type: "Category",
				Name: name,
				URL:  baseURL + "/rss/" + slug + ".xml",
			})
		}

		data := gin.H{
			"title":                 "RSS Feeds",
			"meta_title":            "RSS Feeds" + clientBrowserTitleSuffix(siteSetting),
			"meta_description":      "Daftar RSS feed per kategori.",
			"meta_keywords":         siteMeta.DefaultKeywords,
			"meta_image":            siteMeta.DefaultImage,
			"site_name":             siteSetting.SiteName,
			"tagline":               strings.TrimSpace(siteSetting.Tagline),
			"homepage_layout":       entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
			"footer_copy":           siteSetting.FooterCopy,
			"logo_url":              siteSetting.LogoURL,
			"favicon_url":           siteSetting.FaviconURL,
			"theme_color":           themeColorMeta(siteSetting),
			"accent_color":          siteSetting.AccentColor,
			"background_primary":    siteSetting.BackgroundPrimary,
			"background_secondary":  siteSetting.BackgroundSecondary,
			"text_primary":          siteSetting.TextPrimary,
			"text_secondary":        siteSetting.TextSecondary,
			"border_color":          siteSetting.BorderColor,
			"static_version":        clientStaticVersion,
			"header_script":         clientSiteScriptHTML(siteSetting.HeaderScript),
			"footer_script":         clientSiteScriptHTML(siteSetting.FooterScript),
			"ads_top":               clientAdSlotHTML(siteSetting.AdsTop),
			"ads_center":            clientAdSlotHTML(siteSetting.AdsCenter),
			"ads_bottom":            clientAdSlotHTML(siteSetting.AdsBottom),
			"ads_lazy_load_enabled": siteSetting.AdsLazyLoadEnabled,
			"current_url":           clientCanonicalURL(c),
			"local_timezone":        clientPublicTimezone,
			"client_pages":          loadClientMenuPages(db, 0),
			"nav_categories":        loadNavCategories(db, siteSetting.HomepageLayout),
			"rss_rows":              rows,
		}
		for k, v := range clientSocialLinksH(siteSetting) {
			data[k] = v
		}
		injectClientVerificationMeta(db, data)
		injectClientAbsoluteAssetURLs(c, siteSetting, data)
		if err := tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
			return
		}
	})

	// URL lama /halaman/:slug → kanonik /:slug
	r.GET("/halaman/:slug", func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" {
			redirectHome(c)
			return
		}
		c.Redirect(http.StatusMovedPermanently, "/"+slug)
	})

	// SITEMAP
	r.GET("/sitemap.xml", func(c *gin.Context) {
		baseURL := canonicalBaseURL(c)

		rows, err := repositories.NewCategoryRepository(db).SitemapRows()
		if err != nil {
			c.String(http.StatusInternalServerError, "sitemap error: %v", err)
			return
		}

		items := make([]sitemapIndexEntry, 0, len(rows)+1)
		items = append(items, sitemapIndexEntry{
			Loc: baseURL + "/rss/sitemap-news.xml",
		})
		items = append(items, sitemapIndexEntry{
			Loc: baseURL + "/rss/latest-posts.xml",
		})
		for _, row := range rows {
			loc := baseURL + "/rss/" + row.Slug + ".xml"
			lastMod := ""
			if !row.UpdatedAt.IsZero() {
				lastMod = row.UpdatedAt.Format(time.RFC3339)
			}
			items = append(items, sitemapIndexEntry{
				Loc:     loc,
				LastMod: lastMod,
			})
		}

		c.Header("Cache-Control", "no-store")
		writeXMLResponse(c, sitemapIndex{
			Xmlns:    "http://www.sitemaps.org/schemas/sitemap/0.9",
			Sitemaps: items,
		})
	})

	r.GET("/rss/sitemap-news.xml", func(c *gin.Context) {
		site := loadSiteSetting(db)
		baseURL := canonicalBaseURL(c)
		cutoff := time.Now().Add(-48 * time.Hour)

		var posts []entity.Post
		newsScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
		if publicshelf.Active(site.HomepageLayout) {
			newsScopes = append(newsScopes, publicshelf.Scope("posts"))
		}
		err := db.
			Preload("Category").
			Scopes(newsScopes...).
			Where("COALESCE(posts.updated_at, posts.created_at) >= ?", cutoff).
			Order("COALESCE(posts.updated_at, posts.created_at) DESC").
			Limit(1500).
			Find(&posts).Error
		if err != nil {
			c.String(http.StatusInternalServerError, "news sitemap error: %v", err)
			return
		}

		urls := make([]newsSitemapURL, 0, 1000)
		seen := make(map[string]struct{}, len(posts))
		for _, p := range posts {
			if len(urls) >= 1000 {
				break
			}
			postSlug := strings.TrimSpace(p.Slug)
			if postSlug == "" {
				continue
			}
			rel := entity.PublishedArticleWebPath(site.ArticleSlugMode, categoryPathForPublishedPost(site.ArticleSlugMode, p), postSlug)
			postURL := strings.TrimRight(baseURL, "/") + rel
			if _, exists := seen[postURL]; exists {
				continue
			}
			seen[postURL] = struct{}{}
			publishedAt := p.UpdatedAt
			if publishedAt.IsZero() {
				publishedAt = p.CreatedAt
			}
			urls = append(urls, newsSitemapURL{
				Loc: postURL,
				News: newsMeta{
					Publication: newsPublication{
						Name:     strings.TrimSpace(site.SiteName),
						Language: "id",
					},
					PublicationDate: publishedAt.UTC().Format(time.RFC3339),
					Title:           strings.TrimSpace(p.Title),
				},
			})
		}

		writeXMLResponse(c, newsSitemap{
			Xmlns:      "http://www.sitemaps.org/schemas/sitemap/0.9",
			XmlnsNews:  "http://www.google.com/schemas/sitemap-news/0.9",
			URLSetRows: urls,
		})
	})

	r.GET("/robots.txt", func(c *gin.Context) {
		site := loadSiteSetting(db)
		body := strings.TrimSpace(site.RobotsTxt)
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(body+"\n"))
	})

	r.GET("/ads.txt", func(c *gin.Context) {
		site := loadSiteSetting(db)
		// Leave this empty until AdSense approves the site and returns a publisher ID.
		// A sample pub- line is not a valid authorization and is not served.
		body := publicAdsTxt(site.AdsTxt)
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(body+"\n"))
	})

	// RSS
	r.GET("/rss/:slug", func(c *gin.Context) {
		site := loadSiteSetting(db)
		baseURL := canonicalBaseURL(c)
		categorySlug := strings.TrimSpace(c.Param("slug"))
		categorySlug = strings.TrimSuffix(categorySlug, ".xml")
		if categorySlug == "" {
			redirectHome(c)
			return
		}

		// Special feed: latest posts across all categories.
		if categorySlug == "latest-posts" {
			var posts []entity.Post
			latestScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
			if publicshelf.Active(site.HomepageLayout) {
				latestScopes = append(latestScopes, publicshelf.Scope(""))
			}
			err := db.
				Preload("Author").
				Preload("Category").
				Preload("Tags").
				Scopes(latestScopes...).
				Order("updated_at DESC").
				Limit(1000).
				Find(&posts).Error
			if err != nil {
				c.String(http.StatusInternalServerError, "rss error: %v", err)
				return
			}

			items := make([]rssItem, 0, len(posts))
			for _, p := range posts {
				pubDate := p.UpdatedAt.UTC().Format(time.RFC1123Z)
				keywords := make([]string, 0, len(p.Tags))
				for _, t := range p.Tags {
					name := strings.TrimSpace(t.Name)
					if name != "" {
						keywords = append(keywords, name)
					}
				}
				description := buildSEODescription(p.Title, textPreview(p.Content, 280))
				var mediaContent *rssMediaContent
				var mediaThumbnail *rssMediaThumbnail
				if img := toAbsoluteURL(baseURL, strings.TrimSpace(derefString(p.Image))); img != "" {
					mediaContent = &rssMediaContent{
						URL:    img,
						Medium: "image",
						Type:   detectImageMediaType(img),
						Title: &rssMediaTitle{
							Type:  "plain",
							Value: p.Title,
						},
					}
					mediaThumbnail = &rssMediaThumbnail{URL: img}
				}
				rel := entity.PublishedArticleWebPath(site.ArticleSlugMode, categoryPathForPublishedPost(site.ArticleSlugMode, p), p.Slug)
				itemURL := strings.TrimRight(baseURL, "/") + rel
				items = append(items, rssItem{
					Title:          p.Title,
					Link:           itemURL,
					GUID:           rssGUID{IsPermaLink: "true", Value: itemURL},
					Description:    rssCDATA{Value: description},
					ContentEncoded: rssCDATA{Value: scraperguard.PrependMarkerToCDATA(p.Content)},
					PubDate:        pubDate,
					DCCreator:      strings.TrimSpace(p.Author.Name),
					MediaKeywords:  strings.Join(keywords, ", "),
					MediaContent:   mediaContent,
					MediaThumbnail: mediaThumbnail,
					Category:       strings.TrimSpace(p.Category.Name),
					AtomLinks: []rssAtomLink{
						{Href: itemURL, Rel: "alternate", Type: "text/html"},
					},
					News: &rssNews{
						Publication: rssNewsPublication{
							Name:     strings.TrimSpace(site.SiteName),
							Language: "id",
						},
						PublicationDate: p.UpdatedAt.UTC().Format(time.RFC3339),
						Title:           strings.TrimSpace(p.Title),
					},
				})
			}

			feed := rssFeed{
				Version:      "2.0",
				XMLNSDC:      "http://purl.org/dc/elements/1.1/",
				XMLNSSY:      "http://purl.org/rss/1.0/modules/syndication/",
				XMLNSAdmin:   "http://webns.net/mvcb/",
				XMLNSRDF:     "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
				XMLNSMedia:   "http://search.yahoo.com/mrss/",
				XMLNSContent: "http://purl.org/rss/1.0/modules/content/",
				XMLNSAtom:    "http://www.w3.org/2005/Atom",
				XMLNSNews:    "http://www.google.com/schemas/sitemap-news/0.9",
				Channel: rssChannel{
					Title:       strings.TrimSpace(site.SiteName),
					Link:        baseURL + "/rss/latest-posts.xml",
					Description: strings.TrimSpace(site.SiteName) + " — Latest Posts",
					DCLanguage:  "id-ID",
					DCRights:    "Copyright " + strconv.Itoa(time.Now().Year()) + " " + strings.TrimSpace(site.SiteName) + " & All Rights Reserved.",
					AtomLinks: []rssAtomLink{
						{Href: baseURL + "/rss/latest-posts.xml", Rel: "self", Type: "application/rss+xml"},
						{Href: "https://pubsubhubbub.appspot.com/", Rel: "hub"},
					},
					Items: items,
				},
			}

			var buf bytes.Buffer
			buf.WriteString(xml.Header)
			enc := xml.NewEncoder(&buf)
			enc.Indent("", "  ")
			if err := enc.Encode(feed); err != nil {
				c.String(http.StatusInternalServerError, "rss encode error: %v", err)
				return
			}
			if err := enc.Flush(); err != nil {
				c.String(http.StatusInternalServerError, "rss flush error: %v", err)
				return
			}
			c.Data(http.StatusOK, "application/xml; charset=utf-8", buf.Bytes())
			return
		}

		var category entity.Category
		if err := db.Where("slug = ?", categorySlug).First(&category).Error; err != nil {
			redirectHome(c)
			return
		}

		var posts []entity.Post
		catScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
		if publicshelf.Active(site.HomepageLayout) {
			catScopes = append(catScopes, publicshelf.Scope(""))
		}
		err := db.
			Preload("Author").
			Preload("Category").
			Preload("Tags").
			Where("category_id = ?", category.ID).
			Scopes(catScopes...).
			Order("updated_at DESC").
			Find(&posts).Error
		if err != nil {
			c.String(http.StatusInternalServerError, "rss error: %v", err)
			return
		}

		items := make([]rssItem, 0, len(posts))
		for _, p := range posts {
			pubDate := p.UpdatedAt.UTC().Format(time.RFC1123Z)
			keywords := make([]string, 0, len(p.Tags))
			for _, t := range p.Tags {
				name := strings.TrimSpace(t.Name)
				if name != "" {
					keywords = append(keywords, name)
				}
			}
			description := buildSEODescription(p.Title, textPreview(p.Content, 280))
			var mediaContent *rssMediaContent
			var mediaThumbnail *rssMediaThumbnail
			if img := toAbsoluteURL(baseURL, strings.TrimSpace(derefString(p.Image))); img != "" {
				mediaContent = &rssMediaContent{
					URL:    img,
					Medium: "image",
					Type:   detectImageMediaType(img),
					Title: &rssMediaTitle{
						Type:  "plain",
						Value: p.Title,
					},
				}
				mediaThumbnail = &rssMediaThumbnail{URL: img}
			}
			rel := entity.PublishedArticleWebPath(site.ArticleSlugMode, categoryPathForPublishedPost(site.ArticleSlugMode, p), p.Slug)
			itemURL := strings.TrimRight(baseURL, "/") + rel
			items = append(items, rssItem{
				Title:          p.Title,
				Link:           itemURL,
				GUID:           rssGUID{IsPermaLink: "true", Value: itemURL},
				Description:    rssCDATA{Value: description},
				ContentEncoded: rssCDATA{Value: scraperguard.PrependMarkerToCDATA(p.Content)},
				PubDate:        pubDate,
				DCCreator:      strings.TrimSpace(p.Author.Name),
				MediaKeywords:  strings.Join(keywords, ", "),
				MediaContent:   mediaContent,
				MediaThumbnail: mediaThumbnail,
				Category:       strings.TrimSpace(category.Name),
				AtomLinks: []rssAtomLink{
					{Href: itemURL, Rel: "alternate", Type: "text/html"},
				},
				News: &rssNews{
					Publication: rssNewsPublication{
						Name:     strings.TrimSpace(site.SiteName),
						Language: "id",
					},
					PublicationDate: p.UpdatedAt.UTC().Format(time.RFC3339),
					Title:           strings.TrimSpace(p.Title),
				},
			})
		}

		feed := rssFeed{
			Version:      "2.0",
			XMLNSDC:      "http://purl.org/dc/elements/1.1/",
			XMLNSSY:      "http://purl.org/rss/1.0/modules/syndication/",
			XMLNSAdmin:   "http://webns.net/mvcb/",
			XMLNSRDF:     "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
			XMLNSMedia:   "http://search.yahoo.com/mrss/",
			XMLNSContent: "http://purl.org/rss/1.0/modules/content/",
			XMLNSAtom:    "http://www.w3.org/2005/Atom",
			XMLNSNews:    "http://www.google.com/schemas/sitemap-news/0.9",
			Channel: rssChannel{
				Title:       strings.TrimSpace(site.SiteName),
				Link:        baseURL + "/rss/" + category.Slug,
				Description: strings.TrimSpace(site.SiteName) + " & Title: " + category.Name,
				DCLanguage:  "id-ID",
				DCRights:    "Copyright " + strconv.Itoa(time.Now().Year()) + " " + strings.TrimSpace(site.SiteName) + " & All Rights Reserved.",
				AtomLinks: []rssAtomLink{
					{Href: baseURL + "/rss/" + category.Slug, Rel: "self", Type: "application/rss+xml"},
					{Href: "https://pubsubhubbub.appspot.com/", Rel: "hub"},
				},
				Items: items,
			},
		}

		var buf bytes.Buffer
		buf.WriteString(xml.Header)
		enc := xml.NewEncoder(&buf)
		enc.Indent("", "  ")
		if err := enc.Encode(feed); err != nil {
			c.String(http.StatusInternalServerError, "rss encode error: %v", err)
			return
		}
		if err := enc.Flush(); err != nil {
			c.String(http.StatusInternalServerError, "rss flush error: %v", err)
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", buf.Bytes())
	})

	// LOGIN (follows site setting; if set to /masuk then /login is not registered)
	clientLoginPage := func(c *gin.Context) {
		tmpl, err := template.ParseFiles(
			"public/templates/layouts/client/app.html",
			"public/templates/layouts/client/header.html",
			"public/templates/layouts/client/footer.html",
			"public/templates/pages/auth/login.html",
		)
		if err != nil {
			c.String(http.StatusInternalServerError, "template error: %v", err)
			return
		}

		siteMeta := loadSiteMeta(db)
		siteSetting := loadSiteSetting(db)
		loginHref := entity.EffectiveLoginHref(siteSetting.LoginURL)
		data := gin.H{
			"title":                "Login",
			"meta_title":           "Login" + clientBrowserTitleSuffix(siteSetting),
			"meta_description":     siteMeta.DefaultDesc,
			"meta_keywords":        siteMeta.DefaultKeywords,
			"meta_image":           siteMeta.DefaultImage,
			"site_name":            siteSetting.SiteName,
			"tagline":              strings.TrimSpace(siteSetting.Tagline),
			"homepage_layout":      entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
			"footer_copy":          siteSetting.FooterCopy,
			"logo_url":             siteSetting.LogoURL,
			"favicon_url":          siteSetting.FaviconURL,
			"theme_color":          themeColorMeta(siteSetting),
			"accent_color":         siteSetting.AccentColor,
			"background_primary":   siteSetting.BackgroundPrimary,
			"background_secondary": siteSetting.BackgroundSecondary,
			"text_primary":         siteSetting.TextPrimary,
			"text_secondary":       siteSetting.TextSecondary,
			"border_color":         siteSetting.BorderColor,
			"client_pages":         loadClientMenuPages(db, 0),
			"nav_categories":       loadNavCategories(db, siteSetting.HomepageLayout),
			"login_href":           loginHref,
			"static_version":       clientStaticVersion,
		}
		for k, v := range clientSocialLinksH(siteSetting) {
			data[k] = v
		}
		injectClientVerificationMeta(db, data)
		injectClientAbsoluteAssetURLs(c, siteSetting, data)

		if err := tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
			return
		}
	}
	loginHref := entity.EffectiveLoginHref(loadSiteSetting(db).LoginURL)
	lowLoginHref := strings.ToLower(strings.TrimSpace(loginHref))
	if !strings.HasPrefix(lowLoginHref, "http://") && !strings.HasPrefix(lowLoginHref, "https://") {
		r.GET(loginHref, clientLoginPage)
	}

	// ADMIN
	admin := r.Group("/admin")
	admin.Use(middleware.AdminPageAuthMiddleware(jwtMgr, db))
	{
		// DASHBOARD
		admin.GET("/dashboard", func(c *gin.Context) {
			tmpl, err := template.ParseFiles(
				"public/templates/layouts/admin/app.html",
				"public/templates/layouts/admin/navbar.html",
				"public/templates/layouts/admin/sidebar.html",
				"public/templates/pages/admin/dashboard.html",
			)
			if err != nil {
				c.String(http.StatusInternalServerError, "template error: %v", err)
				return
			}

			data := adminDashboardPage(db)

			if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
				return
			}
		})

		// PROFILE
		admin.GET("/profile", func(c *gin.Context) {
			tmpl, err := template.ParseFiles(
				"public/templates/layouts/admin/app.html",
				"public/templates/layouts/admin/navbar.html",
				"public/templates/layouts/admin/sidebar.html",
				"public/templates/pages/admin/profile/index.html",
			)
			if err != nil {
				c.String(http.StatusInternalServerError, "template error: %v", err)
				return
			}
			data := adminSitePage(db, "Profile", "profile", nil)
			if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
				return
			}
		})

		adminOnly := admin.Group("")
		adminOnly.Use(middleware.AdminPageRoleMiddleware(db, "admin"))
		{
			// LIST ALL USERS
			adminOnly.GET("/users", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/user/create.html",
					"public/templates/pages/admin/user/edit.html",
					"public/templates/pages/admin/user/delete.html",
					"public/templates/pages/admin/user/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "User", "users", CrudListView(
					"users",
					"Semua User",
					"Tambah User",
					"bi-person-plus",
					"#modalUserCreate",
					[]gin.H{
						{"label": "User"},
						{"label": "Nama pengguna"},
						{"label": "Peran"},
						{"label": "Status"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// CREATE PAGE
			adminOnly.GET("/pages/create", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/page/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPageFormPage(db, "Tambah Page", "create", 0)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT PAGE
			adminOnly.GET("/pages/:id/edit", func(c *gin.Context) {
				id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
				if err != nil || id64 == 0 {
					redirectHome(c)
					return
				}
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/page/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPageFormPage(db, "Edit Page", "edit", uint(id64))
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LIST ALL PAGES
			adminOnly.GET("/pages", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/page/delete.html",
					"public/templates/pages/admin/page/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Page", "pages", CrudListViewWithCreateLink(
					"pages",
					"Semua Page",
					"Tambah Page",
					"bi-folder-plus",
					"/admin/pages/create",
					[]gin.H{
						{"label": "Page"},
						{"label": "Slug"},
						{"label": "Status"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))
				data["admin_pages_subnav"] = "list"

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LIST ALL CATEGORIES
			adminOnly.GET("/categories", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/category/create.html",
					"public/templates/pages/admin/category/edit.html",
					"public/templates/pages/admin/category/delete.html",
					"public/templates/pages/admin/category/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Category", "categories", CrudListView(
					"categories",
					"Semua Category",
					"Tambah Category",
					"bi-folder-plus",
					"#modalCategoryCreate",
					[]gin.H{
						{"label": "Category"},
						{"label": "Urutan"},
						{"label": "Slug"},
						{"label": "Judul"},
						{"label": "Kata kunci"},
						{"label": "Deskripsi"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT SITE SETTINGS
			adminOnly.GET("/site-settings", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/site_setting/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSiteSettingsPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT AI SETTINGS
			adminOnly.GET("/ai-setting", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/ai_setting/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminAISettingPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// RESTORE & BACKUP DATABASE
			adminOnly.GET("/restore-backup", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/backup/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminSitePage(db, "Restore & Backup", "settings", nil)
				data["admin_settings_subnav"] = "backup"
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT SCRAPER SETTINGS
			adminOnly.GET("/scraper-settings", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/scraper_setting/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminScraperSettingsPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// SCRAPER LOGS (read-only viewer)
			adminOnly.GET("/scraper-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/scraper_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminScraperLogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LOGIN LOGS (read-only audit)
			adminOnly.GET("/login-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/login_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminLoginLogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})
		}

		writerOrAdmin := admin.Group("")
		writerOrAdmin.Use(middleware.AdminPageRoleMiddleware(db, "admin", "writer"))
		{
			// LIST ALL TAGS
			writerOrAdmin.GET("/tags", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/tag/create.html",
					"public/templates/pages/admin/tag/edit.html",
					"public/templates/pages/admin/tag/delete.html",
					"public/templates/pages/admin/tag/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Tag", "tags", CrudListView(
					"tags",
					"Tag",
					"Tambah tag",
					"bi-tag-fill",
					"#modalTagCreate",
					[]gin.H{
						{"label": "Tag"},
						{"label": "Slug"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			postListColumns := []gin.H{
				{"label": "Post"},
				{"label": "Status"},
				{"label": "Kategori"},
				{"label": "Penulis"},
				{"label": "Diperbarui"},
				{"label": "Aksi", "class": "text-end"},
			}

			// CREATE POST
			writerOrAdmin.GET("/posts/create", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/post/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPostFormPage(db, c, "Tambah post", "create", 0)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/bulk-posts", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/post/bulk_post.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminBulkPostPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/bulk-post-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/bulk_post_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminBulkPostLogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/manual-post-ai-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/manual_post_ai_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminManualPostAILogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/posts/bulk", func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, "/admin/bulk-posts")
			})

			// EDIT POST
			writerOrAdmin.GET("/posts/:id/edit", func(c *gin.Context) {
				id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
				if err != nil || id64 == 0 {
					redirectHome(c)
					return
				}
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/post/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPostFormPage(db, c, "Edit post", "edit", uint(id64))
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LIST ALL POSTS
			writerOrAdmin.GET("/posts", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/post/delete.html",
					"public/templates/pages/admin/post/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Post", "posts", CrudListViewWithCreateLink(
					"posts",
					"Post",
					"Tambah post",
					"bi-file-earmark-plus",
					"/admin/posts/create",
					postListColumns,
				))
				data["admin_posts_subnav"] = "list"

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})
		}
	}

	// Tiga segmen: artikel di subkategori (/induk/sub/judul) bila article_slug_mode=category_nested.
	// Wildcard names must match the two-segment route at each depth (:slug, :postSlug) for Gin's radix tree.
	r.GET("/:slug/:postSlug/:articleSlug", func(c *gin.Context) {
		parentSlug := strings.TrimSpace(c.Param("slug"))
		childSlug := strings.TrimSpace(c.Param("postSlug"))
		postSlug := strings.TrimSpace(c.Param("articleSlug"))
		if parentSlug == "" || childSlug == "" || postSlug == "" {
			redirectHome(c)
			return
		}
		if clientReservedPublicPathSegment(strings.ToLower(parentSlug)) {
			redirectHome(c)
			return
		}
		ss := loadSiteSetting(db)
		slugMode := entity.NormalizeArticleSlugMode(ss.ArticleSlugMode)
		child, err := findSubcategoryBySlugs(db, parentSlug, childSlug)
		if err != nil {
			redirectHome(c)
			return
		}
		var post entity.Post
		if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
			Where("slug = ? AND category_id = ?", postSlug, child.ID).
			Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err != nil {
			var alt entity.Post
			if err2 := db.Preload("Category").Preload("Category.Parent").Where("slug = ?", postSlug).
				Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&alt).Error; err2 == nil {
				c.Redirect(http.StatusMovedPermanently, clientPostPublicPath(ss, alt))
				return
			}
			redirectHome(c)
			return
		}
		canon := clientPostPublicPath(ss, post)
		if strings.TrimSpace(c.Request.URL.Path) != canon {
			c.Redirect(http.StatusMovedPermanently, canon)
			return
		}
		if slugMode != entity.ArticleSlugModeCategoryNested {
			redirectHome(c)
			return
		}
		if err := serveClientArticlePage(c, db, &post); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
		}
	})

	// Dua segmen: artikel /{kategori}/{post} atau halaman subkategori /{induk}/{sub}.
	r.GET("/:slug/:postSlug", func(c *gin.Context) {
		catSlug := strings.TrimSpace(c.Param("slug"))
		postSlug := strings.TrimSpace(c.Param("postSlug"))
		if catSlug == "" || postSlug == "" {
			redirectHome(c)
			return
		}
		if clientReservedPublicPathSegment(strings.ToLower(catSlug)) {
			redirectHome(c)
			return
		}
		ss := loadSiteSetting(db)
		slugMode := entity.NormalizeArticleSlugMode(ss.ArticleSlugMode)

		servePostWithCanonicalRedirect := func(post entity.Post) bool {
			canon := clientPostPublicPath(ss, post)
			if strings.TrimSpace(c.Request.URL.Path) != canon {
				c.Redirect(http.StatusMovedPermanently, canon)
				return true
			}
			if err := serveClientArticlePage(c, db, &post); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
			}
			return true
		}

		// Mode category: artikel di kategori/subkategori (/mbg/judul) — cek dulu agar tidak tertukar dengan listing /induk/sub.
		if slugMode == entity.ArticleSlugModeCategory {
			var cat entity.Category
			if err := db.Preload("Parent").Where("slug = ?", catSlug).First(&cat).Error; err == nil {
				var post entity.Post
				if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
					Where("slug = ? AND category_id = ?", postSlug, cat.ID).
					Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
					if servePostWithCanonicalRedirect(post) {
						return
					}
				}
			}
		}

		if child, err := findSubcategoryBySlugs(db, catSlug, postSlug); err == nil {
			if err := serveClientCategoryPage(c, db, child); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
			}
			return
		}

		if !entity.ArticleSlugModeUsesCategoryPrefix(ss.ArticleSlugMode) {
			var post entity.Post
			if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
				Where("slug = ?", postSlug).Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
				c.Redirect(http.StatusMovedPermanently, clientPostPublicPath(ss, post))
				return
			}
			redirectHome(c)
			return
		}

		if slugMode == entity.ArticleSlugModeCategoryNested {
			var cat entity.Category
			if err := db.Preload("Parent").Where("slug = ?", catSlug).First(&cat).Error; err == nil {
				var post entity.Post
				if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
					Where("slug = ? AND category_id = ?", postSlug, cat.ID).
					Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
					if servePostWithCanonicalRedirect(post) {
						return
					}
				}
			}
		}

		var alt entity.Post
		if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
			Where("slug = ?", postSlug).
			Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&alt).Error; err == nil {
			c.Redirect(http.StatusMovedPermanently, clientPostPublicPath(ss, alt))
			return
		}
		redirectHome(c)
	})

	// Satu segmen: halaman statis (SSR) jika ada Page terbit; jika tidak, SPA (kategori / artikel).
	r.GET("/:slug", func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" {
			redirectHome(c)
			return
		}
		ss := loadSiteSetting(db)
		if lh := entity.EffectiveLoginHref(ss.LoginURL); lh != "/login" &&
			!strings.HasPrefix(strings.ToLower(lh), "http://") &&
			!strings.HasPrefix(strings.ToLower(lh), "https://") {
			inner := strings.Trim(lh, "/")
			if !strings.Contains(inner, "/") && inner != "" && strings.EqualFold(inner, slug) {
				clientLoginPage(c)
				return
			}
		}
		if clientReservedPublicPathSegment(strings.ToLower(slug)) {
			redirectHome(c)
			return
		}
		var page entity.Page
		if err := db.Where("slug = ? AND status = ?", slug, "published").First(&page).Error; err == nil {
			if err := serveClientStaticPage(c, db, &page); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
			}
			return
		}
		// If page slug exists but is not published, keep URL dead on public site.
		var pageAny entity.Page
		if err := db.Select("id").Where("slug = ?", slug).First(&pageAny).Error; err == nil {
			redirectHome(c)
			return
		}
		var cat entity.Category
		// Load full category row (title, keyword, description) for SEO on serveClientCategoryPage.
		if err := db.Preload("Parent").Where("slug = ?", slug).First(&cat).Error; err == nil {
			if !cat.IsRoot() {
				c.Redirect(http.StatusMovedPermanently, entity.CategoryPublicWebPath(cat))
				return
			}
			if err := serveClientCategoryPage(c, db, &cat); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
			}
			return
		}
		var post entity.Post
		if err := db.Preload("Author").Preload("Category").Preload("Tags").
			Where("slug = ?", slug).Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
			if canon := clientPostPublicPath(ss, post); strings.TrimSpace(c.Request.URL.Path) != canon {
				c.Redirect(http.StatusMovedPermanently, canon)
				return
			}
			if err := serveClientArticlePage(c, db, &post); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
			}
			return
		}
		// If slug exists but is not published (e.g. draft), treat as dead URL on public site.
		var postAny entity.Post
		if err := db.Select("id").Where("slug = ?", slug).First(&postAny).Error; err == nil {
			redirectHome(c)
			return
		}
		redirectHome(c)
	})

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if strings.HasPrefix(p, "/static/") || p == "/health" || p == "/sitemap.xml" || strings.HasPrefix(p, "/rss/") {
			redirectHome(c)
			return
		}
		redirectHome(c)
	})
	r.NoMethod(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
			return
		}
		serveClientErrorPage(c, db, http.StatusMethodNotAllowed, "Metode tidak didukung", "Permintaan tidak dapat diproses dengan metode ini.")
	})
}

// formatIntDecimalGrouping renders n with comma thousands separators (e.g. 1234567 -> "1,234,567").
func formatIntDecimalGrouping(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	u := uint64(n)
	if neg {
		u = uint64(-(n + 1)) + 1
	}
	s := strconv.FormatUint(u, 10)
	l := len(s)
	first := l % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.Grow(l + l/3)
	b.WriteString(s[:first])
	for i := first; i < l; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	out := b.String()
	if neg {
		return "-" + out
	}
	return out
}

// adminDashboardPage builds data for the admin home dashboard (stats + meta).
func adminDashboardPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Dashboard"

	var nUsers, nPosts, nCategories, nTags, nPublished, nDraft, nScheduled int64
	var nPages, nScraperLogs, nLoginLogs, nBulkPostLogs, nManualPostAILogs int64
	_ = db.Model(&entity.User{}).Count(&nUsers).Error
	_ = db.Model(&entity.Post{}).Count(&nPosts).Error
	_ = db.Model(&entity.Category{}).Count(&nCategories).Error
	_ = db.Model(&entity.Tag{}).Count(&nTags).Error
	_ = db.Model(&entity.Post{}).Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).Count(&nPublished).Error
	_ = db.Model(&entity.Post{}).Where("status = ?", "draft").Count(&nDraft).Error
	_ = db.Model(&entity.Post{}).Where("status = ?", "scheduled").Count(&nScheduled).Error
	_ = db.Model(&entity.Page{}).Count(&nPages).Error
	_ = db.Model(&entity.ScraperLog{}).Count(&nScraperLogs).Error
	_ = db.Model(&entity.LoginLog{}).Count(&nLoginLogs).Error
	_ = db.Model(&entity.BulkPostLog{}).Count(&nBulkPostLogs).Error
	_ = db.Model(&entity.ManualPostAILog{}).Count(&nManualPostAILogs).Error

	publishedPct, draftPct := 0, 0
	if nPosts > 0 {
		publishedPct = int(nPublished * 100 / nPosts)
		draftPct = int(nDraft * 100 / nPosts)
	}

	return gin.H{
		"title":                        title,
		"admin_nav":                    "dashboard",
		"admin_settings_subnav":        "",
		"admin_pages_subnav":           "",
		"admin_posts_subnav":           "",
		"meta_title":                   title + " - " + siteMeta.DefaultTitle,
		"meta_description":             siteMeta.DefaultDesc,
		"meta_keywords":                siteMeta.DefaultKeywords,
		"meta_image":                   siteMeta.DefaultImage,
		"site_name":                    siteSetting.SiteName,
		"logo_url":                     siteSetting.LogoURL,
		"favicon_url":                  siteSetting.FaviconURL,
		"theme_color":                  themeColorMeta(siteSetting),
		"asset_version":                adminAssetQueryVersion(),
		"stat_users":                   nUsers,
		"stat_users_fmt":               formatIntDecimalGrouping(nUsers),
		"stat_posts":                   nPosts,
		"stat_posts_fmt":               formatIntDecimalGrouping(nPosts),
		"stat_categories":              nCategories,
		"stat_categories_fmt":          formatIntDecimalGrouping(nCategories),
		"stat_tags":                    nTags,
		"stat_tags_fmt":                formatIntDecimalGrouping(nTags),
		"stat_pages":                   nPages,
		"stat_pages_fmt":               formatIntDecimalGrouping(nPages),
		"stat_scraper_logs":            nScraperLogs,
		"stat_scraper_logs_fmt":        formatIntDecimalGrouping(nScraperLogs),
		"stat_login_logs":              nLoginLogs,
		"stat_login_logs_fmt":          formatIntDecimalGrouping(nLoginLogs),
		"stat_bulk_post_logs":          nBulkPostLogs,
		"stat_bulk_post_logs_fmt":      formatIntDecimalGrouping(nBulkPostLogs),
		"stat_manual_post_ai_logs":     nManualPostAILogs,
		"stat_manual_post_ai_logs_fmt": formatIntDecimalGrouping(nManualPostAILogs),
		"stat_published":               nPublished,
		"stat_published_fmt":           formatIntDecimalGrouping(nPublished),
		"stat_drafts":                  nDraft,
		"stat_drafts_fmt":              formatIntDecimalGrouping(nDraft),
		"stat_scheduled":               nScheduled,
		"stat_scheduled_fmt":           formatIntDecimalGrouping(nScheduled),
		"stat_published_pct":           publishedPct,
		"stat_draft_pct":               draftPct,
		"login_href":                   entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
}

// adminSiteSettingsPage builds data for the site settings edit form (singleton).
func adminSiteSettingsPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	siteCloudStorage := loadSiteCloudStorage(db)
	title := "Site settings"

	var sc entity.SiteConnection
	_ = db.First(&sc).Error

	ct := entity.NormalizeColorTheme(strings.TrimSpace(siteSetting.ColorTheme))
	sidebarLimits := sidebarListLimitFields(siteSetting)
	return gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "site",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteMeta.DefaultTitle,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"color_theme_custom":    ct == entity.ColorThemeCustom,
		"setting": gin.H{
			"site_name":                           siteSetting.SiteName,
			"footer_copy":                         siteSetting.FooterCopy,
			"tagline":                             siteSetting.Tagline,
			"logo_url":                            siteSetting.LogoURL,
			"favicon_url":                         siteSetting.FaviconURL,
			"theme_color":                         themeColorMeta(siteSetting),
			"text_primary":                        siteSetting.TextPrimary,
			"text_secondary":                      siteSetting.TextSecondary,
			"background_primary":                  siteSetting.BackgroundPrimary,
			"background_secondary":                siteSetting.BackgroundSecondary,
			"primary_button":                      siteSetting.PrimaryButton,
			"secondary_button":                    siteSetting.SecondaryButton,
			"accent_color":                        siteSetting.AccentColor,
			"border_color":                        siteSetting.BorderColor,
			"font_family":                         siteSetting.FontFamily,
			"content_font_size":                   siteSetting.ContentFontSize,
			"sidebar_font_size":                   siteSetting.SidebarFontSize,
			"sidebar_list_limit":                  sidebarLimits["sidebar_list_limit"],
			"sidebar_list_limit_popular":          sidebarLimits["sidebar_list_limit_popular"],
			"sidebar_list_limit_latest":           sidebarLimits["sidebar_list_limit_latest"],
			"image_caption_enabled":               siteSetting.ImageCaptionEnabled,
			"image_caption_mode":                  siteSetting.ImageCaptionMode,
			"image_caption_template":              siteSetting.ImageCaptionTemplate,
			"image_resize_enabled":                siteSetting.ImageResizeEnabled,
			"img_resize_enabled":                  siteSetting.ImgResizeEnabled,
			"img_resize_width":                    siteSetting.ImgResizeWidth,
			"img_resize_height":                   siteSetting.ImgResizeHeight,
			"img_resize_quality":                  siteSetting.ImgResizeQuality,
			"image_resize_width":                  siteSetting.ImageResizeWidth,
			"image_resize_quality":                siteSetting.ImageResizeQuality,
			"ads_lazy_load_enabled":               siteSetting.AdsLazyLoadEnabled,
			"stored_image_max_width":              siteSetting.StoredImageMaxWidth,
			"stored_image_quality":                siteSetting.StoredImageQuality,
			"avatar_resize_width":                 siteSetting.AvatarResizeWidth,
			"avatar_resize_quality":               siteSetting.AvatarResizeQuality,
			"not_found_redirect_mode":             entity.NormalizeNotFoundRedirectMode(strings.TrimSpace(siteSetting.NotFoundRedirectMode)),
			"default_title":                       siteSetting.DefaultTitle,
			"default_description":                 siteSetting.DefaultDesc,
			"default_image":                       siteSetting.DefaultImage,
			"default_keywords":                    siteSetting.DefaultKeywords,
			"homepage_layout":                     entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
			"color_theme":                         ct,
			"header_script":                       siteSetting.HeaderScript,
			"footer_script":                       siteSetting.FooterScript,
			"ads_top":                             siteSetting.AdsTop,
			"ads_center":                          siteSetting.AdsCenter,
			"ads_bottom":                          siteSetting.AdsBottom,
			"article_ads_json":                    siteSetting.ArticleAdsJSON,
			"social_facebook_url":                 siteSetting.SocialFacebookURL,
			"social_x_url":                        siteSetting.SocialXURL,
			"social_instagram_url":                siteSetting.SocialInstagramURL,
			"social_youtube_url":                  siteSetting.SocialYouTubeURL,
			"social_tiktok_url":                   siteSetting.SocialTikTokURL,
			"social_rss_url":                      siteSetting.SocialRSSURL,
			"login_url":                           siteSetting.LoginURL,
			"article_slug_mode":                   entity.NormalizeArticleSlugMode(strings.TrimSpace(siteSetting.ArticleSlugMode)),
			"article_schema_type":                 entity.NormalizeArticleSchemaType(strings.TrimSpace(siteSetting.ArticleSchemaType)),
			"scraper_auto_tag":                    siteSetting.ScraperAutoTag,
			"auto_href_enabled":                   siteSetting.AutoHrefEnabled,
			"also_read_enabled":                   siteSetting.AlsoReadEnabled,
			"also_read_max_inserts":               entity.EffectiveAlsoReadMaxInserts(siteSetting.AlsoReadMaxInserts),
			"post_date_refresh_enabled":           siteSetting.PostDateRefreshEnabled,
			"post_date_refresh_days":              siteSetting.PostDateRefreshDays,
			"post_date_interval_mins":             siteSetting.PostDateIntervalMins,
			"post_date_url_recycle_enabled":       siteSetting.PostDateUrlRecycleEnabled,
			"post_date_url_recycle_interval_mins": siteSetting.PostDateUrlRecycleIntervalMins,
			"source_disclaimer_mode":              siteSetting.SourceDisclaimerMode,
			"source_disclaimer_template":          siteSetting.SourceDisclaimerTemplate,
		},
		"cloud_storage": gin.H{
			"r2_account_id":        siteCloudStorage.R2AccountID,
			"r2_access_key_id":     siteCloudStorage.R2AccessKeyID,
			"r2_secret_access_key": siteCloudStorage.R2SecretAccessKey,
			"r2_bucket":            siteCloudStorage.R2Bucket,
			"r2_public_base_url":   siteCloudStorage.R2PublicBaseURL,
			"r2_prefix":            siteCloudStorage.R2Prefix,
		},
		"connections": gin.H{
			"ahrefs":                  sc.Ahrefs,
			"google":                  sc.Google,
			"bing":                    sc.Bing,
			"yandex":                  sc.Yandex,
			"auto_post_enabled":       sc.AutoPostEnabled,
			"auto_post_template":      sc.AutoPostTemplate,
			"facebook_enabled":        sc.FacebookEnabled,
			"facebook_client_id":      sc.FacebookClientID,
			"facebook_client_secret":  sc.FacebookClientSecret,
			"facebook_token_url":      sc.FacebookTokenURL,
			"facebook_publish_url":    sc.FacebookPublishURL,
			"facebook_scope":          sc.FacebookScope,
			"facebook_account_id":     sc.FacebookAccountID,
			"x_enabled":               sc.XEnabled,
			"x_client_id":             sc.XClientID,
			"x_client_secret":         sc.XClientSecret,
			"x_token_url":             sc.XTokenURL,
			"x_publish_url":           sc.XPublishURL,
			"x_scope":                 sc.XScope,
			"x_account_id":            sc.XAccountID,
			"tiktok_enabled":          sc.TiktokEnabled,
			"tiktok_client_id":        sc.TiktokClientID,
			"tiktok_client_secret":    sc.TiktokClientSecret,
			"tiktok_token_url":        sc.TiktokTokenURL,
			"tiktok_publish_url":      sc.TiktokPublishURL,
			"tiktok_scope":            sc.TiktokScope,
			"tiktok_account_id":       sc.TiktokAccountID,
			"instagram_enabled":       sc.InstagramEnabled,
			"instagram_client_id":     sc.InstagramClientID,
			"instagram_client_secret": sc.InstagramClientSecret,
			"instagram_token_url":     sc.InstagramTokenURL,
			"instagram_publish_url":   sc.InstagramPublishURL,
			"instagram_scope":         sc.InstagramScope,
			"instagram_account_id":    sc.InstagramAccountID,
		},
	}
}

// adminAISettingPage builds template data for the AI settings admin page.
func adminAISettingPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "AI settings"
	aiSetting := loadAISetting(db)
	return gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "ai",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_setting":            aiSetting,
	}
}

// adminLogRetentionHelp documents the shared rolling window for admin audit tables (env ADMIN_LOG_RETENTION_HOURS).
func adminLogRetentionHelp(cfg config.Config) string {
	h := cfg.AdminLogRetentionHours
	if h <= 0 {
		return "Pembersihan otomatis nonaktif (ADMIN_LOG_RETENTION_HOURS <= 0). Log tetap tersimpan sampai Anda menghapusnya manual."
	}
	if h == 1 {
		return "Baris lebih lama dari 1 jam otomatis dihapus dari log scraper, login, bulk post, dan AI post manual (ADMIN_LOG_RETENTION_HOURS). Pembersihan berjalan sekitar 15 menit sekali."
	}
	return fmt.Sprintf("Baris lebih lama dari %d jam otomatis dihapus dari log scraper, login, bulk post, dan AI post manual (ADMIN_LOG_RETENTION_HOURS). Pembersihan berjalan sekitar 15 menit sekali.", h)
}

func adminLoginLogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log login"
	crud := CrudListViewReadOnly("login-logs", "Log login", []gin.H{
		{"label": "Waktu"},
		{"label": "Email"},
		{"label": "Hasil"},
		{"label": "Pengguna"},
		{"label": "Alasan"},
		{"label": "IP"},
		{"label": "User agent"},
	})
	crud["card_subtitle"] = "Percobaan masuk (berhasil dan gagal). Waktu memakai zona waktu lokal browser Anda. " + adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "login-logs-refresh"
	crud["clear_logs_btn_id"] = "login-logs-clear"
	crud["success_filter_id"] = "login-logs-success-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "login",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func adminScraperLogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log scraper"
	crud := CrudListViewReadOnly("scr-logs", "Log scraper", []gin.H{
		{"label": "Waktu"},
		{"label": "Level"},
		{"label": "AI dipakai"},
		{"label": "Pesan"},
		{"label": "URL"},
		{"label": "Kategori"},
	})
	crud["card_subtitle"] = adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "scr-logs-refresh"
	crud["clear_logs_btn_id"] = "scr-logs-clear"
	crud["level_filter_id"] = "scr-logs-level-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "scraper",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func adminScraperSettingsPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Scraper settings"
	aiOK := aiConfigured(db)
	var s entity.ScraperSetting
	_ = db.Order("id desc").First(&s).Error
	var categories []entity.Category
	_ = db.Order("name asc").Find(&categories).Error
	// s.ScraperAIProvider = siteSetting.ScraperAIProvider
	// s.ScraperPromptTitle = siteSetting.ScraperPromptTitle
	// s.ScraperPromptContent = siteSetting.ScraperPromptContent
	return gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "scraper",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"accent_color":          siteSetting.AccentColor,
		"background_primary":    siteSetting.BackgroundPrimary,
		"background_secondary":  siteSetting.BackgroundSecondary,
		"text_primary":          siteSetting.TextPrimary,
		"text_secondary":        siteSetting.TextSecondary,
		"border_color":          siteSetting.BorderColor,
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_configured":         aiOK,
		"scraper_setting":       s,
		"categories":            categories,
	}
}

// adminPostGeneratePage builds template data for the AI generate-post admin page.
func adminPostGeneratePage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Buat post"
	aiOK := aiConfigured(db)
	return gin.H{
		"title":                 title,
		"admin_nav":             "posts",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "generate",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_configured":         aiOK,
	}
}

// adminPageFormPage builds template data for full-page create/edit static pages.
func adminPageFormPage(db *gorm.DB, title, formMode string, pageID uint) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	subnav := ""
	switch formMode {
	case "create":
		subnav = "create"
	case "edit":
		subnav = "edit"
	}
	return gin.H{
		"title":                 title,
		"admin_nav":             "pages",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    subnav,
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"page_form_mode":        formMode,
		"page_id":               pageID,
	}
}

// adminPostFormPage builds template data for full-page create/edit post forms.
func adminPostFormPage(db *gorm.DB, c *gin.Context, title, formMode string, postID uint) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	aiOK := aiConfigured(db)
	subnav := ""
	switch formMode {
	case "create":
		subnav = "create"
	case "edit":
		subnav = "edit"
	}
	var requesterID uint
	if uid, ok := c.Get("userID"); ok {
		if v, ok := uid.(uint); ok {
			requesterID = v
		}
	}
	requesterRole := ""
	if r, ok := c.Get("userRole"); ok {
		if s, ok := r.(string); ok {
			requesterRole = strings.ToLower(strings.TrimSpace(s))
		}
	}
	var authors []entity.User
	_ = db.Where("role IN ? AND enabled = ?", []string{"admin", "writer"}, true).
		Order("name asc").Find(&authors).Error
	var aiRow entity.AISetting
	_ = db.First(&aiRow).Error
	promptName1 := strings.TrimSpace(aiRow.GeneratePromptContentName)
	if promptName1 == "" {
		promptName1 = "Prompt content 1"
	}
	promptName2 := strings.TrimSpace(aiRow.GeneratePromptContentName2)
	if promptName2 == "" {
		promptName2 = "Prompt content 2"
	}
	promptName3 := strings.TrimSpace(aiRow.GeneratePromptContentName3)
	if promptName3 == "" {
		promptName3 = "Prompt content 3"
	}
	return gin.H{
		"title":                    title,
		"admin_nav":                "posts",
		"admin_settings_subnav":    "",
		"admin_pages_subnav":       "",
		"admin_posts_subnav":       subnav,
		"meta_title":               title + " - " + siteSetting.SiteName,
		"meta_description":         siteMeta.DefaultDesc,
		"meta_keywords":            siteMeta.DefaultKeywords,
		"meta_image":               siteMeta.DefaultImage,
		"site_name":                siteSetting.SiteName,
		"logo_url":                 siteSetting.LogoURL,
		"favicon_url":              siteSetting.FaviconURL,
		"theme_color":              themeColorMeta(siteSetting),
		"asset_version":            adminAssetQueryVersion(),
		"login_href":               entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_configured":            aiOK,
		"post_form_mode":           formMode,
		"post_id":                  postID,
		"post_form_requester_id":   requesterID,
		"post_form_requester_role": requesterRole,
		"post_form_authors":        authors,
		"prompt_content_name_1":    promptName1,
		"prompt_content_name_2":    promptName2,
		"prompt_content_name_3":    promptName3,
	}
}

// adminBulkPostPage builds data for AI bulk post generation (template + suggest titles + generate).
func adminBulkPostPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Bulk post"
	aiOK := aiConfigured(db)
	var categories []entity.Category
	_ = db.Order("name asc").Find(&categories).Error
	var aiRow entity.AISetting
	_ = db.First(&aiRow).Error
	promptName1 := strings.TrimSpace(aiRow.GeneratePromptContentName)
	if promptName1 == "" {
		promptName1 = "Prompt content 1"
	}
	promptName2 := strings.TrimSpace(aiRow.GeneratePromptContentName2)
	if promptName2 == "" {
		promptName2 = "Prompt content 2"
	}
	promptName3 := strings.TrimSpace(aiRow.GeneratePromptContentName3)
	if promptName3 == "" {
		promptName3 = "Prompt content 3"
	}
	var authors []entity.User
	_ = db.Where("role IN ?", []string{"admin", "writer"}).Order("name asc").Find(&authors).Error
	var tags []entity.Tag
	_ = db.Order("name asc").Find(&tags).Error
	return gin.H{
		"title":                            title,
		"admin_nav":                        "bulk_posts",
		"admin_settings_subnav":            "",
		"admin_pages_subnav":               "",
		"admin_posts_subnav":               "",
		"meta_title":                       title + " - " + siteSetting.SiteName,
		"meta_description":                 siteMeta.DefaultDesc,
		"meta_keywords":                    siteMeta.DefaultKeywords,
		"meta_image":                       siteMeta.DefaultImage,
		"site_name":                        siteSetting.SiteName,
		"logo_url":                         siteSetting.LogoURL,
		"favicon_url":                      siteSetting.FaviconURL,
		"theme_color":                      themeColorMeta(siteSetting),
		"asset_version":                    adminAssetQueryVersion(),
		"ai_configured":                    aiOK,
		"generate_prompt_title":            aiRow.GeneratePromptTitle,
		"prompt_content_name_1":            promptName1,
		"generate_prompt_content":          aiRow.GeneratePromptContent,
		"prompt_content_name_2":            promptName2,
		"prompt_content_name_3":            promptName3,
		"generate_prompt_meta_description": aiRow.GeneratePromptMetaDescription,
		"generate_prompt_meta_keywords":    aiRow.GeneratePromptMetaKeywords,
		"bulk_cover_background_url":        aiRow.BulkCoverBackgroundURL,
		"categories":                       categories,
		"authors":                          authors,
		"tags":                             tags,
		"login_href":                       entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
}

func adminBulkPostLogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log bulk post"
	crud := CrudListViewReadOnly("bulk-post-logs", "Log bulk post", []gin.H{
		{"label": "Waktu"},
		{"label": "Pengguna"},
		{"label": "Level"},
		{"label": "Event"},
		{"label": "AI dipakai"},
		{"label": "Pesan"},
		{"label": "Kategori"},
		{"label": "Post"},
		{"label": "Judul"},
	})
	crud["card_subtitle"] = "Saran judul AI dan pembuatan batch. Penulis hanya melihat baris miliknya; admin melihat semua. " + adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "bulk-post-logs-refresh"
	crud["clear_logs_btn_id"] = "bulk-post-logs-clear"
	crud["level_filter_id"] = "bulk-post-logs-level-filter"
	crud["event_filter_id"] = "bulk-post-logs-event-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "bulk",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func adminManualPostAILogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log AI post manual"
	crud := CrudListViewReadOnly("manual-post-ai-logs", "Log AI post manual", []gin.H{
		{"label": "Waktu"},
		{"label": "Pengguna"},
		{"label": "Level"},
		{"label": "Event"},
		{"label": "AI diminta"},
		{"label": "AI dipakai"},
		{"label": "Pesan"},
		{"label": "Post"},
		{"label": "Judul"},
	})
	crud["card_subtitle"] = "Dicatat hanya saat form post manual memicu pembuatan AI (konten/gambar). Simpan/update manual tanpa generate tidak dicatat. Penulis hanya melihat baris miliknya; admin melihat semua. " + adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "manual-post-ai-logs-refresh"
	crud["clear_logs_btn_id"] = "manual-post-ai-logs-clear"
	crud["level_filter_id"] = "manual-post-ai-logs-level-filter"
	crud["event_filter_id"] = "manual-post-ai-logs-event-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "manual_post_ai",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

// adminSitePage builds gin.H for admin HTML pages that use partial admin-crud-list (.crud).
// navKey drives sidebar active state: dashboard, users, categories, posts, tags, settings.
func adminSitePage(db *gorm.DB, title, navKey string, crud gin.H) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	return gin.H{
		"title":                 title,
		"admin_nav":             navKey,
		"admin_logs_subnav":     "",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteMeta.DefaultTitle,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func executeAdminTemplate(c *gin.Context, tmpl *template.Template, db *gorm.DB, data gin.H) error {
	if data == nil {
		data = gin.H{}
	}
	ss := loadSiteSetting(db)
	data["article_slug_mode"] = entity.NormalizeArticleSlugMode(ss.ArticleSlugMode)
	injectAdminAbsoluteBaseURL(c, data)
	adminInjectCurrentUser(c, db, data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-admin-app", data)
}

func adminInjectCurrentUser(c *gin.Context, db *gorm.DB, data gin.H) {
	if c == nil || db == nil || data == nil {
		return
	}
	userIDAny, ok := c.Get("userID")
	if !ok {
		return
	}
	userID, ok := userIDAny.(uint)
	if !ok || userID == 0 {
		return
	}
	var u entity.User
	if err := db.Select("id, name, image").First(&u, userID).Error; err != nil {
		return
	}
	data["admin_user_name"] = strings.TrimSpace(u.Name)
	if u.Image != nil {
		data["admin_user_image"] = strings.TrimSpace(*u.Image)
	}
}

type siteMetaView struct {
	DefaultTitle    string
	DefaultDesc     string
	DefaultImage    string
	DefaultKeywords string
}

type sitemapIndex struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	Xmlns    string              `xml:"xmlns,attr"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

type sitemapIndexEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type urlSet struct {
	XMLName xml.Name      `xml:"urlset"`
	Xmlns   string        `xml:"xmlns,attr"`
	URLs    []urlSetEntry `xml:"url"`
}

type urlSetEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type newsSitemap struct {
	XMLName    xml.Name         `xml:"urlset"`
	Xmlns      string           `xml:"xmlns,attr"`
	XmlnsNews  string           `xml:"xmlns:news,attr"`
	URLSetRows []newsSitemapURL `xml:"url"`
}

type newsSitemapURL struct {
	Loc  string   `xml:"loc"`
	News newsMeta `xml:"news:news"`
}

type newsMeta struct {
	Publication     newsPublication `xml:"news:publication"`
	PublicationDate string          `xml:"news:publication_date"`
	Title           string          `xml:"news:title"`
}

type newsPublication struct {
	Name     string `xml:"news:name"`
	Language string `xml:"news:language"`
}

type rssFeed struct {
	XMLName      xml.Name   `xml:"rss"`
	XMLNSDC      string     `xml:"xmlns:dc,attr"`
	XMLNSSY      string     `xml:"xmlns:sy,attr"`
	XMLNSAdmin   string     `xml:"xmlns:admin,attr"`
	XMLNSRDF     string     `xml:"xmlns:rdf,attr"`
	XMLNSMedia   string     `xml:"xmlns:media,attr"`
	XMLNSContent string     `xml:"xmlns:content,attr"`
	XMLNSAtom    string     `xml:"xmlns:atom,attr"`
	XMLNSNews    string     `xml:"xmlns:news,attr,omitempty"`
	Version      string     `xml:"version,attr"`
	Channel      rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	DCLanguage  string        `xml:"dc:language"`
	DCRights    string        `xml:"dc:rights,omitempty"`
	AtomLinks   []rssAtomLink `xml:"atom:link"`
	Items       []rssItem     `xml:"item"`
}

type rssAtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
}

type rssItem struct {
	Title          string             `xml:"title"`
	Link           string             `xml:"link"`
	GUID           rssGUID            `xml:"guid"`
	Description    rssCDATA           `xml:"description"`
	ContentEncoded rssCDATA           `xml:"content:encoded"`
	MediaContent   *rssMediaContent   `xml:"media:content,omitempty"`
	MediaThumbnail *rssMediaThumbnail `xml:"media:thumbnail,omitempty"`
	PubDate        string             `xml:"pubDate,omitempty"`
	DCCreator      string             `xml:"dc:creator,omitempty"`
	MediaKeywords  string             `xml:"media:keywords,omitempty"`
	Category       string             `xml:"category,omitempty"`
	AtomLinks      []rssAtomLink      `xml:"atom:link,omitempty"`
	News           *rssNews           `xml:"news:news,omitempty"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr,omitempty"`
	Value       string `xml:",chardata"`
}

type rssCDATA struct {
	Value string `xml:",cdata"`
}

type rssMediaContent struct {
	URL    string         `xml:"url,attr,omitempty"`
	Length string         `xml:"length,attr,omitempty"`
	Medium string         `xml:"medium,attr,omitempty"`
	Type   string         `xml:"type,attr,omitempty"`
	Width  string         `xml:"width,attr,omitempty"`
	Height string         `xml:"height,attr,omitempty"`
	Title  *rssMediaTitle `xml:"media:title,omitempty"`
}

type rssMediaTitle struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

type rssMediaThumbnail struct {
	URL string `xml:"url,attr"`
}

type rssNews struct {
	Publication     rssNewsPublication `xml:"news:publication"`
	PublicationDate string             `xml:"news:publication_date"`
	Title           string             `xml:"news:title"`
}

type rssNewsPublication struct {
	Name     string `xml:"news:name"`
	Language string `xml:"news:language"`
}

// loadSiteMeta maps default meta values from the singleton site_settings row.
func loadSiteMeta(db *gorm.DB) siteMetaView {
	return siteMetaFromSetting(loadSiteSetting(db))
}

func siteMetaFromSetting(setting entity.SiteSetting) siteMetaView {
	return siteMetaView{
		DefaultTitle:    setting.DefaultTitle,
		DefaultDesc:     setting.DefaultDesc,
		DefaultImage:    setting.DefaultImage,
		DefaultKeywords: setting.DefaultKeywords,
	}
}

func canonicalBaseURL(c *gin.Context) string {
	proto := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		proto = "https"
	}
	return proto + "://" + c.Request.Host
}

// normalizedPublicSiteOrigin returns scheme://host from PUBLIC_SITE_URL when set (path/query ignored).
// Admin "view on site" links use this so the hostname matches the canonical public URL (e.g. apex vs www),
// consistent with social autopost (see internal/social/autoposter.go).
func normalizedPublicSiteOrigin() string {
	raw := strings.TrimSpace(os.Getenv("PUBLIC_SITE_URL"))
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func writeXMLResponse(c *gin.Context, payload any) {
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte(xml.Header))
	_, _ = c.Writer.Write([]byte(scraperguard.MarkerComment() + "\n"))
	enc := xml.NewEncoder(c.Writer)
	enc.Indent("", "  ")
	_ = enc.Encode(payload)
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func buildSEODescription(title, preview string) string {
	title = strings.TrimSpace(textPreview(title, 500))
	preview = strings.TrimSpace(textPreview(preview, 600))
	if preview == "" {
		return title
	}
	if title == "" {
		return preview
	}
	return title + ". " + preview
}

func detectImageMediaType(imageURL string) string {
	lower := strings.ToLower(strings.TrimSpace(imageURL))
	switch {
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	default:
		return "image/jpeg"
	}
}

// toClientAbsoluteURL prefixes site-relative paths with the public origin, including /img proxy paths.
func toClientAbsoluteURL(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		if strings.HasPrefix(strings.ToLower(baseURL), "https://") {
			return "https:" + raw
		}
		return "http:" + raw
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(raw, "/")
}

func toAbsoluteURL(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Keep /img proxy paths relative so the browser does not re-wrap them as nested /img?src=... proxies.
	if strings.HasPrefix(raw, media.ResizeProxyPath+"/") || strings.HasPrefix(raw, media.ResizeProxyPath+"?") {
		return raw
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if stripped := media.StripResizePrefixes(raw); stripped != raw {
			return toAbsoluteURL(baseURL, stripped)
		}
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		if strings.HasPrefix(strings.ToLower(baseURL), "https://") {
			return "https:" + raw
		}
		return "http:" + raw
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(raw, "/")
}

var indonesianMonths = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func slugifyTextPath(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "'", "")
	v = strings.ReplaceAll(v, "\"", "")
	v = slugifyNonAlnum.ReplaceAllString(v, "-")
	return strings.Trim(v, "-")
}

func authorPublicSlug(u entity.User) string {
	if s := slugifyTextPath(u.Username); s != "" {
		return s
	}
	em := strings.TrimSpace(u.Email)
	if em != "" {
		local := strings.SplitN(em, "@", 2)[0]
		if s := slugifyTextPath(local); s != "" {
			return s
		}
	}
	return ""
}

func authorPublicPathFromUser(u entity.User) string {
	if s := authorPublicSlug(u); s != "" {
		return "/profile/" + url.PathEscape(s)
	}
	return "/posts"
}

func normalizePostImgURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return s
	}
	if strings.HasPrefix(s, "/") {
		return s
	}
	return "/" + s
}

// imageResizeURL applies /img with quality only; width is chosen client-side from layout.
func imageResizeURL(siteSetting entity.SiteSetting, raw string) string {
	src := normalizePostImgURL(raw)
	if src == "" || src == BLANK_IMG {
		return src
	}
	if !siteSetting.ImgResizeEnabled {
		return media.StripResizePrefixes(src)
	}
	quality := strings.TrimSpace(siteSetting.ImgResizeQuality)
	return imageResizeURLWithOptions(src, "", quality)
}

func avatarResizeURL(siteSetting entity.SiteSetting, raw string) string {
	src := normalizePostImgURL(raw)
	if src == "" || src == BLANK_IMG {
		return src
	}
	if !siteSetting.ImgResizeEnabled {
		return media.StripResizePrefixes(src)
	}
	width := strings.TrimSpace(siteSetting.AvatarResizeWidth)
	quality := strings.TrimSpace(siteSetting.AvatarResizeQuality)
	return imageResizeURLWithOptions(src, width, quality)
}

func imageResizeURLWithOptions(src, width, quality string) string {
	if width == "" && quality == "" {
		return src
	}
	return media.BuildResizeURL(media.StripResizePrefixes(src), width, "", quality)
}

func firstImgSrcFromHTML(html string) string {
	m := firstImgSrcRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// postCoverRawURL returns the original cover image path/URL without /img or CDN resize wrappers.
func postCoverRawURL(image *string, content string) string {
	u := ""
	if image != nil {
		u = strings.TrimSpace(*image)
	}
	if u == "" {
		u = firstImgSrcFromHTML(content)
	}
	if u == "" {
		return ""
	}
	return media.StripResizePrefixes(normalizePostImgURL(u))
}

// clientMetaImageURL returns the original image as a full absolute URL for og/twitter/JSON-LD.
// Unlike toAbsoluteURL, this never keeps /img proxy paths relative.
func clientMetaImageURL(base, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = media.StripResizePrefixes(normalizePostImgURL(raw))
	if strings.HasPrefix(raw, media.ResizeProxyPath+"/") || strings.HasPrefix(raw, media.ResizeProxyPath+"?") {
		if inner, ok := media.ParseResizeProxyURL(raw); ok && inner != "" {
			raw = inner
		}
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if stripped := media.StripResizePrefixes(raw); stripped != raw {
			return clientMetaImageURL(base, stripped)
		}
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		if strings.HasPrefix(strings.ToLower(base), "https://") {
			return "https:" + raw
		}
		return "http:" + raw
	}
	if strings.HasPrefix(raw, media.ResizeProxyPath+"/") || strings.HasPrefix(raw, media.ResizeProxyPath+"?") {
		return ""
	}
	if base == "" {
		return raw
	}
	return base + "/" + strings.TrimLeft(raw, "/")
}

// normalizeClientSocialImageURLs strips resize wrappers and absolutizes social/JSON-LD image fields.
func normalizeClientSocialImageURLs(base string, data gin.H) {
	normalizeClientMetaImageField(base, data, "meta_image")
	normalizeClientMetaImageField(base, data, "publisher_image")
}

func normalizeClientMetaImageField(base string, data gin.H, key string) {
	if data == nil {
		return
	}
	var raw string
	switch v := data[key].(type) {
	case string:
		raw = v
	case template.HTML:
		raw = string(v)
	default:
		if v, ok := data[key]; ok && v != nil {
			raw = fmt.Sprint(v)
		}
	}
	if u := clientMetaImageURL(base, raw); u != "" {
		data[key] = u
	}
}

func postHeroURL(siteSetting entity.SiteSetting, image *string, content string) string {
	u := ""
	if image != nil {
		u = strings.TrimSpace(*image)
	}
	if u == "" {
		u = firstImgSrcFromHTML(content)
	}
	if u == "" {
		return ""
	}
	return imageResizeURL(siteSetting, u)
}

func postThumbURL(siteSetting entity.SiteSetting, image *string, content string) string {
	u := ""
	if image != nil {
		u = strings.TrimSpace(*image)
	}
	if u == "" {
		u = firstImgSrcFromHTML(content)
	}
	if u == "" {
		return BLANK_IMG
	}
	return imageResizeURL(siteSetting, u)
}

func postDisplayTimeRef(p entity.Post) *time.Time {
	if !p.UpdatedAt.IsZero() {
		t := p.UpdatedAt
		return &t
	}
	if !p.CreatedAt.IsZero() {
		t := p.CreatedAt
		return &t
	}
	return nil
}

// formatViewCountID renders a non-negative count with dot thousand separators ("1.234") plus " tayangan".
func formatViewCountID(n int64) string {
	if n <= 0 {
		return ""
	}
	s := strconv.FormatInt(n, 10)
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	var b strings.Builder
	for len(s) > 0 {
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		take := pre
		if take > len(s) {
			take = len(s)
		}
		b.WriteString(s[:take])
		s = s[take:]
		pre = 3
	}
	b.WriteString(" tayangan")
	return b.String()
}

func buildClientListPosts(siteSetting entity.SiteSetting, posts []entity.Post) []gin.H {
	rows := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}
		viewLabel := formatViewCountID(p.View)
		rows = append(rows, gin.H{
			"title":       title,
			"href":        clientPostPublicPath(siteSetting, p),
			"thumb":       postThumbURL(siteSetting, p.Image, p.Content),
			"time":        formatRelativeTimeID(postDisplayTimeRef(p)),
			"date_label":  formatPostDateCalendar(postDisplayTimeRef(p)),
			"has_view":    viewLabel != "",
			"view_label":  viewLabel,
			"author_name": authorDisplayName(p.Author),
			"excerpt":     postExcerptForCard(p, 140),
		})
	}
	return rows
}

func buildClientRecommendPosts(siteSetting entity.SiteSetting, posts []entity.Post) []gin.H {
	rows := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}
		rows = append(rows, gin.H{
			"title":    title,
			"href":     clientPostPublicPath(siteSetting, p),
			"thumb":    postThumbURL(siteSetting, p.Image, p.Content),
			"time":     formatRelativeTimeID(postDisplayTimeRef(p)),
			"cat_name": strings.TrimSpace(p.Category.Name),
		})
	}
	return rows
}

const (
	articleRelatedLimit     = 12
	articleRelatedBatchSize = 5
	articleRecommendLimit   = 6
)

func loadArticleRelatedPosts(db *gorm.DB, post *entity.Post, limit int) []entity.Post {
	if post == nil || post.CategoryID == 0 || limit <= 0 {
		return nil
	}
	var out []entity.Post
	relatedScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
	if publicshelf.Active(loadSiteSetting(db).HomepageLayout) {
		relatedScopes = append(relatedScopes, publicshelf.Scope(""))
	}
	_ = db.
		Select("id, title, slug, content, image, category_id, updated_at, created_at").
		Preload("Category").
		Where("category_id = ?", post.CategoryID).
		Where("id <> ?", post.ID).
		Scopes(relatedScopes...).
		Order("updated_at DESC").
		Limit(limit).
		Find(&out).Error
	return out
}

func loadArticleRecommendPosts(db *gorm.DB, excludePostID uint, limit int) []entity.Post {
	if limit <= 0 {
		return nil
	}
	repo := repositories.NewPostRepository(db)
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	popular, _ := repo.FindPopularPublishedSince(cutoff, limit+4)
	out := make([]entity.Post, 0, limit)
	seen := map[uint]struct{}{}
	if excludePostID > 0 {
		seen[excludePostID] = struct{}{}
	}
	for _, p := range popular {
		if _, ok := seen[p.ID]; ok {
			continue
		}
		out = append(out, p)
		seen[p.ID] = struct{}{}
		if len(out) >= limit {
			return out
		}
	}
	need := limit - len(out)
	if need <= 0 {
		return out
	}
	seenIDs := make([]uint, 0, len(seen))
	for id := range seen {
		seenIDs = append(seenIDs, id)
	}
	var fill []entity.Post
	q := db.
		Select("id, title, slug, content, image, category_id, updated_at, created_at").
		Preload("Category").
		Scopes(visiblePublicScopes(loadSiteSetting(db).HomepageLayout)...).
		Order("updated_at DESC").
		Limit(need)
	if len(seenIDs) > 0 {
		q = q.Where("id NOT IN ?", seenIDs)
	}
	_ = q.Find(&fill).Error
	return append(out, fill...)
}

const (
	homeBootstrapSize = 36
	homeFeedPageSize  = 15
	homeFeedSkip      = 1
)

func homeFeedTotalPages(total int64, skip, size int) int {
	if size <= 0 {
		return 0
	}
	if skip < 0 {
		skip = 0
	}
	effective := total - int64(skip)
	if effective < 0 {
		effective = 0
	}
	return int(math.Ceil(float64(effective) / float64(size)))
}

func postIsBreakingHome(p entity.Post) bool {
	tref := postDisplayTimeRef(p)
	if tref == nil {
		return false
	}
	return time.Since(*tref) < 6*time.Hour
}

func collectHomePopularTags(posts []entity.Post, limit int) []gin.H {
	type agg struct {
		slug  string
		name  string
		count int
	}
	m := make(map[string]*agg)
	for _, p := range posts {
		for _, t := range p.Tags {
			slug := strings.TrimSpace(t.Slug)
			name := strings.TrimSpace(t.Name)
			if slug == "" && name == "" {
				continue
			}
			key := slug
			if key == "" {
				key = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			}
			if m[key] == nil {
				displaySlug := slug
				if displaySlug == "" {
					displaySlug = key
				}
				displayName := name
				if displayName == "" {
					displayName = displaySlug
				}
				m[key] = &agg{slug: displaySlug, name: displayName, count: 0}
			}
			m[key].count++
		}
	}
	list := make([]*agg, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].count != list[j].count {
			return list[i].count > list[j].count
		}
		return strings.Compare(list[i].name, list[j].name) < 0
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]gin.H, 0, len(list))
	for _, a := range list {
		out = append(out, gin.H{
			"name": a.name,
			"href": "/tag/" + url.PathEscape(strings.TrimSpace(a.slug)),
		})
	}
	return out
}

// homeSlidePostsFromBootstrap picks up to 5 carousel posts from an already-loaded bootstrap list.
func homeSlidePostsFromBootstrap(bootstrap []entity.Post, latestPost entity.Post) []entity.Post {
	slidePool := make([]entity.Post, 0, 5)
	for _, p := range bootstrap {
		if latestPost.ID != 0 && p.ID == latestPost.ID {
			continue
		}
		slidePool = append(slidePool, p)
		if len(slidePool) >= 5 {
			break
		}
	}
	return slidePool
}

func postsSliceFromBootstrap(bootstrap []entity.Post, skip, limit int) []entity.Post {
	if skip >= len(bootstrap) || limit <= 0 {
		return nil
	}
	end := skip + limit
	if end > len(bootstrap) {
		end = len(bootstrap)
	}
	return bootstrap[skip:end]
}

func buildHomePageViewModel(db *gorm.DB, siteSetting entity.SiteSetting) (gin.H, []entity.Post) {
	vm := gin.H{"home_ssr": false}
	homeScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
	if publicshelf.Active(siteSetting.HomepageLayout) {
		homeScopes = append(homeScopes, publicshelf.Scope(""))
	}
	var totalPublished int64
	if err := db.Model(&entity.Post{}).Scopes(homeScopes...).Count(&totalPublished).Error; err != nil || totalPublished == 0 {
		return vm, nil
	}

	var bootstrap []entity.Post
	_ = db.Preload("Category").Preload("Tags").Preload("Author").
		Scopes(homeScopes...).
		Order("updated_at DESC").
		Limit(homeBootstrapSize).
		Find(&bootstrap).Error
	if len(bootstrap) == 0 {
		return vm, nil
	}

	var latestNonEditor entity.Post
	latestNonEditor = bootstrap[0]

	slidePosts := homeSlidePostsFromBootstrap(bootstrap, latestNonEditor)
	feed := postsSliceFromBootstrap(bootstrap, homeFeedSkip, homeFeedPageSize)

	slides := make([]gin.H, 0, len(slidePosts))
	for _, p := range slidePosts {
		catName := strings.TrimSpace(p.Category.Name)
		catSlug := strings.TrimSpace(p.Category.Slug)
		catHref := ""
		if catSlug != "" {
			catHref = "/" + url.PathEscape(catSlug)
		}
		slides = append(slides, gin.H{
			"title":       strings.TrimSpace(p.Title),
			"href":        clientPostPublicPath(siteSetting, p),
			"thumb":       postThumbURL(siteSetting, p.Image, p.Content),
			"cat_upper":   strings.ToUpper(catName),
			"cat_name":    catName,
			"cat_href":    catHref,
			"has_cat":     catName != "",
			"rel_time":    formatRelativeTimeID(postDisplayTimeRef(p)),
			"date_label":  formatPostDateCalendar(postDisplayTimeRef(p)),
			"excerpt":     postExcerptForCard(p, 260),
			"author_name": authorDisplayName(p.Author),
			"breaking":    postIsBreakingHome(p),
		})
	}

	if latestNonEditor.ID != 0 {
		catName := strings.TrimSpace(latestNonEditor.Category.Name)
		catSlug := strings.TrimSpace(latestNonEditor.Category.Slug)
		vm["home_latest"] = gin.H{
			"title":        strings.TrimSpace(latestNonEditor.Title),
			"href":         clientPostPublicPath(siteSetting, latestNonEditor),
			"thumb":        postThumbURL(siteSetting, latestNonEditor.Image, latestNonEditor.Content),
			"cat_name":     catName,
			"cat_slug":     catSlug,
			"has_cat":      catName != "",
			"has_cat_slug": catSlug != "",
			"cat_href":     "/" + url.PathEscape(catSlug),
			"rel_time":     formatRelativeTimeID(postDisplayTimeRef(latestNonEditor)),
			"excerpt":      postExcerptForCard(latestNonEditor, 220),
			"author_name":  authorDisplayName(latestNonEditor.Author),
			"date_label":   formatPostDateCalendar(postDisplayTimeRef(latestNonEditor)),
		}
	}

	feedRows := make([]gin.H, 0, len(feed))
	for _, p := range feed {
		cn := strings.TrimSpace(p.Category.Name)
		cs := strings.TrimSpace(p.Category.Slug)
		feedRows = append(feedRows, gin.H{
			"title":        strings.TrimSpace(p.Title),
			"href":         clientPostPublicPath(siteSetting, p),
			"thumb":        postThumbURL(siteSetting, p.Image, p.Content),
			"rel_time":     formatRelativeTimeID(postDisplayTimeRef(p)),
			"cat_name":     cn,
			"has_cat_slug": cs != "",
			"cat_href":     "/" + url.PathEscape(cs),
			"excerpt":      postExcerptForCard(p, 200),
			"author_name":  authorDisplayName(p.Author),
			"date_label":   formatPostDateCalendar(postDisplayTimeRef(p)),
		})
	}

	vm["home_ssr"] = true
	if len(slides) > 0 {
		vm["home_slides"] = slides
	}
	vm["home_tags"] = collectHomePopularTags(bootstrap, 18)
	vm["home_feed"] = feedRows
	vm["home_feed_total_pages"] = homeFeedTotalPages(totalPublished, homeFeedSkip, homeFeedPageSize)
	vm["neo_ticker_posts"] = buildNeoTickerRows(bootstrap, siteSetting, 8)
	vm["neo_featured_tabs"] = buildNeoFeaturedCategoryTabs(db, siteSetting, 3, 5)
	return vm, bootstrap
}

func loadDetikSidebarTags(db *gorm.DB, limit int) []gin.H {
	var posts []entity.Post
	if err := db.Preload("Tags").
		Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).
		Order("updated_at DESC").
		Limit(homeBootstrapSize).
		Find(&posts).Error; err != nil {
		return nil
	}
	return collectHomePopularTags(posts, limit)
}

func baseClientData(c *gin.Context, db *gorm.DB, title, metaTitle, metaDescription string) gin.H {
	siteSetting := loadSiteSetting(db)
	siteMeta := siteMetaFromSetting(siteSetting)
	if strings.TrimSpace(metaDescription) == "" {
		metaDescription = strings.TrimSpace(siteMeta.DefaultDesc)
	}
	data := gin.H{
		"title":                  title,
		"meta_title":             metaTitle,
		"meta_description":       metaDescription,
		"meta_keywords":          siteMeta.DefaultKeywords,
		"meta_image":             siteMeta.DefaultImage,
		"publisher_image":        siteMeta.DefaultImage,
		"og_type":                "website",
		"site_name":              siteSetting.SiteName,
		"tagline":                strings.TrimSpace(siteSetting.Tagline),
		"footer_copy":            siteSetting.FooterCopy,
		"logo_url":               siteSetting.LogoURL,
		"favicon_url":            siteSetting.FaviconURL,
		"theme_color":            themeColorMeta(siteSetting),
		"accent_color":           siteSetting.AccentColor,
		"background_primary":     siteSetting.BackgroundPrimary,
		"background_secondary":   siteSetting.BackgroundSecondary,
		"text_primary":           siteSetting.TextPrimary,
		"text_secondary":         siteSetting.TextSecondary,
		"border_color":           siteSetting.BorderColor,
		"font_family":            siteSetting.FontFamily,
		"content_font_size":      siteSetting.ContentFontSize,
		"sidebar_font_size":      siteSetting.SidebarFontSize,
		"image_caption_enabled":  siteSetting.ImageCaptionEnabled,
		"image_caption_mode":     siteSetting.ImageCaptionMode,
		"image_caption_template": siteSetting.ImageCaptionTemplate,
		"also_read_enabled":      siteSetting.AlsoReadEnabled,
		"also_read_max_inserts":  entity.EffectiveAlsoReadMaxInserts(siteSetting.AlsoReadMaxInserts),
		"image_resize_enabled":   siteSetting.ImageResizeEnabled,
		"img_resize_enabled":     siteSetting.ImgResizeEnabled,
		"img_resize_width":       siteSetting.ImgResizeWidth,
		"img_resize_height":      siteSetting.ImgResizeHeight,
		"img_resize_quality":     siteSetting.ImgResizeQuality,
		"image_resize_width":     siteSetting.ImageResizeWidth,
		"image_resize_quality":   siteSetting.ImageResizeQuality,
		"ads_lazy_load_enabled":  siteSetting.AdsLazyLoadEnabled,
		"avatar_resize_width":    siteSetting.AvatarResizeWidth,
		"avatar_resize_quality":  siteSetting.AvatarResizeQuality,
		"homepage_layout":        entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
		"static_version":         clientStaticVersion,
		"header_script":          clientSiteScriptHTML(siteSetting.HeaderScript),
		"footer_script":          clientSiteScriptHTML(siteSetting.FooterScript),
		"ads_top":                clientAdSlotHTML(siteSetting.AdsTop),
		"ads_center":             clientAdSlotHTML(siteSetting.AdsCenter),
		"ads_bottom":             clientAdSlotHTML(siteSetting.AdsBottom),
		"social_facebook_url":    strings.TrimSpace(siteSetting.SocialFacebookURL),
		"social_x_url":           strings.TrimSpace(siteSetting.SocialXURL),
		"social_instagram_url":   strings.TrimSpace(siteSetting.SocialInstagramURL),
		"social_youtube_url":     strings.TrimSpace(siteSetting.SocialYouTubeURL),
		"social_tiktok_url":      strings.TrimSpace(siteSetting.SocialTikTokURL),
		"social_rss_url":         strings.TrimSpace(siteSetting.SocialRSSURL),
		"current_url":            clientCanonicalURL(c),
		"client_pages":           loadClientMenuPages(db, 0),
		"nav_categories":         loadNavCategories(db, siteSetting.HomepageLayout),
		"load_app_js":            false,
		"load_post_js":           false,
		"local_timezone":         clientPublicTimezone,
		"login_href":             entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
	mergeSidebarListLimitData(data, siteSetting)
	injectNeoHeaderTicker(db, siteSetting, data)
	injectClientVerificationMeta(db, data)
	injectClientAbsoluteAssetURLs(c, siteSetting, data)
	return data
}

func loadSidebarPosts(db *gorm.DB, siteSetting entity.SiteSetting, popularLimit, latestLimit int, latestPrefetch []entity.Post) (popularRows []gin.H, latestRows []gin.H) {
	popularLimit = entity.EffectiveSidebarListLimit(popularLimit)
	latestLimit = entity.EffectiveSidebarListLimit(latestLimit)
	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	// Sama dengan API /posts/popular: ranking berdasarkan view di jendela 24 jam,
	// tanpa memaksa posts.updated_at di jendela yang sama (artikel lama tetap bisa populer).
	popular, _ := repositories.NewPostRepository(db).FindPopularPublishedSince(cutoff, popularLimit)

	// Jika tidak ada view di jendela, tampilkan Artikel terbaru yang dipublikasikan.
	if len(popular) == 0 {
		var fill []entity.Post
		fillScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
		if publicshelf.Active(siteSetting.HomepageLayout) {
			fillScopes = append(fillScopes, publicshelf.Scope(""))
		}
		_ = db.
			Preload("Category").
			Select("id, title, slug, content, image, category_id, updated_at, created_at").
			Scopes(fillScopes...).
			Order("COALESCE(updated_at, created_at) DESC").
			Limit(popularLimit).
			Find(&fill).Error
		popular = append(popular, fill...)
	}

	if len(popular) < popularLimit {
		seen := make(map[uint]struct{}, len(popular))
		seenIDs := make([]uint, 0, len(popular))
		for _, p := range popular {
			seen[p.ID] = struct{}{}
			seenIDs = append(seenIDs, p.ID)
		}

		need := popularLimit - len(popular)
		fallback, _ := repositories.NewPostRepository(db).FindPopularPublishedAllTime(seenIDs, need)
		for _, p := range fallback {
			if len(popular) >= popularLimit {
				break
			}
			if _, ok := seen[p.ID]; ok {
				continue
			}
			popular = append(popular, p)
			seen[p.ID] = struct{}{}
		}
	}

	var latest []entity.Post
	if len(latestPrefetch) >= latestLimit {
		latest = latestPrefetch[:latestLimit]
	} else {
		latestScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
		if publicshelf.Active(siteSetting.HomepageLayout) {
			latestScopes = append(latestScopes, publicshelf.Scope(""))
		}
		_ = db.
			Preload("Category").
			Select("id, title, slug, content, image, category_id, updated_at, created_at").
			Scopes(latestScopes...).
			Order("updated_at DESC").
			Limit(latestLimit).
			Find(&latest).Error
	}

	return buildClientListPosts(siteSetting, popular), buildClientListPosts(siteSetting, latest)
}

func sidebarListLimitFields(siteSetting entity.SiteSetting) gin.H {
	entity.NormalizeSidebarListLimits(&siteSetting)
	return gin.H{
		"sidebar_list_limit":         siteSetting.SidebarListLimitPopular,
		"sidebar_list_limit_popular": siteSetting.SidebarListLimitPopular,
		"sidebar_list_limit_latest":  siteSetting.SidebarListLimitLatest,
	}
}

func mergeSidebarListLimitData(data gin.H, siteSetting entity.SiteSetting) {
	for k, v := range sidebarListLimitFields(siteSetting) {
		data[k] = v
	}
}

func serveClientCategoryPage(c *gin.Context, db *gorm.DB, category *entity.Category) error {
	tmpl, err := clientCategoryTemplate()
	if err != nil {
		return err
	}

	var total int64
	perPage := clientListPageSize
	page := parseClientListPage(c)
	if page < 1 {
		page = 1
	}

	postRepo := repositories.NewPostRepository(db)
	posts, total, err := postRepo.FindPublishedPaginatedByCategoryID(page, perPage, 0, category.ID)
	if err != nil {
		return err
	}
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
		posts, _, err = postRepo.FindPublishedPaginatedByCategoryID(page, perPage, 0, category.ID)
		if err != nil {
			return err
		}
	}

	if !category.IsRoot() && category.ParentID != nil && (category.Parent == nil || category.Parent.ID == 0) {
		if parent, err := repositories.NewCategoryRepository(db).FindByID(*category.ParentID); err == nil {
			category.Parent = parent
		}
	}

	siteSetting := loadSiteSetting(db)
	entity.NormalizeSidebarListLimits(&siteSetting)
	popularRows, latestRows := loadSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitPopular, siteSetting.SidebarListLimitLatest, nil)
	heading := strings.TrimSpace(category.Name)
	if heading == "" {
		heading = strings.TrimSpace(category.Slug)
	}
	webTitleBase := strings.TrimSpace(category.Title)
	if webTitleBase == "" {
		webTitleBase = heading
	}
	metaTitle := webTitleBase + clientBrowserTitleSuffix(siteSetting)
	metaDesc := strings.TrimSpace(category.Description)
	data := baseClientData(c, db, webTitleBase, metaTitle, metaDesc)
	if kw := strings.TrimSpace(category.Keyword); kw != "" {
		data["meta_keywords"] = kw
	}
	data["schema_page_type"] = "category"
	data["schema_category_title"] = webTitleBase
	data["category_name"] = heading
	if !category.IsRoot() && category.Parent != nil {
		if pn := strings.TrimSpace(category.Parent.Name); pn != "" {
			data["category_parent_name"] = pn
			data["category_parent_href"] = entity.CategoryPublicWebPath(*category.Parent)
		}
	}
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["popular_posts"] = popularRows
	data["latest_posts"] = latestRows
	data["pagination_links"] = buildClientPaginationLinks(page, totalPages, categoryPublicWebPath(*category))
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	if publicshelf.Active(siteSetting.HomepageLayout) && total == 0 {
		data["robots"] = "noindex, follow"
	}
	rebuildClientJSONLD(data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

func serveClientTagPage(c *gin.Context, db *gorm.DB, tag *entity.Tag) error {
	tmpl, err := clientTagTemplate()
	if err != nil {
		return err
	}

	perPage := clientListPageSize
	page := parseClientListPage(c)
	if page < 1 {
		page = 1
	}

	postRepo := repositories.NewPostRepository(db)
	posts, total, err := postRepo.FindPublishedPaginatedByTagID(page, perPage, 0, tag.ID)
	if err != nil {
		return err
	}
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
		posts, _, err = postRepo.FindPublishedPaginatedByTagID(page, perPage, 0, tag.ID)
		if err != nil {
			return err
		}
	}
	siteSetting := loadSiteSetting(db)
	entity.NormalizeSidebarListLimits(&siteSetting)
	popularRows, latestRows := loadSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitPopular, siteSetting.SidebarListLimitLatest, nil)
	title := strings.TrimSpace(tag.Name)
	if title == "" {
		title = strings.TrimSpace(tag.Slug)
	}
	metaTitle := "Tag: " + title + clientBrowserTitleSuffix(siteSetting)
	data := baseClientData(c, db, "Tag: "+title, metaTitle, "Kumpulan artikel dengan tag "+title)
	data["schema_page_type"] = "tag"
	data["tag_name"] = title
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["popular_posts"] = popularRows
	data["latest_posts"] = latestRows
	tagPath := "/tag/" + url.PathEscape(strings.TrimSpace(tag.Slug))
	data["pagination_links"] = buildClientPaginationLinks(page, totalPages, tagPath)
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	rebuildClientJSONLD(data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

func serveClientPostsPage(c *gin.Context, db *gorm.DB) error {
	tmpl, err := clientPostsTemplate()
	if err != nil {
		return err
	}

	siteSetting := loadSiteSetting(db)
	listScopes := []func(*gorm.DB) *gorm.DB{publishedpost.ScopeVisiblePublished("", time.Now())}
	if publicshelf.Active(siteSetting.HomepageLayout) {
		listScopes = append(listScopes, publicshelf.Scope(""))
	}
	var total int64
	if err := db.Model(&entity.Post{}).
		Scopes(listScopes...).
		Count(&total).Error; err != nil {
		return err
	}
	perPage := clientListPageSize
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	page := parseClientListPage(c)
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	var posts []entity.Post
	if err := db.
		Select("id, title, slug, content, image, category_id, updated_at, created_at").
		Scopes(listScopes...).
		Order("updated_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts).Error; err != nil {
		return err
	}
	entity.NormalizeSidebarListLimits(&siteSetting)
	popularRows, latestRows := loadSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitPopular, siteSetting.SidebarListLimitLatest, nil)
	metaTitle := "Semua postingan" + clientBrowserTitleSuffix(siteSetting)
	data := baseClientData(c, db, "Semua postingan", metaTitle, "Daftar semua artikel yang dipublikasikan.")
	data["schema_page_type"] = "posts"
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["popular_posts"] = popularRows
	data["latest_posts"] = latestRows
	data["pagination_links"] = buildClientPaginationLinks(page, totalPages, "/posts")
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	rebuildClientJSONLD(data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

func matchAuthorSlug(u entity.User, slug string) bool {
	target := slugifyTextPath(slug)
	if target == "" {
		return false
	}
	if authorPublicSlug(u) == target {
		return true
	}
	// Legacy paths from older canonical rules (role → name → email local).
	if slugifyTextPath(u.Role) == target {
		return true
	}
	if slugifyTextPath(u.Name) == target {
		return true
	}
	em := strings.TrimSpace(u.Email)
	if em != "" {
		local := strings.SplitN(em, "@", 2)[0]
		if slugifyTextPath(local) == target {
			return true
		}
	}
	return false
}

func serveClientAuthorPage(c *gin.Context, db *gorm.DB, slug string) error {
	tmpl, err := clientAuthorTemplate()
	if err != nil {
		return err
	}

	var users []entity.User
	if err := db.Select("id, name, username, email, role, profile, image, enabled").Order("id asc").Find(&users).Error; err != nil {
		return err
	}
	var author *entity.User
	for i := range users {
		if !users[i].Enabled {
			continue
		}
		if matchAuthorSlug(users[i], slug) {
			author = &users[i]
			break
		}
	}
	if author == nil {
		return gorm.ErrRecordNotFound
	}

	siteSetting := loadSiteSetting(db)
	authorScopes := visiblePublicScopes(siteSetting.HomepageLayout)
	var total int64
	if err := db.Model(&entity.Post{}).
		Where("author_id = ?", author.ID).
		Scopes(authorScopes...).
		Count(&total).Error; err != nil {
		return err
	}
	perPage := clientListPageSize
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	page := parseClientListPage(c)
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	var posts []entity.Post
	if err := db.
		Preload("Category").
		Select("id, title, slug, content, image, category_id, updated_at, created_at").
		Where("author_id = ?", author.ID).
		Scopes(authorScopes...).
		Order("updated_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts).Error; err != nil {
		return err
	}

	entity.NormalizeSidebarListLimits(&siteSetting)
	popularRows, latestRows := loadSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitPopular, siteSetting.SidebarListLimitLatest, nil)
	authorName := strings.TrimSpace(author.Name)
	if authorName == "" {
		authorName = slug
	}
	metaTitle := "Author: " + authorName + clientBrowserTitleSuffix(siteSetting)
	data := baseClientData(c, db, "Author: "+authorName, metaTitle, "Kumpulan artikel dari author "+authorName)
	data["schema_page_type"] = "author"
	data["author_name"] = authorName
	data["author_avatar_letter"] = firstRuneUpperDisplay(authorName)
	data["author_slug"] = slug
	data["author_bio"] = strings.TrimSpace(author.Profile)
	data["author_posts_count"] = int(total)
	data["author_role_label"] = authorRoleLabel(author.Role)
	if author.Image != nil {
		data["author_img"] = avatarResizeURL(siteSetting, strings.TrimSpace(*author.Image))
	}
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["popular_posts"] = popularRows
	data["latest_posts"] = latestRows
	data["pagination_links"] = buildClientPaginationLinks(page, totalPages, authorPublicListPath(slug))
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	rebuildClientJSONLD(data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

func authorRoleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "Administrator"
	case "writer":
		return "Penulis"
	default:
		r := strings.TrimSpace(role)
		if r == "" {
			return "Kontributor"
		}
		return r
	}
}

func firstRuneUpperDisplay(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return "?"
	}
	return strings.ToUpper(string(r))
}

func formatPostDateLocalized(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	v := *t
	if loc, err := time.LoadLocation(clientPublicTimezone); err == nil {
		v = v.In(loc)
	}
	m := int(v.Month())
	if m < 1 || m > 12 {
		m = 1
	}
	return fmt.Sprintf("%d %s %d %02d.%02d", v.Day(), indonesianMonths[m], v.Year(), v.Hour(), v.Minute())
}

func formatRelativeTimeID(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.After(now) {
		return "1 menit lalu"
	}
	delta := now.Sub(*t)
	sec := int64(delta / time.Second)
	if sec < 3600 {
		m := sec / 60
		if m < 1 {
			m = 1
		}
		return fmt.Sprintf("%d menit lalu", m)
	}
	if sec < 86400 {
		return fmt.Sprintf("%d jam lalu", sec/3600)
	}
	days := sec / 86400
	if days < 30 {
		if days < 1 {
			days = 1
		}
		return fmt.Sprintf("%d hari lalu", days)
	}
	months := days / 30
	if months < 12 {
		if months < 1 {
			months = 1
		}
		return fmt.Sprintf("%d bulan lalu", months)
	}
	years := days / 365
	if years < 1 {
		years = 1
	}
	return fmt.Sprintf("%d tahun lalu", years)
}

// serveClientArticlePage renders a published post with full article HTML (SSR) for SEO and no-JS reading.
func serveClientArticlePage(c *gin.Context, db *gorm.DB, post *entity.Post) error {
	tmpl, err := clientArticleTemplate()
	if err != nil {
		return err
	}

	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)

	displayTitle := strings.TrimSpace(post.Title)
	metaHead := displayTitle
	// Tab browser: judul artikel saja (meta judul SEO jika ada, tanpa nama situs).
	metaTitleForHead := metaHead

	metaDesc := ""
	if post.MetaDesc != nil {
		metaDesc = strings.TrimSpace(*post.MetaDesc)
	}
	if metaDesc == "" {
		metaDesc = textPreview(post.Content, 180)
	}
	if metaDesc == "" {
		metaDesc = strings.TrimSpace(siteMeta.DefaultDesc)
	}

	metaKw := siteMeta.DefaultKeywords
	if post.MetaKeywords != nil && strings.TrimSpace(*post.MetaKeywords) != "" {
		metaKw = strings.TrimSpace(*post.MetaKeywords)
	}

	var nViews int64
	_ = db.Model(&entity.PostView{}).Where("post_id = ?", post.ID).Count(&nViews).Error

	var metaLine string
	if !post.UpdatedAt.IsZero() {
		t := post.UpdatedAt
		metaLine = formatPostDateLocalized(&t)
	} else if !post.CreatedAt.IsZero() {
		t := post.CreatedAt
		metaLine = formatPostDateLocalized(&t)
	}
	viewStr := strconv.FormatInt(nViews, 10) + " dibaca"
	bylineMetaTop := ""
	if metaLine != "" || viewStr != "" {
		ml := metaLine
		if ml == "" {
			ml = "-"
		}
		vl := viewStr
		if vl == "" {
			vl = "0 dibaca"
		}
		bylineMetaTop = ml + " · " + vl
	}

	catName := ""
	catSlug := ""
	catHref := ""
	if post.Category.ID != 0 {
		preloadCategoryParentIfNeeded(db, &post.Category)
		catName = strings.TrimSpace(post.Category.Name)
		catSlug = strings.TrimSpace(post.Category.Slug)
		catHref = entity.CategoryPublicWebPath(post.Category)
	}

	author := post.Author
	authorName := strings.TrimSpace(author.Name)
	if authorName == "" {
		authorName = "Tim redaksi"
	}
	avatarLetter := firstRuneUpperDisplay(authorName)
	authorImg := ""
	if author.Image != nil {
		authorImg = avatarResizeURL(siteSetting, strings.TrimSpace(*author.Image))
	}
	authorBio := strings.TrimSpace(author.Profile)
	authorPath := authorPublicPathFromUser(author)

	heroURL := postHeroURL(siteSetting, post.Image, post.Content)

	metaImg := postCoverRawURL(post.Image, post.Content)
	if metaImg == "" {
		metaImg = siteMeta.DefaultImage
	}

	entity.NormalizeSidebarListLimits(&siteSetting)
	popularRows, latestRows := loadSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitPopular, siteSetting.SidebarListLimitLatest, nil)
	detikTags := loadDetikSidebarTags(db, 18)

	relatedPosts := loadArticleRelatedPosts(db, post, articleRelatedLimit)
	recommendPosts := loadArticleRecommendPosts(db, post.ID, articleRecommendLimit)
	articleRelatedRows := buildClientListPosts(siteSetting, relatedPosts)
	articleRecommendRows := buildClientRecommendPosts(siteSetting, recommendPosts)

	sourceURL := ""
	sourceLabel := ""
	sourceDisclaimerHTML := ""
	if post.IncludeSource && post.Source != nil {
		su := strings.TrimSpace(*post.Source)
		if su != "" {
			sourceURL = su
			if u, err := url.Parse(su); err == nil && u.Host != "" {
				sourceLabel = u.Host
			} else {
				sourceLabel = su
			}
		}
	}
	if sourceURL != "" {
		sourceDisclaimerHTML = buildSourceDisclaimerHTML(
			strings.TrimSpace(siteSetting.SourceDisclaimerMode),
			strings.TrimSpace(siteSetting.SourceDisclaimerTemplate),
			sourceURL,
			sourceLabel,
		)
	}

	postBodyHTML := posthtml.BuildPublicPostBodyHTML(canonicalBaseURL(c), post.Content, displayTitle, siteSetting)
	if siteSetting.AutoHrefEnabled {
		postBodyHTML = autohref.Apply(db, siteSetting, post.ID, postBodyHTML)
	}
	heroCaptionHTML := template.HTML(imagecaption.BuildHeroCaptionInnerHTML(
		siteSetting.ImageCaptionEnabled,
		siteSetting.ImageCaptionMode,
		siteSetting.ImageCaptionTemplate,
		*post,
		displayTitle,
	))

	robots := "index, follow"
	if publicshelf.Active(siteSetting.HomepageLayout) && !publicshelf.Matches(post.Title, post.Slug) {
		robots = "noindex, follow"
	}
	data := gin.H{
		"title":                      metaHead,
		"meta_title":                 metaTitleForHead,
		"meta_description":           metaDesc,
		"robots":                     robots,
		"meta_keywords":              metaKw,
		"meta_image":                 metaImg,
		"meta_author":                authorName,
		"og_type":                    "article",
		"site_name":                  siteSetting.SiteName,
		"tagline":                    strings.TrimSpace(siteSetting.Tagline),
		"footer_copy":                siteSetting.FooterCopy,
		"logo_url":                   siteSetting.LogoURL,
		"publisher_image":            siteMeta.DefaultImage,
		"favicon_url":                siteSetting.FaviconURL,
		"theme_color":                themeColorMeta(siteSetting),
		"accent_color":               siteSetting.AccentColor,
		"background_primary":         siteSetting.BackgroundPrimary,
		"background_secondary":       siteSetting.BackgroundSecondary,
		"text_primary":               siteSetting.TextPrimary,
		"text_secondary":             siteSetting.TextSecondary,
		"border_color":               siteSetting.BorderColor,
		"font_family":                siteSetting.FontFamily,
		"content_font_size":          siteSetting.ContentFontSize,
		"sidebar_font_size":          siteSetting.SidebarFontSize,
		"image_caption_enabled":      siteSetting.ImageCaptionEnabled,
		"image_caption_mode":         siteSetting.ImageCaptionMode,
		"image_caption_template":     siteSetting.ImageCaptionTemplate,
		"also_read_enabled":          siteSetting.AlsoReadEnabled,
		"also_read_max_inserts":      entity.EffectiveAlsoReadMaxInserts(siteSetting.AlsoReadMaxInserts),
		"image_resize_enabled":       siteSetting.ImageResizeEnabled,
		"img_resize_enabled":         siteSetting.ImgResizeEnabled,
		"img_resize_width":           siteSetting.ImgResizeWidth,
		"img_resize_height":          siteSetting.ImgResizeHeight,
		"img_resize_quality":         siteSetting.ImgResizeQuality,
		"image_resize_width":         siteSetting.ImageResizeWidth,
		"image_resize_quality":       siteSetting.ImageResizeQuality,
		"ads_lazy_load_enabled":      siteSetting.AdsLazyLoadEnabled,
		"avatar_resize_width":        siteSetting.AvatarResizeWidth,
		"avatar_resize_quality":      siteSetting.AvatarResizeQuality,
		"homepage_layout":            entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
		"static_version":             clientStaticVersion,
		"header_script":              clientSiteScriptHTML(siteSetting.HeaderScript),
		"footer_script":              clientSiteScriptHTML(siteSetting.FooterScript),
		"ads_top":                    clientAdSlotHTML(siteSetting.AdsTop),
		"ads_center":                 clientAdSlotHTML(siteSetting.AdsCenter),
		"ads_bottom":                 clientAdSlotHTML(siteSetting.AdsBottom),
		"social_facebook_url":        strings.TrimSpace(siteSetting.SocialFacebookURL),
		"social_x_url":               strings.TrimSpace(siteSetting.SocialXURL),
		"social_instagram_url":       strings.TrimSpace(siteSetting.SocialInstagramURL),
		"social_youtube_url":         strings.TrimSpace(siteSetting.SocialYouTubeURL),
		"social_tiktok_url":          strings.TrimSpace(siteSetting.SocialTikTokURL),
		"social_rss_url":             strings.TrimSpace(siteSetting.SocialRSSURL),
		"current_url":                clientCanonicalURL(c),
		"client_pages":               loadClientMenuPages(db, 0),
		"nav_categories":             loadNavCategories(db, siteSetting.HomepageLayout),
		"load_app_js":                false,
		"load_post_js":               true,
		"local_timezone":             clientPublicTimezone,
		"post_slug":                  post.Slug,
		"post_title":                 displayTitle,
		"article_schema_type":        entity.NormalizeArticleSchemaType(strings.TrimSpace(siteSetting.ArticleSchemaType)),
		"post_created_at_iso":        post.CreatedAt.UTC().Format(time.RFC3339),
		"post_updated_at_iso":        post.UpdatedAt.UTC().Format(time.RFC3339),
		"post_body":                  template.HTML(postBodyHTML),
		"cat_name":                   catName,
		"cat_slug":                   catSlug,
		"cat_href":                   catHref,
		"byline_meta_top":            bylineMetaTop,
		"post_time_label":            metaLine,
		"post_read_label":            viewStr,
		"author_name":                authorName,
		"author_img":                 authorImg,
		"avatar_letter":              avatarLetter,
		"author_path":                authorPath,
		"author_bio":                 authorBio,
		"hero_url":                   heroURL,
		"hero_caption_html":          heroCaptionHTML,
		"source_url":                 sourceURL,
		"source_label":               sourceLabel,
		"source_disclaimer_html":     template.HTML(sourceDisclaimerHTML),
		"popular_posts":              popularRows,
		"latest_posts":               latestRows,
		"detik_recommend_posts":      buildRandomRecommendationRows(popularRows, latestRows, siteSetting.SidebarListLimitLatest),
		"home_tags":                  detikTags,
		"article_related_posts":      articleRelatedRows,
		"article_related_batch_size": articleRelatedBatchSize,
		"article_recommend_posts":    articleRecommendRows,
		"login_href":                 entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
	// Breadcrumb induk + sub hanya untuk mode URL 3 segmen; mode category (/mbg/judul) → Home > sub > judul.
	if entity.NormalizeArticleSlugMode(siteSetting.ArticleSlugMode) == entity.ArticleSlugModeCategoryNested &&
		post.Category.ID != 0 && !post.Category.IsRoot() && post.Category.Parent != nil {
		if pn := strings.TrimSpace(post.Category.Parent.Name); pn != "" {
			data["category_parent_name"] = pn
			data["category_parent_href"] = entity.CategoryPublicWebPath(*post.Category.Parent)
		}
	}
	mergeSidebarListLimitData(data, siteSetting)
	injectNeoHeaderTicker(db, siteSetting, data)
	injectClientVerificationMeta(db, data)
	injectClientAbsoluteAssetURLs(c, siteSetting, data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-client-app", data)
}

// publicAdsTxt drops sample publisher IDs. A real line is served only after it is saved in admin.
func publicAdsTxt(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "pub-0000000000000000") || strings.Contains(lower, "pub-xxxxxxxxxxxxxxxx") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func buildSourceDisclaimerHTML(mode, customTpl, sourceURL, sourceLabel string) string {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return ""
	}
	anchorText := strings.TrimSpace(sourceLabel)
	if anchorText == "" {
		anchorText = sourceURL
	}
	anchor := `<a href="` + template.HTMLEscapeString(sourceURL) + `" target="_blank" rel="nofollow noopener noreferrer">` + template.HTMLEscapeString(anchorText) + `</a>`
	if strings.EqualFold(strings.TrimSpace(mode), "custom") {
		tpl := strings.TrimSpace(customTpl)
		if tpl == "" || entity.SourceDisclaimerAdmitsAutomaticRewrite(tpl) {
			tpl = entity.DefaultSourceDisclaimerTemplate
		}
		escaped := template.HTMLEscapeString(tpl)
		escaped = strings.ReplaceAll(escaped, "{{source}}", anchor)
		escaped = strings.ReplaceAll(escaped, "\n", "<br />")
		return escaped
	}
	return "<strong>Sumber:</strong> " + anchor
}

// loadAllHTMLTemplates loads all HTML templates from a directory.
func loadAllHTMLTemplates(r *gin.Engine, root string) {
	var files []string

	// WalkDir is safe on all platforms and avoids accidental directory matches.
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d == nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".html") {
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		panic("no html templates found under " + root)
	}

	r.LoadHTMLFiles(files...)
}

// aiConfigured reports whether at least one provider key is stored in ai_setting.
func aiConfigured(db *gorm.DB) bool {
	var row entity.AISetting
	if err := db.Table("ai_setting").First(&row).Error; err != nil {
		return false
	}
	return strings.TrimSpace(row.Gemini) != "" ||
		strings.TrimSpace(row.Groq) != "" ||
		strings.TrimSpace(row.OpenAI) != "" ||
		strings.TrimSpace(row.Claude) != ""
}

// loadAISetting returns the first ai_setting row for server-rendered defaults, or empty values.
func loadAISetting(db *gorm.DB) entity.AISetting {
	var row entity.AISetting
	if err := db.Model(&entity.AISetting{}).First(&row).Error; err != nil {
		return entity.AISetting{}
	}
	return row
}

func randomPublishedPostPath(db *gorm.DB) string {
	ss := loadSiteSetting(db)
	baseScope := db.Model(&entity.Post{}).
		Scopes(visiblePublicScopes(ss.HomepageLayout)...).
		Where("TRIM(slug) <> ''")
	var total int64
	if err := baseScope.Count(&total).Error; err != nil || total < 1 {
		return ""
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	offset := rng.Int63n(total)
	var post entity.Post
	if err := db.
		Preload("Category").
		Scopes(visiblePublicScopes(ss.HomepageLayout)...).
		Where("TRIM(slug) <> ''").
		Order("id ASC").
		Offset(int(offset)).
		Limit(1).
		First(&post).Error; err != nil {
		return ""
	}
	return clientPostPublicPath(ss, post)
}

func notFoundRedirectTarget(db *gorm.DB) string {
	mode := entity.NotFoundRedirectHome
	if db != nil {
		s := loadSiteSetting(db)
		mode = entity.NormalizeNotFoundRedirectMode(s.NotFoundRedirectMode)
	}
	if mode == entity.NotFoundRedirectRandom {
		if p := randomPublishedPostPath(db); p != "" {
			return p
		}
	}
	return "/"
}

// loadSiteSetting returns the singleton site_settings row (cached), or sensible defaults.
func loadSiteSetting(db *gorm.DB) entity.SiteSetting {
	return repositories.SiteSettingOrDefault(db)
}

// loadSiteCloudStorage fetches the first SiteCloudStorage row, or returns sensible defaults.
func loadSiteCloudStorage(db *gorm.DB) entity.SiteCloudStorage {
	var s entity.SiteCloudStorage
	if err := db.First(&s).Error; err == nil {
		return s
	}

	return entity.DefaultSiteCloudStorage()
}

// themeColorMeta picks accent for <meta name="theme-color">; falls back if accent is empty.
func themeColorMeta(s entity.SiteSetting) string {
	if v := strings.TrimSpace(s.AccentColor); v != "" {
		return v
	}
	return strings.TrimSpace(s.BackgroundPrimary)
}

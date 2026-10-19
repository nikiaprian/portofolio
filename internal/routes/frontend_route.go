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
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/cache"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/client"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/html"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/scraper"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/middleware"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/handler"
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

func googleFontStylesheet(family string) string {
	known := []string{
		"Source Serif 4", "Playfair Display", "Nunito Sans", "Open Sans", "Work Sans",
		"PT Serif", "Comic Neue", "Merriweather", "Montserrat", "Literata", "Poppins",
		"Inter", "Roboto", "Oswald", "Lora",
	}
	lower := strings.ToLower(family)
	for _, name := range known {
		if strings.Contains(lower, strings.ToLower(name)) {
			return "https://fonts.googleapis.com/css2?family=" + strings.ReplaceAll(name, " ", "+") + ":wght@400;600;700&display=swap"
		}
	}
	return ""
}

// BLANK_IMG is a 1x1 transparent GIF data URL used as a safe fallback.
// Keep this in sync with the frontend constant in `public/static/js/app.js`.
const BLANK_IMG = "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw=="

// cache-bust client static assets (css/js) on each deploy/restart
var clientStaticVersion = strconv.FormatInt(time.Now().Unix(), 10)

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
	if err := db.Select("id", "name", "slug").Where("enabled = ?", true).First(&parent, *cat.ParentID).Error; err == nil {
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
	if err := db.Where("slug = ? AND enabled = ?", parentSlug, true).First(&parent).Error; err != nil {
		return nil, err
	}
	if !parent.IsRoot() {
		return nil, gorm.ErrRecordNotFound
	}
	var child entity.Category
	if err := db.Preload("Parent").Where("slug = ? AND parent_id = ? AND enabled = ?", childSlug, parent.ID, true).First(&child).Error; err != nil {
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
		"favicon.ico", "robots.txt", "ads.txt", "beli", "kelola", "testimoni":
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
	data["scraper_guard_meta"] = template.HTML(scraputil.MetaTagHTML())
	data["current_year"] = time.Now().Year()
	data["tagline"] = siteSetting.Tagline
	data["site_description"] = siteSetting.DefaultDesc
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

func isBackendMediaPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(raw, "//") {
		return false
	}
	path := media.StripResizePrefixes(raw)
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, media.ResizeProxyPath+"/") || strings.HasPrefix(path, media.ResizeProxyPath+"?") {
		return true
	}
	return strings.HasPrefix(path, "/static/")
}

// absolutizeRSSContentURLs absolutizes href/src in RSS content:encoded.
// Backend-hosted assets (/static/, /img) use mediaBaseURL; article and other links use siteBaseURL.
func absolutizeRSSContentURLs(mediaBaseURL, siteBaseURL, html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	return htmlAttrURLRe.ReplaceAllStringFunc(html, func(m string) string {
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
		base := siteBaseURL
		if attr == "src" && isBackendMediaPath(path) {
			base = mediaBaseURL
		}
		abs := toClientAbsoluteURL(base, path)
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
func loadNavCategories(db *gorm.DB) []entity.NavCategoryNode {
	// declare cache param
	store := cache.GlobalStore()
	var cats []entity.Category
	const loadNavCategoriesCacheKey = "nav_categories"

	// get nav_categories
	if store != nil {
		if ok, _ := store.GetJSON(context.Background(), loadNavCategoriesCacheKey, &cats); ok {
			return entity.BuildNavCategoryTree(cats)
		}
	}

	if err := db.Model(&entity.Category{}).
		Select("id", "enabled", "sort", "name", "slug", "parent_id").
		Where("enabled = ?", true).
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
		store.SetJSON(context.Background(), loadNavCategoriesCacheKey, &cats, cache.NavCategoriesTTL())
	}
	return entity.BuildNavCategoryTree(cats)
}

// serveClientStaticPage renders a published static Page (e.g. /kebijakan-privasi).
func serveClientStaticPage(c *gin.Context, db *gorm.DB, page *entity.Page) error {
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
		"nav_categories":        loadNavCategories(db),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
	injectClientVerificationMeta(db, data)
	injectClientAbsoluteAssetURLs(c, siteSetting, data)
	return renderPortfolioFile(c, db, http.StatusOK, "info.html", data)
}

func serveClientErrorPage(c *gin.Context, db *gorm.DB, status int, heading, description string) {
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
	data["SiteName"] = portfolioSiteName(siteSetting)
	if err := renderPortfolioFile(c, db, status, "error.html", data); err != nil {
		c.String(status, "error: %s", http.StatusText(status))
	}
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

func isFrontendDisabledExemptPath(path string) bool {
	if path == "/health" {
		return true
	}
	if strings.HasPrefix(path, "/api/") {
		return true
	}
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	if path == media.ResizeProxyPath || strings.HasPrefix(path, media.ResizeProxyPath+"/") {
		return true
	}
	// Public SEO/XML feeds must stay available when frontend SSR is off (headless + external FE).
	if path == "/robots.txt" || path == "/sitemap.xml" || strings.HasPrefix(path, "/rss/") {
		return true
	}
	return false
}

func serveFrontendDisabledPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusServiceUnavailable)
	if c.Request.Method != http.MethodHead {
		c.File("./public/static/frontend-disabled.html")
	}
	c.Abort()
}

// frontendRoutesDisabledMiddleware blocks public SSR routes when frontend_routes_disabled is on.
// Admin/login paths are handled separately; API, static, /img, /health, and SEO/XML feeds stay available.
func frontendRoutesDisabledMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isFrontendDisabledExemptPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		setting := loadSiteSetting(db)
		if entity.IsAdminOrLoginFrontendPath(c.Request.URL.Path, setting.LoginURL) {
			c.Next()
			return
		}
		if setting.FrontendRoutesDisabled {
			serveFrontendDisabledPage(c)
			return
		}
		c.Next()
	}
}

// adminLoginFrontendDisabledMiddleware blocks SSR login and /admin when admin_login_frontend_disabled is on.
func adminLoginFrontendDisabledMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		setting := loadSiteSetting(db)
		if !setting.AdminLoginFrontendDisabled {
			c.Next()
			return
		}
		if entity.IsAdminOrLoginFrontendPath(c.Request.URL.Path, setting.LoginURL) {
			serveFrontendDisabledPage(c)
			return
		}
		c.Next()
	}
}

// RegisterFrontendRoutes configures public client pages, login, and catch-all SSR routes.
// Admin pages that require login are registered via RegisterAdminFrontendRoutes.
func RegisterFrontendRoutes(r *gin.Engine, db *gorm.DB, cfg config.Config) {
	clientPublicTimezone = normalizePublicTimezone(cfg.LocalTimezone)
	redirectHome := func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, notFoundRedirectTarget(db))
	}

	r.Use(staticCacheHeaders())
	r.Use(frontendRoutesDisabledMiddleware(db))
	r.Use(adminLoginFrontendDisabledMiddleware(db))
	r.GET(media.ResizeProxyPath, handler.ServeResizedImage)
	r.GET(media.ResizeProxyPath+"/*filepath", handler.ServeResizedImage)
	r.GET("/static/*filepath", handler.ServeStaticOrResized)
	loadAllHTMLTemplates(r, "public/templates")

	serveClientIndex := func(c *gin.Context) {
		if err := renderPortfolioLanding(c, db, cfg); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
		}
	}

	r.GET("/", middleware.PublicSearchLimit(), serveClientIndex)
	registerPortfolioBuyerRoutes(r, db, cfg)

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
			Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
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
		if err := db.Where("enabled = ?", true).Order("slug ASC").Find(&cats).Error; err != nil {
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
			"nav_categories":        loadNavCategories(db),
			"rss_rows":              rows,
		}
		for k, v := range clientSocialLinksH(siteSetting) {
			data[k] = v
		}
		injectClientVerificationMeta(db, data)
		injectClientAbsoluteAssetURLs(c, siteSetting, data)
		data["SiteName"] = portfolioSiteName(siteSetting)
		if err := renderPortfolioFile(c, db, http.StatusOK, "rss.html", data); err != nil {
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
		seo.ServeSitemapIndex(c, db, canonicalBaseURL(c))
	})

	r.GET("/rss/sitemap-news.xml", func(c *gin.Context) {
		site := loadSiteSetting(db)
		baseURL := canonicalBaseURL(c)
		cutoff := time.Now().Add(-48 * time.Hour)

		var posts []entity.Post
		err := db.
			Preload("Category").
			Scopes(postutil.ScopeVisiblePublished("", time.Now())).
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
			rel := entity.PublicArticleWebPath(site.ArticleSlugMode, categoryPathForPublishedPost(site.ArticleSlugMode, p), postSlug)
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
		seo.ServeRobotsTxt(c, db)
	})

	r.GET("/ads.txt", func(c *gin.Context) {
		seo.ServeAdsTxt(c, db)
	})

	// RSS
	r.GET("/rss/:slug", func(c *gin.Context) {
		site := loadSiteSetting(db)
		baseURL := canonicalBaseURL(c)
		mediaBaseURL := seo.MediaOrigin(c)
		categorySlug := strings.TrimSpace(c.Param("slug"))
		categorySlug = strings.TrimSuffix(categorySlug, ".xml")
		if categorySlug == "" {
			redirectHome(c)
			return
		}

		// Special feed: latest posts across all categories.
		if categorySlug == "latest-posts" {
			var posts []entity.Post
			err := db.
				Preload("Author").
				Preload("Category").
				Preload("Tags").
				Scopes(postutil.ScopeVisiblePublished("", time.Now())).
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
				if img := clientMetaImageURL(mediaBaseURL, derefString(p.Image)); img != "" {
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
				rel := entity.PublicArticleWebPath(site.ArticleSlugMode, categoryPathForPublishedPost(site.ArticleSlugMode, p), p.Slug)
				itemURL := strings.TrimRight(baseURL, "/") + rel
				items = append(items, rssItem{
					Title:          p.Title,
					Link:           itemURL,
					GUID:           rssGUID{IsPermaLink: "true", Value: itemURL},
					Description:    rssCDATA{Value: description},
					ContentEncoded: rssCDATA{Value: scraputil.PrependMarkerToCDATA(absolutizeRSSContentURLs(mediaBaseURL, baseURL, p.Content))},
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
		if err := db.Where("slug = ? AND enabled = ?", categorySlug, true).First(&category).Error; err != nil {
			redirectHome(c)
			return
		}

		var posts []entity.Post
		err := db.
			Preload("Author").
			Preload("Category").
			Preload("Tags").
			Where("category_id = ?", category.ID).
			Scopes(postutil.ScopeVisiblePublished("", time.Now())).
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
			if img := clientMetaImageURL(mediaBaseURL, derefString(p.Image)); img != "" {
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
			rel := entity.PublicArticleWebPath(site.ArticleSlugMode, categoryPathForPublishedPost(site.ArticleSlugMode, p), p.Slug)
			itemURL := strings.TrimRight(baseURL, "/") + rel
			items = append(items, rssItem{
				Title:          p.Title,
				Link:           itemURL,
				GUID:           rssGUID{IsPermaLink: "true", Value: itemURL},
				Description:    rssCDATA{Value: description},
				ContentEncoded: rssCDATA{Value: scraputil.PrependMarkerToCDATA(absolutizeRSSContentURLs(mediaBaseURL, baseURL, p.Content))},
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
			"public/templates/layouts/portfolio/app.html",
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
			"nav_categories":       loadNavCategories(db),
			"login_href":           loginHref,
			"static_version":       clientStaticVersion,
		}
		for k, v := range clientSocialLinksH(siteSetting) {
			data[k] = v
		}
		injectClientVerificationMeta(db, data)
		injectClientAbsoluteAssetURLs(c, siteSetting, data)
		data["SiteName"] = portfolioSiteName(siteSetting)
		data["NoIndex"] = true
		applyPortfolioHead(c, db, data)

		if err := tmpl.ExecuteTemplate(c.Writer, "pf-app", data); err != nil {
			c.String(http.StatusInternalServerError, "render error: %v", err)
			return
		}
	}
	loginHref := entity.EffectiveLoginHref(loadSiteSetting(db).LoginURL)
	lowLoginHref := strings.ToLower(strings.TrimSpace(loginHref))
	if !strings.HasPrefix(lowLoginHref, "http://") && !strings.HasPrefix(lowLoginHref, "https://") {
		r.GET(loginHref, clientLoginPage)
	}

	RegisterAdminFrontendRoutes(r, db, cfg)

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
			Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&post).Error; err != nil {
			var alt entity.Post
			if err2 := db.Preload("Category").Preload("Category.Parent").Where("slug = ?", postSlug).
				Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&alt).Error; err2 == nil {
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
			if err := db.Preload("Parent").Where("slug = ? AND enabled = ?", catSlug, true).First(&cat).Error; err == nil {
				var post entity.Post
				if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
					Where("slug = ? AND category_id = ?", postSlug, cat.ID).
					Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
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
				Where("slug = ?", postSlug).Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
				c.Redirect(http.StatusMovedPermanently, clientPostPublicPath(ss, post))
				return
			}
			redirectHome(c)
			return
		}

		if slugMode == entity.ArticleSlugModeCategoryNested {
			var cat entity.Category
			if err := db.Preload("Parent").Where("slug = ? AND enabled = ?", catSlug, true).First(&cat).Error; err == nil {
				var post entity.Post
				if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
					Where("slug = ? AND category_id = ?", postSlug, cat.ID).
					Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
					if servePostWithCanonicalRedirect(post) {
						return
					}
				}
			}
		}

		var alt entity.Post
		if err := db.Preload("Author").Preload("Category").Preload("Category.Parent").Preload("Tags").
			Where("slug = ?", postSlug).
			Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&alt).Error; err == nil {
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
		if err := db.Preload("Parent").Where("slug = ? AND enabled = ?", slug, true).First(&cat).Error; err == nil {
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
			Where("slug = ?", slug).Scopes(postutil.ScopeVisiblePublished("", time.Now())).First(&post).Error; err == nil {
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

type siteMetaView struct {
	DefaultTitle    string
	DefaultDesc     string
	DefaultImage    string
	DefaultKeywords string
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
	return seo.RequestOrigin(c)
}

func writeXMLResponse(c *gin.Context, payload any) {
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte(xml.Header))
	_, _ = c.Writer.Write([]byte(scraputil.MarkerComment() + "\n"))
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

// imageResizeURL applies /static/...?q= when enabled; width is chosen client-side from layout.
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
	for i, p := range posts {
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
			"rank":        fmt.Sprintf("%d", i+1),
			"cat_name":    strings.TrimSpace(p.Category.Name),
			"has_cat":     strings.TrimSpace(p.Category.Name) != "",
		})
	}
	return rows
}

const articleRelatedLimit = 12

func loadArticleRelatedPosts(db *gorm.DB, post *entity.Post, limit int) []entity.Post {
	if post == nil || post.CategoryID == 0 || limit <= 0 {
		return nil
	}
	var out []entity.Post
	_ = db.
		Select("id, title, slug, content, image, category_id, updated_at, created_at").
		Preload("Category").
		Where("category_id = ?", post.CategoryID).
		Where("id <> ?", post.ID).
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Order("updated_at DESC").
		Limit(limit).
		Find(&out).Error
	return out
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
		"font_css":               template.CSS(siteSetting.FontFamily),
		"font_stylesheet":        googleFontStylesheet(siteSetting.FontFamily),
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
		"nav_categories":         loadNavCategories(db),
		"load_app_js":            false,
		"load_post_js":           false,
		"local_timezone":         clientPublicTimezone,
		"login_href":             entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
	mergeSidebarListLimitData(data, siteSetting)
	injectClientVerificationMeta(db, data)
	injectClientAbsoluteAssetURLs(c, siteSetting, data)
	return data
}

func loadLatestSidebarPosts(db *gorm.DB, siteSetting entity.SiteSetting, limit int) []gin.H {
	limit = entity.EffectiveSidebarListLimit(limit)
	var latest []entity.Post
	_ = db.
		Preload("Category").
		Select("id, title, slug, content, image, category_id, updated_at, created_at").
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Order("updated_at DESC").
		Limit(limit).
		Find(&latest).Error
	return buildClientListPosts(siteSetting, latest)
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
	var total int64
	perPage := client.ListPageSize
	page := client.ParseListPage(c)
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
	latestRows := loadLatestSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitLatest)
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
	data["latest_posts"] = latestRows
	data["list_kicker"] = "Kategori"
	data["list_title"] = heading
	data["list_lead"] = metaDesc
	data["pagination_links"] = client.BuildPaginationLinks(page, totalPages, client.CategoryPublicWebPath(*category))
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	return renderPortfolioFile(c, db, http.StatusOK, "blog.html", data)
}

func serveClientTagPage(c *gin.Context, db *gorm.DB, tag *entity.Tag) error {
	perPage := client.ListPageSize
	page := client.ParseListPage(c)
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
	latestRows := loadLatestSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitLatest)
	title := strings.TrimSpace(tag.Name)
	if title == "" {
		title = strings.TrimSpace(tag.Slug)
	}
	metaTitle := "Tag: " + title + clientBrowserTitleSuffix(siteSetting)
	data := baseClientData(c, db, "Tag: "+title, metaTitle, "Kumpulan artikel dengan tag "+title)
	data["schema_page_type"] = "tag"
	data["tag_name"] = title
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["latest_posts"] = latestRows
	data["list_kicker"] = "Tag"
	data["list_title"] = title
	data["list_lead"] = "Artikel dengan tag " + title + "."
	tagPath := "/tag/" + url.PathEscape(strings.TrimSpace(tag.Slug))
	data["pagination_links"] = client.BuildPaginationLinks(page, totalPages, tagPath)
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	return renderPortfolioFile(c, db, http.StatusOK, "blog.html", data)
}

func serveClientPostsPage(c *gin.Context, db *gorm.DB) error {
	var total int64
	if err := db.Model(&entity.Post{}).
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Count(&total).Error; err != nil {
		return err
	}
	perPage := client.ListPageSize
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	page := client.ParseListPage(c)
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
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Order("updated_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts).Error; err != nil {
		return err
	}
	siteSetting := loadSiteSetting(db)
	entity.NormalizeSidebarListLimits(&siteSetting)
	latestRows := loadLatestSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitLatest)
	metaTitle := "Artikel" + clientBrowserTitleSuffix(siteSetting)
	data := baseClientData(c, db, "Artikel", metaTitle, "Daftar artikel yang sudah terbit.")
	data["schema_page_type"] = "posts"
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["latest_posts"] = latestRows
	data["pagination_links"] = client.BuildPaginationLinks(page, totalPages, "/posts")
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	return renderPortfolioFile(c, db, http.StatusOK, "blog.html", data)
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

	var total int64
	if err := db.Model(&entity.Post{}).
		Where("author_id = ?", author.ID).
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Count(&total).Error; err != nil {
		return err
	}
	perPage := client.ListPageSize
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}
	page := client.ParseListPage(c)
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
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Order("updated_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts).Error; err != nil {
		return err
	}

	siteSetting := loadSiteSetting(db)
	entity.NormalizeSidebarListLimits(&siteSetting)
	latestRows := loadLatestSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitLatest)
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
		if raw := strings.TrimSpace(*author.Image); raw != "" {
			if img := avatarResizeURL(siteSetting, raw); img != "" && img != BLANK_IMG {
				data["author_img"] = img
			}
		}
	}
	data["posts"] = buildClientListPosts(siteSetting, posts)
	data["latest_posts"] = latestRows
	data["list_kicker"] = "Penulis"
	data["list_title"] = authorName
	lead := strings.TrimSpace(author.Profile)
	if lead == "" {
		lead = "Artikel dari " + authorName + "."
	}
	data["list_lead"] = lead
	data["pagination_links"] = client.BuildPaginationLinks(page, totalPages, client.AuthorPublicListPath(slug))
	data["pagination_page"] = page
	data["pagination_total_pages"] = totalPages
	return renderPortfolioFile(c, db, http.StatusOK, "blog.html", data)
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

	viewerIP := c.ClientIP()
	if viewerIP != "" {
		day := time.Now().UTC().Format("2006-01-02")
		pv := entity.PostView{PostID: post.ID, ViewerIP: viewerIP, ViewDate: day}
		_ = db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}, {Name: "viewer_ip"}, {Name: "view_date"}},
			DoNothing: true,
		}).Create(&pv).Error
	}

	var metaLine string
	if !post.UpdatedAt.IsZero() {
		t := post.UpdatedAt
		metaLine = formatPostDateLocalized(&t)
	} else if !post.CreatedAt.IsZero() {
		t := post.CreatedAt
		metaLine = formatPostDateLocalized(&t)
	}

	catName := ""
	catSlug := ""
	catHref := ""
	if post.Category.ID != 0 && post.Category.Enabled {
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
		if raw := strings.TrimSpace(*author.Image); raw != "" {
			authorImg = avatarResizeURL(siteSetting, raw)
			if authorImg == BLANK_IMG {
				authorImg = ""
			}
		}
	}
	authorBio := strings.TrimSpace(author.Profile)
	authorPath := authorPublicPathFromUser(author)

	heroURL := postHeroURL(siteSetting, post.Image, post.Content)

	metaImg := postCoverRawURL(post.Image, post.Content)
	if metaImg == "" {
		metaImg = siteMeta.DefaultImage
	}

	entity.NormalizeSidebarListLimits(&siteSetting)
	latestRows := loadLatestSidebarPosts(db, siteSetting, siteSetting.SidebarListLimitLatest)
	relatedPosts := loadArticleRelatedPosts(db, post, articleRelatedLimit)
	articleRelatedRows := buildClientListPosts(siteSetting, relatedPosts)

	postSource := ""
	if post.Source != nil {
		postSource = strings.TrimSpace(*post.Source)
	}

	postBodyHTML := posthtml.BuildPublicPostBodyHTML(canonicalBaseURL(c), post.Content, displayTitle, siteSetting, db, post.ID, post.CategoryID)
	heroCaptionHTML := template.HTML(postutil.BuildHeroCaptionInnerHTML(
		siteSetting.ImageCaptionEnabled,
		siteSetting.ImageCaptionMode,
		siteSetting.ImageCaptionTemplate,
		*post,
		displayTitle,
	))

	data := gin.H{
		"title":                      metaHead,
		"meta_title":                 metaTitleForHead,
		"meta_description":           metaDesc,
		"meta_keywords":              metaKw,
		"meta_image":                 metaImg,
		"meta_author":                authorName,
		"og_type":                    "article",
		"site_name":                  siteSetting.SiteName,
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
		"font_css":                   template.CSS(siteSetting.FontFamily),
		"font_stylesheet":            googleFontStylesheet(siteSetting.FontFamily),
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
		"nav_categories":             loadNavCategories(db),
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
		"post_time_label":            metaLine,
		"author_name":                authorName,
		"author_img":                 authorImg,
		"avatar_letter":              avatarLetter,
		"author_path":                authorPath,
		"author_bio":                 authorBio,
		"hero_url":                   heroURL,
		"hero_caption_html":          heroCaptionHTML,
		"post_source":                postSource,
		"post_include_source":        post.IncludeSource,
		"source_disclaimer_mode":     siteSetting.SourceDisclaimerMode,
		"source_disclaimer_template": siteSetting.SourceDisclaimerTemplate,
		"latest_posts":               latestRows,
		"article_related_posts":      articleRelatedRows,
		"login_href":                 entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
	// Breadcrumb induk + sub hanya untuk mode URL 3 segmen; mode category (/mbg/judul) → Home > sub > judul.
	if entity.NormalizeArticleSlugMode(siteSetting.ArticleSlugMode) == entity.ArticleSlugModeCategoryNested &&
		post.Category.ID != 0 && post.Category.Enabled && !post.Category.IsRoot() && post.Category.Parent != nil && post.Category.Parent.Enabled {
		if pn := strings.TrimSpace(post.Category.Parent.Name); pn != "" {
			data["category_parent_name"] = pn
			data["category_parent_href"] = entity.CategoryPublicWebPath(*post.Category.Parent)
		}
	}
	mergeSidebarListLimitData(data, siteSetting)
	injectClientVerificationMeta(db, data)
	injectClientAbsoluteAssetURLs(c, siteSetting, data)
	data["layout"] = "article"
	return renderPortfolioFile(c, db, http.StatusOK, "article.html", data)
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

	r.SetFuncMap(template.FuncMap{
		"resizeImg": func(src string, w, h int) string {
			src = strings.TrimSpace(src)
			if src == "" || strings.HasPrefix(src, "data:") {
				return src
			}
			u, err := url.Parse(src)
			if err != nil {
				return src
			}
			q := u.Query()
			if w > 0 {
				q.Set("w", strconv.Itoa(w))
			}
			if h > 0 {
				q.Set("h", strconv.Itoa(h))
			}
			u.RawQuery = q.Encode()
			return u.String()
		},
	})

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
		strings.TrimSpace(row.Claude) != "" ||
		strings.TrimSpace(row.OpenCode) != "" ||
		row.OpenCodeLocal.Configured()
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
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
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
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
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

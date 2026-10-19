package posthtml

import (
	"regexp"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/alsoread"
)

var stripInnerTagsRE = regexp.MustCompile(`<[^>]*>`)
var leadingDupH1RE = regexp.MustCompile(`(?is)^\s*<h1\b[^>]*>([\s\S]*?)</h1>\s*`)

// StripLeadingDuplicateH1HTML removes one leading <h1> when its text duplicates the article title,
// mirroring SSR and client duplicate-title stripping.
func StripLeadingDuplicateH1HTML(html string, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return html
	}
	m := leadingDupH1RE.FindStringSubmatch(html)
	if m == nil {
		return html
	}
	inner := strings.TrimSpace(stripInnerTagsRE.ReplaceAllString(m[1], " "))
	inner = strings.ToLower(strings.Join(strings.Fields(inner), " "))
	t := strings.ToLower(title)
	if inner == t || strings.HasPrefix(inner, t) {
		return leadingDupH1RE.ReplaceAllString(html, "")
	}
	return html
}

func applyAlsoReadForDisplay(db *gorm.DB, html string, siteSetting entity.SiteSetting, postID, categoryID uint) string {
	if !siteSetting.AlsoReadEnabled {
		return alsoread.Strip(html)
	}
	if db == nil || postID == 0 {
		return html
	}
	injected, err := alsoread.InjectForDisplay(db, html, postID, categoryID)
	if err != nil {
		return alsoread.Strip(html)
	}
	return injected
}

// BuildPublicPostBodyHTML applies the same transforms as SSR article rendering: strip duplicate H1,
// inject in-body "Baca juga" when enabled, external-anchor policy, CDN image sizing, then ad slots.
// Pass postID > 0 with db so published articles get fresh Baca juga blocks at render time.
func BuildPublicPostBodyHTML(siteBaseURL string, rawContent string, displayTitle string, siteSetting entity.SiteSetting, db *gorm.DB, postID, categoryID uint) string {
	// Re-parse first so ChatGPT/UI chrome and orphan </div></section> cannot break page/footer layout.
	postBodyHTML := SanitizePublicArticleHTML(rawContent)
	postBodyHTML = WrapTablesForMobileScroll(postBodyHTML)
	postBodyHTML = StripLeadingDuplicateH1HTML(postBodyHTML, displayTitle)
	postBodyHTML = applyAlsoReadForDisplay(db, postBodyHTML, siteSetting, postID, categoryID)
	postBodyHTML = EnsureInternalAnchorsAbsolute(postBodyHTML, siteBaseURL)
	postBodyHTML = EnsureExternalAnchorsNofollowBlank(postBodyHTML, siteBaseURL)
	if siteSetting.ImgResizeEnabled {
		q := strings.TrimSpace(siteSetting.ImgResizeQuality)
		if q != "" {
			postBodyHTML = EnsureCDNImageResizeOnImgSrc(postBodyHTML, "", q)
		} else {
			postBodyHTML = StripCDNImageResizeFromImgSrc(postBodyHTML)
		}
	} else {
		postBodyHTML = StripCDNImageResizeFromImgSrc(postBodyHTML)
	}
	if slots := ParseArticleAdsJSON(siteSetting.ArticleAdsJSON); len(slots) > 0 {
		postBodyHTML = InjectArticleAds(postBodyHTML, slots)
	}
	return postBodyHTML
}

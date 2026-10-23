package posthtml

import (
	"regexp"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/alsoread"
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

// BuildPublicPostBodyHTML applies the same transforms as SSR article rendering: strip duplicate H1,
// external-anchor policy, CDN image sizing, then in-article ad slots from site settings JSON.
func BuildPublicPostBodyHTML(siteBaseURL string, rawContent string, displayTitle string, siteSetting entity.SiteSetting) string {
	postBodyHTML := StripLeadingDuplicateH1HTML(rawContent, displayTitle)
	if !siteSetting.AlsoReadEnabled {
		postBodyHTML = alsoread.Strip(postBodyHTML)
	}
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

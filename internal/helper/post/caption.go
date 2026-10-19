package postutil

import (
	"html"

	"net/url"

	"strings"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

var providerDisplayNames = map[string]string{

	entity.WebImageProviderPexels: "Pexels",

	entity.WebImageProviderGoogle: "Google",

	entity.WebImageProviderBing:     "Bing",

	entity.WebImageProviderBingNet:  "Bing.net",

	entity.WebImageProviderUnsplash: "Unsplash",
}

// NormalizeOrigin returns a supported image origin id or empty for legacy/unknown.

func NormalizeOrigin(origin string) string {

	switch strings.ToLower(strings.TrimSpace(origin)) {

	case entity.ImageOriginScrape,

		entity.ImageOriginWebSearch,

		entity.ImageOriginTemplate,

		entity.ImageOriginAI,

		entity.ImageOriginManual:

		return strings.ToLower(strings.TrimSpace(origin))

	default:

		return ""

	}

}

// SetPostImageOrigin assigns cover metadata on a post.

func SetPostImageOrigin(post *entity.Post, origin, detail string) {

	if post == nil {

		return

	}

	post.ImageOrigin = NormalizeOrigin(origin)

	post.ImageOriginDetail = strings.TrimSpace(detail)

	if len(post.ImageOriginDetail) > 200 {

		post.ImageOriginDetail = post.ImageOriginDetail[:200]

	}

}

// ClearPostImageOrigin clears cover metadata when the image is removed.

func ClearPostImageOrigin(post *entity.Post) {

	if post == nil {

		return

	}

	post.ImageOrigin = ""

	post.ImageOriginDetail = ""

}

// ProviderDisplayName maps provider slug to a human label.

func ProviderDisplayName(provider string) string {

	p := strings.ToLower(strings.TrimSpace(provider))

	if p == "" || p == "auto" {

		return ""

	}

	if label, ok := providerDisplayNames[p]; ok {

		return label

	}

	if len(p) == 0 {

		return ""

	}

	return strings.ToUpper(p[:1]) + p[1:]

}

// ScrapeSourceLabel derives a short label for scrape captions from article source URL and optional detail.

func ScrapeSourceLabel(articleSource, detail string) string {

	detail = strings.TrimSpace(detail)

	if detail != "" {

		if u, err := url.Parse(detail); err == nil && u.Host != "" {

			return u.Host

		}

		return detail

	}

	articleSource = strings.TrimSpace(articleSource)

	if articleSource == "" {

		return ""

	}

	if u, err := url.Parse(articleSource); err == nil && u.Host != "" {

		return u.Host

	}

	return articleSource

}

// BuildHeroCaptionInnerHTML returns figcaption inner HTML (escaped text + optional span), or empty when no caption.

func BuildHeroCaptionInnerHTML(enabled bool, mode, customTpl string, post entity.Post, title string) string {

	if !enabled {

		return ""

	}

	if post.Image == nil || strings.TrimSpace(*post.Image) == "" {

		return ""

	}

	if strings.EqualFold(strings.TrimSpace(mode), entity.ImageCaptionModeCustom) {

		return buildCustomCaptionInnerHTML(customTpl, post, title)

	}

	return buildDefaultCaptionInnerHTML(post, title)

}

func buildDefaultCaptionInnerHTML(post entity.Post, title string) string {

	origin := NormalizeOrigin(post.ImageOrigin)

	title = strings.TrimSpace(title)

	if title == "" {

		title = strings.TrimSpace(post.Title)

	}

	if origin == "" {

		return ilustrasiTitleCaptionHTML(title)

	}

	switch origin {

	case entity.ImageOriginWebSearch:

		if title == "" {

			return ""

		}

		provider := ProviderDisplayName(post.ImageOriginDetail)

		inner := fotoTitleCaptionHTML(title)

		if inner == "" {

			return ""

		}

		if provider != "" {

			inner += ` <span class="blog-article-hero-caption-credit">(Illustration by ` + html.EscapeString(provider) + `)</span>`

		}

		return inner

	case entity.ImageOriginAI, entity.ImageOriginTemplate, entity.ImageOriginManual:

		return ilustrasiTitleCaptionHTML(title)

	case entity.ImageOriginScrape:

		return fotoTitleCaptionHTML(title)

	default:

		return ""

	}

}

func buildCustomCaptionInnerHTML(customTpl string, post entity.Post, title string) string {

	tpl := strings.TrimSpace(customTpl)

	if tpl == "" {

		tpl = entity.DefaultImageCaptionTemplate

	}

	title = strings.TrimSpace(title)

	if title == "" {

		title = strings.TrimSpace(post.Title)

	}

	provider := ProviderDisplayName(post.ImageOriginDetail)

	source := ScrapeSourceLabel(sourceString(post.Source), post.ImageOriginDetail)

	creditHTML := ""

	if provider != "" {

		creditHTML = ` <span class="blog-article-hero-caption-credit">(Illustration by ` + html.EscapeString(provider) + `)</span>`

	}

	escaped := html.EscapeString(tpl)

	escaped = strings.ReplaceAll(escaped, "{{title}}", html.EscapeString(title))

	escaped = strings.ReplaceAll(escaped, "{{provider}}", html.EscapeString(provider))

	escaped = strings.ReplaceAll(escaped, "{{source}}", html.EscapeString(source))

	escaped = strings.ReplaceAll(escaped, "{{credit}}", creditHTML)

	return escaped

}

func fotoTitleCaptionHTML(title string) string {

	title = strings.TrimSpace(title)

	if title == "" {

		return ""

	}

	return "Foto: " + html.EscapeString(title) + "."

}

func ilustrasiTitleCaptionHTML(title string) string {

	title = strings.TrimSpace(title)

	if title == "" {

		return ""

	}

	return "Foto: Ilustrasi " + html.EscapeString(title) + "."

}

func sourceString(src *string) string {

	if src == nil {

		return ""

	}

	return strings.TrimSpace(*src)

}

package posthtml

import (
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/nikiaprian/tirtayasa/internal/media"
)

// EnsureCDNImageResizeOnImgSrc rewrites <img src> to self-hosted /img resize URLs when width/quality are set.
func EnsureCDNImageResizeOnImgSrc(fragmentHTML, width, quality string) string {
	fh := strings.TrimSpace(fragmentHTML)
	if fh == "" {
		return fragmentHTML
	}
	width = strings.TrimSpace(width)
	quality = strings.TrimSpace(quality)
	if width == "" && quality == "" {
		return fragmentHTML
	}

	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_tirtayasa_post_root">` + fragmentHTML + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return fragmentHTML
	}
	doc.Find("#_tirtayasa_post_root img[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if out := withImageResize(src, width, quality); out != "" && out != strings.TrimSpace(src) {
			s.SetAttr("src", out)
		}
	})
	out, err := doc.Find("#_tirtayasa_post_root").Html()
	if err != nil {
		return fragmentHTML
	}
	return out
}

// StripCDNImageResizeFromImgSrc removes /img and legacy /cdn-cgi/image/ wrappers from <img src>.
func StripCDNImageResizeFromImgSrc(fragmentHTML string) string {
	fh := strings.TrimSpace(fragmentHTML)
	if fh == "" {
		return fragmentHTML
	}
	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_tirtayasa_post_root">` + fragmentHTML + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return fragmentHTML
	}
	doc.Find("#_tirtayasa_post_root img[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if out := media.StripResizePrefixes(src); out != "" && out != strings.TrimSpace(src) {
			s.SetAttr("src", out)
		}
	})
	out, err := doc.Find("#_tirtayasa_post_root").Html()
	if err != nil {
		return fragmentHTML
	}
	return out
}

func withImageResize(raw, width, quality string) string {
	src := strings.TrimSpace(raw)
	if src == "" {
		return src
	}
	return media.BuildResizeURL(media.StripResizePrefixes(src), width, "", quality)
}

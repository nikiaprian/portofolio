package scraper

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"image"
	"image/color"
	imdraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
	"github.com/gen2brain/webp"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"gorm.io/gorm"
)

const (
	bulkCoverWidth  = 1024
	bulkCoverHeight = 1024
)

var (
	bulkBoldFontOnce sync.Once
	bulkBoldFontTTF  *opentype.Font
)

func generateBulkCoverImageForScraper(
	ctx context.Context,
	db *gorm.DB,
	store storage.ObjectStore,
	title string,
	slug string,
) (string, error) {
	if ctx == nil || db == nil {
		return "", errors.New("missing context or db")
	}

	accent := bulkAccentColorFromDB(db)
	var tpl image.Image
	watermarkDomain := bulkWatermarkDomainFromDB(db)

	var aiSetting entity.AISetting
	if err := db.First(&aiSetting).Error; err == nil {
		u := pickRandomBulkTemplateURL(aiSetting.BulkCoverBackgroundURL)
		if u != "" {
			img, err := decodeBulkTemplateImage(ctx, u)
			if err == nil {
				tpl = img
			}
		}
	}

	raw, err := renderBulkCoverWEBP(title, accent, tpl, watermarkDomain)
	if err != nil {
		return "", err
	}
	maxEdge, webpQ := repositories.PostImageStoreLimitsFromDB(db)
	enc, err := media.EncodePostUploadWithStoreCap(raw, "image/webp", maxEdge, webpQ)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(slug) == "" {
		return "", errors.New("empty slug")
	}
	filename := slug + "-bulk-cover" + enc.Ext

	if store != nil {
		key := "posts/" + filename
		url, err := store.Put(ctx, key, bytes.NewReader(enc.Data), enc.ContentType)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(url), nil
	}

	uploadDir := scrapedPostUploadDir()
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(uploadDir, filename)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		return "", err
	}
	return postImageURLBaseFromScraper() + "/" + filename, nil
}

func postImageURLBaseFromScraper() string {
	// keep URL base consistent with remote_image.go upload path.
	return scrapedImageURLBase
}

func pickRandomBulkTemplateURL(raw string) string {
	items := parseBulkTemplateURLs(raw)
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	var b [1]byte
	if _, err := crand.Read(b[:]); err != nil {
		return items[0]
	}
	return items[int(b[0])%len(items)]
}

func parseBulkTemplateURLs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func bulkAccentColorFromDB(db *gorm.DB) color.RGBA {
	if c, ok := parseHexColor(strings.TrimSpace(repositories.SiteSettingOrDefault(db).AccentColor)); ok {
		return c
	}
	return color.RGBA{R: 198, G: 40, B: 40, A: 255}
}

func bulkWatermarkDomainFromDB(db *gorm.DB) string {
	_ = db
	return ""
}

func decodeBulkTemplateImage(ctx context.Context, rawURL string) (image.Image, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("empty template URL")
	}
	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return decodeBulkTemplateHTTP(ctx, rawURL)
	}
	if strings.HasPrefix(rawURL, "/") {
		return decodeBulkTemplateLocal(rawURL)
	}
	return nil, fmt.Errorf("unsupported template URL (use /static/... or https://)")
}

func decodeBulkTemplateHTTP(ctx context.Context, rawURL string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching template", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 6*1024*1024))
	if err != nil {
		return nil, err
	}
	return decodeImageAllFormats(body)
}

func decodeBulkTemplateLocal(webPath string) (image.Image, error) {
	webPath = strings.TrimSpace(webPath)
	if !strings.HasPrefix(webPath, "/static/") {
		return nil, fmt.Errorf("local template must start with /static/")
	}
	if i := strings.Index(webPath, "?"); i >= 0 {
		webPath = webPath[:i]
	}
	rel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(webPath)), "/")
	full := filepath.Join("public", filepath.FromSlash(rel))
	absPub, err := filepath.Abs("public")
	if err != nil {
		return nil, err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return nil, err
	}
	relFull, err := filepath.Rel(absPub, absFull)
	if err != nil || strings.HasPrefix(relFull, "..") {
		return nil, fmt.Errorf("invalid template path")
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	return decodeImageAllFormats(body)
}

func decodeImageAllFormats(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err == nil {
		return img, nil
	}
	img, err = webp.Decode(bytes.NewReader(b))
	if err == nil {
		return img, nil
	}
	return nil, fmt.Errorf("decode image: %w", err)
}

func renderBulkCoverWEBP(title string, accent color.RGBA, templateImg image.Image, watermarkDomain string) ([]byte, error) {
	var canvas *image.RGBA
	if templateImg != nil {
		canvas = bulkTemplateToCanvas(templateImg)
	} else {
		canvas = image.NewRGBA(image.Rect(0, 0, bulkCoverWidth, bulkCoverHeight))
		drawBulkGradientBackground(canvas, accent)
	}
	drawBulkFooterOverlay(canvas)
	drawBulkTitleText(canvas, title, templateImg != nil)
	drawBulkWatermarkText(canvas, watermarkDomain)

	var out bytes.Buffer
	if err := webp.Encode(&out, canvas, webp.Options{
		Lossless: true,
		Method:   4,
	}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func bulkTemplateToCanvas(src image.Image) *image.RGBA {
	scaled := media.ScalePostImageToEdgeRange(src)
	return media.AsRGBA(scaled)
}

func drawBulkGradientBackground(img *image.RGBA, accent color.RGBA) {
	darkBlack := color.RGBA{R: 10, G: 10, B: 10, A: 255}
	darkAccent := color.RGBA{
		R: uint8(float64(accent.R) * 0.62),
		G: uint8(float64(accent.G) * 0.62),
		B: uint8(float64(accent.B) * 0.62),
		A: 255,
	}
	for y := 0; y < bulkCoverHeight; y++ {
		for x := 0; x < bulkCoverWidth; x++ {
			t := float64(x+y) / float64(bulkCoverWidth+bulkCoverHeight)
			r := uint8(float64(darkBlack.R)*(1-t) + float64(darkAccent.R)*t)
			g := uint8(float64(darkBlack.G)*(1-t) + float64(darkAccent.G)*t)
			b := uint8(float64(darkBlack.B)*(1-t) + float64(darkAccent.B)*t)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
}

func drawBulkFooterOverlay(img *image.RGBA) {
	// Full-canvas transparent overlay for better readability.
	b := img.Bounds()
	imdraw.Draw(img, b, &image.Uniform{
		C: color.RGBA{R: 0, G: 0, B: 0, A: 51},
	}, image.Point{}, imdraw.Over)
}

func drawBulkTitleText(img *image.RGBA, rawTitle string, useTemplate bool) {
	_ = useTemplate // keep signature stable; readability overlay handled globally.

	cw := img.Bounds().Dx()
	ch := img.Bounds().Dy()
	title := strings.Join(strings.Fields(strings.TrimSpace(rawTitle)), " ")
	if title == "" {
		title = "Untitled Post"
	}

	ly := media.BulkCoverTextLayoutForBounds(cw, ch)
	face := bulkBoldFace(ly.TitleSizePt)
	lineHeight := face.Metrics().Height.Ceil() + ly.TitleLineGap
	paddingX := ly.TitlePaddingX

	maxWidth := cw - (paddingX * 2)
	if maxWidth < 40 {
		maxWidth = cw - 8
		if maxWidth < 1 {
			maxWidth = 1
		}
	}
	lines := wrapTitleLines(title, face, maxWidth, 4)
	if len(lines) == 0 {
		lines = []string{"Untitled Post"}
	}

	blockHeight := (len(lines) * lineHeight) - 5
	startY := (ch-blockHeight)/2 + face.Metrics().Ascent.Ceil()

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}),
		Face: face,
	}
	for i, line := range lines {
		w := d.MeasureString(line).Ceil()
		x := (cw - w) / 2
		if x < paddingX {
			x = paddingX
		}
		d.Dot = fixed.P(x, startY+(i*lineHeight))
		d.DrawString(line)
	}
}

func drawBulkWatermarkText(img *image.RGBA, domain string) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return
	}
	cw := img.Bounds().Dx()
	ch := img.Bounds().Dy()
	ly := media.BulkCoverTextLayoutForBounds(cw, ch)
	face := bulkBoldFace(ly.WatermarkSizePt)
	metrics := face.Metrics()
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}),
		Face: face,
	}
	textW := d.MeasureString(domain).Ceil()
	paddingX := ly.WMPaddingX
	paddingY := ly.WMPaddingY
	x := cw - textW - paddingX
	if x < paddingX {
		x = paddingX
	}
	y := ch - paddingY + metrics.Descent.Ceil()
	d.Dot = fixed.P(x, y)
	d.DrawString(domain)
}

func bulkBoldFace(size float64) font.Face {
	bulkBoldFontOnce.Do(func() {
		f, err := opentype.Parse(gobold.TTF)
		if err == nil {
			bulkBoldFontTTF = f
		}
	})
	if bulkBoldFontTTF == nil {
		return basicfont.Face7x13
	}
	face, err := opentype.NewFace(bulkBoldFontTTF, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return basicfont.Face7x13
	}
	return face
}

func wrapTitleLines(s string, face font.Face, maxWidth, maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 1
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	d := &font.Drawer{Face: face}
	lines := make([]string, 0, maxLines)
	current := ""
	for _, w := range words {
		candidate := w
		if current != "" {
			candidate = current + " " + w
		}
		if d.MeasureString(candidate).Ceil() <= maxWidth {
			current = candidate
			continue
		}
		if current == "" {
			current = truncateToWidth(w, d, maxWidth)
		}
		lines = append(lines, current)
		if len(lines) >= maxLines {
			return truncateLastLine(lines, d, maxWidth)
		}
		current = w
		if d.MeasureString(current).Ceil() > maxWidth {
			current = truncateToWidth(current, d, maxWidth)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return truncateLastLine(lines, d, maxWidth)
}

func truncateToWidth(s string, d *font.Drawer, maxWidth int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	runes := []rune(s)
	for len(runes) > 1 && d.MeasureString(string(runes)+"...").Ceil() > maxWidth {
		runes = runes[:len(runes)-1]
	}
	if d.MeasureString(s).Ceil() <= maxWidth {
		return s
	}
	return strings.TrimSpace(string(runes)) + "..."
}

func truncateLastLine(lines []string, d *font.Drawer, maxWidth int) []string {
	if len(lines) == 0 {
		return lines
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return lines
	}
	if d.MeasureString(last).Ceil() <= maxWidth {
		return lines
	}
	// Avoid creating something empty due to weird font metrics.
	trimmed := strings.TrimSpace(truncateToWidth(last, d, maxWidth))
	if utf8.RuneCountInString(trimmed) == 0 {
		return lines
	}
	lines[len(lines)-1] = trimmed
	return lines
}

func parseHexColor(s string) (color.RGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return color.RGBA{}, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{
		R: uint8(n >> 16),
		G: uint8((n >> 8) & 0xff),
		B: uint8(n & 0xff),
		A: 255,
	}, true
}

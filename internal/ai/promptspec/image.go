// Package promptspec derives image API parameters from natural-language prompts
// (dimensions, aspect ratios, keywords) without importing provider SDKs.
package promptspec

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Default blog cover target size (landscape ~3:2). OpenAI gpt-image-1 has no 1024×683;
// we request 1536×1024 (same aspect) then normalize to these pixels before upload.
const (
	DefaultCoverWidth          = 1024
	DefaultCoverHeight         = 683
	DefaultOpenAIImageSize     = "1536x1024"
	DefaultGeminiAspectRatio   = "3:2"
	DefaultOpenAIQuality       = "medium" // biasa — bukan tinggi
	DefaultOpenAIOutputFormat = "webp" // default API output; prompt bisa menimpa (png/jpeg/webp)
)

// RasterMIMEForOutputFormat maps OpenAIOutputFormat (webp|png|jpeg|jpg|"") to a decode MIME type.
// Empty or unknown values use WebP to match DefaultOpenAIOutputFormat.
func RasterMIMEForOutputFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "png":
		return "image/png"
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/webp"
	}
}

// ImageParams holds provider-specific fields resolved from the prompt.
type ImageParams struct {
	OpenAIImageSize   string
	GeminiAspectRatio string
	// StoreWidth and StoreHeight, when both > 0, resize the generated raster to this
	// exact size (cover crop) before WebP/JPEG encode for upload.
	StoreWidth  int
	StoreHeight int
	// OpenAIQuality is one of: low, medium, high (GPT image models).
	OpenAIQuality string
	// OpenAIOutputFormat is one of: webp, png, jpeg (GPT image output).
	OpenAIOutputFormat string
}

func bareGeometryDefaults() ImageParams {
	return ImageParams{
		OpenAIImageSize:   DefaultOpenAIImageSize,
		GeminiAspectRatio: DefaultGeminiAspectRatio,
		StoreWidth:        DefaultCoverWidth,
		StoreHeight:       DefaultCoverHeight,
	}
}

// ImageFromPrompt returns geometry and output options: defaults apply unless the prompt
// mentions size/ratio, quality (tinggi/rendah/sedang, high/low/medium), or format (webp/png/jpeg,
// including Indonesian phrases like "format png", "output jpeg").
func ImageFromPrompt(prompt string) ImageParams {
	low := strings.ToLower(strings.TrimSpace(prompt))
	var p ImageParams
	if low == "" {
		p = bareGeometryDefaults()
	} else if w, h, ok := parsePixelDimensions(prompt); ok {
		p = paramsFromDimensions(w, h)
	} else if a, b, ok := parseExplicitRatio(low); ok {
		p = paramsFromRatio(a, b)
	} else if containsAny(low, "persegi", "square", "satu banding satu", "kotak") {
		w, h := storeDimsForAR(1)
		p = ImageParams{
			OpenAIImageSize:   "1024x1024",
			GeminiAspectRatio: "1:1",
			StoreWidth:        w,
			StoreHeight:       h,
		}
	} else if containsAny(low, "portrait", "vertikal", "potret", "vertical") {
		w, h := storeDimsForAR(9.0 / 16.0)
		p = ImageParams{
			OpenAIImageSize:   "1024x1536",
			GeminiAspectRatio: "9:16",
			StoreWidth:        w,
			StoreHeight:       h,
		}
	} else if containsAny(low, "landscape", "horizontal", "lanskap", "wide") {
		p = bareGeometryDefaults()
	} else {
		p = bareGeometryDefaults()
	}
	applyOutputOptions(low, &p)
	return p
}

func applyOutputOptions(low string, p *ImageParams) {
	q, f := parseOutputQualityAndFormat(low)
	p.OpenAIQuality = q
	p.OpenAIOutputFormat = f
}

var (
	reWordJPEG = regexp.MustCompile(`(?i)\b(jpeg|jpg)\b`)
	reWordPNG  = regexp.MustCompile(`(?i)\bpng\b`)
	reWordWebP = regexp.MustCompile(`(?i)\bwebp\b`)
	reWordHD   = regexp.MustCompile(`(?i)\bhd\b`)
)

func parseOutputQualityAndFormat(low string) (quality, format string) {
	quality = DefaultOpenAIQuality
	format = DefaultOpenAIOutputFormat
	if low == "" {
		return quality, format
	}
	// Quality: explicit API/output instructions only (avoid matching "berkualitas tinggi" in creative briefs).
	switch {
	case containsAny(low, "api kualitas tinggi", "output kualitas tinggi", "generate high quality", "image quality high", "quality: high", "quality high", "kualitas: tinggi", "kualitas render tinggi", "gambar hd", "ultra hd", "premium quality", "kualitas premium"):
		quality = "high"
	case containsAny(low, "kualitas rendah", "low quality", "quality: low", "quality low", "kualitas hemat", "draft quality", "ringan", "hemat token"):
		quality = "low"
	case containsAny(low, "kualitas sedang", "medium quality", "quality: medium", "quality medium", "kualitas normal", "kualitas biasa", "standard quality", "sedang saja", "kualitas api sedang"):
		quality = "medium"
	case reWordHD.MatchString(low) && !containsAny(low, "non-hd", "non hd"):
		quality = "high"
	}
	// Format: first matching token wins (jpeg before png before webp if multiple — user can put one intent).
	// English tokens + frasa Indonesia (format/berformat/tipe/output …).
	if reWordJPEG.MatchString(low) || formatHintID(low, "jpeg") {
		format = "jpeg"
	} else if reWordPNG.MatchString(low) || formatHintID(low, "png") {
		format = "png"
	} else if reWordWebP.MatchString(low) || formatHintID(low, "webp") {
		format = "webp"
	}
	return quality, format
}

func formatHintID(low, kind string) bool {
	switch kind {
	case "jpeg":
		return containsAny(low,
			"format jpg", "format jpeg", "berformat jpg", "berformat jpeg",
			"tipe jpg", "tipe jpeg", "output jpg", "output jpeg", "ekstensi jpg", "ekstensi jpeg",
		)
	case "png":
		return containsAny(low,
			"format png", "berformat png", "tipe png", "output png", "ekstensi png",
		)
	case "webp":
		return containsAny(low,
			"format webp", "berformat webp", "tipe webp", "output webp", "ekstensi webp",
		)
	default:
		return false
	}
}

var (
	rePixels = regexp.MustCompile(`(?i)\b(\d{2,4})\s*[x×*]\s*(\d{2,4})\b`)
	// Ratios like 16:9 — require second part 1–3 digits to reduce time-like false positives.
	reRatio = regexp.MustCompile(`(?i)\b(\d{1,2})\s*:\s*(\d{1,3})\b`)
)

func parsePixelDimensions(s string) (w, h int, ok bool) {
	m := rePixels.FindStringSubmatch(s)
	if len(m) != 3 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(m[1])
	h, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil || w < 8 || h < 8 || w > 8192 || h > 8192 {
		return 0, 0, false
	}
	return w, h, true
}

// parseExplicitRatio finds the first a:b that looks like an aspect ratio (not pixel dimensions).
func parseExplicitRatio(low string) (a, b int, ok bool) {
	for _, m := range reRatio.FindAllStringSubmatch(low, -1) {
		if len(m) != 3 {
			continue
		}
		a, err1 := strconv.Atoi(m[1])
		b, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil || a < 1 || b < 1 || a > 100 || b > 100 {
			continue
		}
		return a, b, true
	}
	return 0, 0, false
}

func paramsFromDimensions(w, h int) ImageParams {
	ar := float64(w) / float64(h)
	p := paramsFromAspect(ar)
	p.StoreWidth, p.StoreHeight = clampStoreDims(w, h)
	return p
}

func paramsFromRatio(a, b int) ImageParams {
	if a < 1 || b < 1 {
		return bareGeometryDefaults()
	}
	ar := float64(a) / float64(b)
	return paramsFromAspect(ar)
}

func paramsFromAspect(ar float64) ImageParams {
	if ar <= 0 || math.IsNaN(ar) || math.IsInf(ar, 0) {
		return bareGeometryDefaults()
	}
	var oai string
	switch {
	case ar < 0.92:
		oai = "1024x1536"
	case ar > 1.08:
		oai = "1536x1024"
	default:
		oai = "1024x1024"
	}
	sw, sh := storeDimsForAR(ar)
	return ImageParams{
		OpenAIImageSize:   oai,
		GeminiAspectRatio: nearestGeminiAspect(ar),
		StoreWidth:        sw,
		StoreHeight:       sh,
	}
}

func clampStoreDims(w, h int) (int, int) {
	const maxEdge = 2048
	if w > maxEdge || h > maxEdge {
		ar := float64(w) / float64(h)
		if ar >= 1 {
			w = maxEdge
			h = int(math.Round(float64(w) / ar))
		} else {
			h = maxEdge
			w = int(math.Round(float64(h) * ar))
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	return w, h
}

// storeDimsForAR returns width×height with long edge DefaultCoverWidth (landscape)
// or portrait height DefaultCoverWidth, matching the requested aspect ratio.
func storeDimsForAR(ar float64) (int, int) {
	if ar <= 0 || math.IsNaN(ar) || math.IsInf(ar, 0) {
		return DefaultCoverWidth, DefaultCoverHeight
	}
	if ar >= 1 {
		w := DefaultCoverWidth
		h := int(math.Round(float64(w) / ar))
		if h < 1 {
			h = 1
		}
		return w, h
	}
	h := DefaultCoverWidth
	w := int(math.Round(float64(h) * ar))
	if w < 1 {
		w = 1
	}
	return w, h
}

// geminiAspectTable must stay in sync with Gemini image generation supported ratios.
var geminiAspectTable = []struct {
	label string
	ar    float64
}{
	{"1:1", 1},
	{"2:3", 2.0 / 3.0},
	{"3:2", 3.0 / 2.0},
	{"3:4", 3.0 / 4.0},
	{"4:3", 4.0 / 3.0},
	{"4:5", 4.0 / 5.0},
	{"5:4", 5.0 / 4.0},
	{"9:16", 9.0 / 16.0},
	{"16:9", 16.0 / 9.0},
	{"21:9", 21.0 / 9.0},
}

func nearestGeminiAspect(ar float64) string {
	best := DefaultGeminiAspectRatio
	bestDist := math.MaxFloat64
	for _, row := range geminiAspectTable {
		d := math.Abs(math.Log(ar) - math.Log(row.ar))
		if d < bestDist {
			bestDist = d
			best = row.label
		}
	}
	return best
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

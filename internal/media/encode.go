package media

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/gen2brain/webp"
)

// ContentType is the MIME type for WebP produced when encoding succeeds.
const ContentType = "image/webp"

const (
	jpegWebPLossyQ = 82 // libwebp lossy (VP8); matches “good photo” default ~75–85
	jpegFallbackQ  = 85 // stdlib fallback when WebP encode fails
)

// Encoded is a normalized upload payload with matching extension and Content-Type.
type Encoded struct {
	Data        []byte
	ContentType string
	Ext         string
}

// EncodePostUpload decodes raster bytes, encodes with libwebp (via gen2brain/webp: WASM or system lib),
// and falls back to JPEG/PNG/GIF/original WebP bytes if encoding fails.
// Images are not resized (used e.g. for profile avatars).
//
// JPEG → lossy WebP (smaller). PNG, GIF, WebP input → lossless WebP so alpha and sharp UI graphics stay intact.
func EncodePostUpload(raw []byte, inputMIME string) (*Encoded, error) {
	return encodePostUpload(raw, inputMIME, 0, 0, 0)
}

// EncodePostUploadForPost is like EncodePostUpload but normalizes the longest side
// to stay within PostImageMinEdgePx..PostImageMaxEdgePx.
func EncodePostUploadForPost(raw []byte, inputMIME string) (*Encoded, error) {
	return encodePostUpload(raw, inputMIME, PostImageMinEdgePx, PostImageMaxEdgePx, 0)
}

// EncodePostUploadWithStoreCap applies an optional maximum longest-edge cap (site_settings.stored_image_max_width)
// before encoding, then optional lossy WebP quality (stored_image_quality 1–100). Storage-side only; separate from
// Cloudflare /cdn-cgi/image (image_resize_*).
func EncodePostUploadWithStoreCap(raw []byte, inputMIME string, maxLongestEdge int, webpQuality int) (*Encoded, error) {
	if maxLongestEdge > 0 {
		var err error
		raw, inputMIME, err = DownscaleRasterMaxLongestEdge(raw, inputMIME, maxLongestEdge)
		if err != nil {
			return nil, err
		}
	}
	return encodePostUpload(raw, inputMIME, 0, 0, webpQuality)
}

func encodePostUpload(raw []byte, inputMIME string, minEdge, maxEdge int, webpQualityOverride int) (*Encoded, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	vipsScaled := false
	if maxEdge > 0 {
		if scaled, err := vipsScaleToEdgeRange(raw, minEdge, maxEdge); err == nil {
			vipsScaled = true
			if !bytes.Equal(scaled, raw) {
				raw = scaled
				inputMIME = "image/png"
			}
		}
	}
	img, err := decodeInput(raw, inputMIME)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if maxEdge > 0 && !vipsScaled {
		img = scaleToEdgeRange(img, minEdge, maxEdge)
	}
	raster := flattenToNRGBA(img)

	q := jpegWebPLossyQ
	if webpQualityOverride >= 1 && webpQualityOverride <= 100 {
		q = webpQualityOverride
	}
	useLossy := inputMIME == "image/jpeg" || (webpQualityOverride >= 1 && webpQualityOverride <= 100)

	var encOpts webp.Options
	if useLossy {
		encOpts = webp.Options{
			Quality:  q,
			Lossless: false,
			Method:   4,
		}
	} else {
		encOpts = webp.Options{
			Lossless: true,
			Method:   4,
		}
	}

	var webpBuf bytes.Buffer
	if err := webp.Encode(&webpBuf, raster, encOpts); err == nil && webpBuf.Len() > 0 {
		return &Encoded{
			Data:        webpBuf.Bytes(),
			ContentType: ContentType,
			Ext:         ".webp",
		}, nil
	}

	return encodeFallback(raster, inputMIME, raw)
}

// Dimensions returns decoded raster dimensions for supported upload MIME types.
func Dimensions(raw []byte, inputMIME string) (int, int, error) {
	if len(raw) == 0 {
		return 0, 0, fmt.Errorf("empty image")
	}
	img, err := decodeInput(raw, inputMIME)
	if err != nil {
		return 0, 0, fmt.Errorf("decode image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid image dimensions")
	}
	return w, h, nil
}

func decodeInput(raw []byte, inputMIME string) (image.Image, error) {
	r := bytes.NewReader(raw)
	switch inputMIME {
	case "image/jpeg":
		return jpeg.Decode(r)
	case "image/png":
		return png.Decode(r)
	case "image/gif":
		return gif.Decode(r)
	case "image/webp":
		return webp.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported image type %q", inputMIME)
	}
}

func flattenToNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}

func encodeFallback(img image.Image, inputMIME string, original []byte) (*Encoded, error) {
	var buf bytes.Buffer
	switch inputMIME {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegFallbackQ}); err != nil {
			return nil, fmt.Errorf("encode jpeg fallback: %w", err)
		}
		return &Encoded{Data: buf.Bytes(), ContentType: "image/jpeg", Ext: ".jpg"}, nil
	case "image/png":
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode png fallback: %w", err)
		}
		return &Encoded{Data: buf.Bytes(), ContentType: "image/png", Ext: ".png"}, nil
	case "image/gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return nil, fmt.Errorf("encode gif fallback: %w", err)
		}
		return &Encoded{Data: buf.Bytes(), ContentType: "image/gif", Ext: ".gif"}, nil
	case "image/webp":
		out := make([]byte, len(original))
		copy(out, original)
		return &Encoded{Data: out, ContentType: ContentType, Ext: ".webp"}, nil
	default:
		return nil, fmt.Errorf("unsupported image type %q", inputMIME)
	}
}

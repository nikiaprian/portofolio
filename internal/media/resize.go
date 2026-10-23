package media

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	// PostImageMinEdgePx is the minimum longest side (width or height) for stored post images.
	PostImageMinEdgePx = 720
	// PostImageMaxEdgePx is the maximum longest side (width or height) for stored post images.
	PostImageMaxEdgePx = 1024
)

func scaleToEdgeRange(src image.Image, minEdge, maxEdge int) image.Image {
	if src == nil {
		return src
	}
	if minEdge <= 0 {
		minEdge = 1
	}
	if maxEdge < minEdge {
		maxEdge = minEdge
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	longest := w
	if h > longest {
		longest = h
	}
	if longest >= minEdge && longest <= maxEdge {
		return src
	}
	targetLongest := longest
	if longest < minEdge {
		targetLongest = minEdge
	}
	if longest > maxEdge {
		targetLongest = maxEdge
	}
	var nw, nh int
	if w >= h {
		nw = targetLongest
		nh = targetLongest * h / w
		if nh < 1 {
			nh = 1
		}
	} else {
		nh = targetLongest
		nw = targetLongest * w / h
		if nw < 1 {
			nw = 1
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// ScalePostImageToEdgeRange scales so the longest side lies in
// [PostImageMinEdgePx, PostImageMaxEdgePx], preserving aspect ratio.
func ScalePostImageToEdgeRange(src image.Image) image.Image {
	return scaleToEdgeRange(src, PostImageMinEdgePx, PostImageMaxEdgePx)
}

// ResizeCoverExact scales src with a "cover" fit (fills dw×dh) and center-crops to exact dw×dh.
func ResizeCoverExact(src image.Image, dw, dh int) image.Image {
	if src == nil || dw < 2 || dh < 2 {
		return src
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return src
	}
	sx := float64(dw) / float64(sw)
	sy := float64(dh) / float64(sh)
	s := sx
	if sy > s {
		s = sy
	}
	nw := int(math.Round(float64(sw) * s))
	nh := int(math.Round(float64(sh) * s))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, b, xdraw.Over, nil)
	x0 := (nw - dw) / 2
	y0 := (nh - dh) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	rect := image.Rect(0, 0, dw, dh)
	dst := image.NewNRGBA(rect)
	draw.Draw(dst, rect, scaled, image.Point{X: x0, Y: y0}, draw.Src)
	return dst
}

// ResizePNGToCoverExact decodes PNG bytes, applies [ResizeCoverExact], re-encodes as PNG.
func ResizePNGToCoverExact(raw []byte, dw, dh int) ([]byte, error) {
	return ResizeRasterToCoverExact(raw, "image/png", dw, dh)
}

// ResizeRasterToCoverExact decodes raster bytes (PNG/JPEG/WebP/GIF), applies [ResizeCoverExact], re-encodes as PNG.
func ResizeRasterToCoverExact(raw []byte, inputMIME string, dw, dh int) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	if dw < 2 || dh < 2 {
		return nil, fmt.Errorf("invalid target size %dx%d", dw, dh)
	}
	if out, err := vipsCoverExact(raw, dw, dh); err == nil {
		return out, nil
	}
	mime := strings.TrimSpace(inputMIME)
	if mime == "" {
		mime = http.DetectContentType(raw)
	}
	img, err := decodeInput(raw, mime)
	if err != nil {
		return nil, err
	}
	out := ResizeCoverExact(img, dw, dh)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// AsRGBA returns an RGBA bitmap the same size as src, for drawing or encoding.
func AsRGBA(src image.Image) *image.RGBA {
	if src == nil {
		return nil
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// scaleDownToMaxLongestEdge returns src unchanged when its longest side is already <= maxEdge.
// Otherwise scales down proportionally so the longest side equals maxEdge (never upscales).
func scaleDownToMaxLongestEdge(src image.Image, maxEdge int) image.Image {
	if src == nil || maxEdge < 2 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	longest := w
	if h > longest {
		longest = h
	}
	if longest <= maxEdge {
		return src
	}
	var nw, nh int
	if w >= h {
		nw = maxEdge
		nh = maxEdge * h / w
		if nh < 1 {
			nh = 1
		}
	} else {
		nh = maxEdge
		nw = maxEdge * w / h
		if nw < 1 {
			nw = 1
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// scaleDownToMaxBox downscales or center-crops to the delivery box.
// Only maxW: longest-edge cap. Only maxH: height cap. Both: cover crop to maxW×maxH.
func scaleDownToMaxBox(src image.Image, maxW, maxH int) image.Image {
	if src == nil {
		return src
	}
	if maxW > 0 && maxH <= 0 {
		return scaleDownToMaxLongestEdge(src, maxW)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	var nw, nh int
	if maxW <= 0 && maxH > 0 {
		if h <= maxH {
			return src
		}
		nh = maxH
		nw = maxH * w / h
		if nw < 1 {
			nw = 1
		}
	} else if maxW > 0 && maxH > 0 {
		return ResizeCoverExact(src, maxW, maxH)
	} else {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// DownscaleRasterMaxBox is a no-op when both maxW and maxH are <= 0 or the image already fits.
func DownscaleRasterMaxBox(raw []byte, inputMIME string, maxW, maxH int) ([]byte, string, error) {
	if (maxW <= 0 && maxH <= 0) || len(raw) == 0 {
		return raw, inputMIME, nil
	}
	if out, err := vipsDownscaleMaxBox(raw, maxW, maxH); err == nil {
		if len(out) == len(raw) && bytes.Equal(out, raw) {
			return raw, inputMIME, nil
		}
		return out, "image/png", nil
	}
	mime := strings.TrimSpace(inputMIME)
	if mime == "" {
		mime = http.DetectContentType(raw)
	}
	img, err := decodeInput(raw, mime)
	if err != nil {
		return nil, "", fmt.Errorf("decode for downscale: %w", err)
	}
	out := scaleDownToMaxBox(img, maxW, maxH)
	if out == img {
		return raw, mime, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

// DownscaleRasterMaxLongestEdge is a no-op when maxLongest <= 0 or the image already fits.
// Otherwise decodes raster bytes, scales down so the longest edge is at most maxLongest,
// and returns PNG bytes (and image/png). Used for server-side storage caps independent of
// Cloudflare /cdn-cgi/image delivery resizing.
func DownscaleRasterMaxLongestEdge(raw []byte, inputMIME string, maxLongest int) ([]byte, string, error) {
	if maxLongest <= 0 || len(raw) == 0 {
		return raw, inputMIME, nil
	}
	if out, err := vipsDownscaleMaxLongest(raw, maxLongest); err == nil {
		if len(out) == len(raw) && bytes.Equal(out, raw) {
			return raw, inputMIME, nil
		}
		return out, "image/png", nil
	}
	mime := strings.TrimSpace(inputMIME)
	if mime == "" {
		mime = http.DetectContentType(raw)
	}
	img, err := decodeInput(raw, mime)
	if err != nil {
		return nil, "", fmt.Errorf("decode for downscale: %w", err)
	}
	out := scaleDownToMaxLongestEdge(img, maxLongest)
	if out == img {
		return raw, mime, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

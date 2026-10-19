//go:build vips

package media

import (
	"fmt"
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
)

var (
	vipsOnce    sync.Once
	vipsReady   bool
	vipsInitErr error
)

// InitVips starts libvips (govips). Safe to call multiple times; no-op after first success.
// Call from main before serving uploads; optional if resize helpers lazy-init on first use.
func InitVips() error {
	vipsOnce.Do(func() {
		vipsInitErr = vips.Startup(nil)
		vipsReady = vipsInitErr == nil
	})
	return vipsInitErr
}

// ShutdownVips releases libvips resources. Call on process exit when InitVips succeeded.
func ShutdownVips() {
	if vipsReady {
		vips.Shutdown()
		vipsReady = false
	}
}

func ensureVips() error {
	if err := InitVips(); err != nil {
		return err
	}
	if !vipsReady {
		return fmt.Errorf("libvips not available")
	}
	return nil
}

func vipsLoadFromBuffer(raw []byte) (*vips.ImageRef, error) {
	img, err := vips.NewImageFromBuffer(raw)
	if err != nil {
		return nil, fmt.Errorf("vips load: %w", err)
	}
	if err := img.AutoRotate(); err != nil {
		img.Close()
		return nil, fmt.Errorf("vips autorotate: %w", err)
	}
	return img, nil
}

func vipsExportPNG(img *vips.ImageRef) ([]byte, error) {
	out, _, err := img.ExportPng(&vips.PngExportParams{
		Compression: 6,
		Interlace:   false,
	})
	if err != nil {
		return nil, fmt.Errorf("vips export png: %w", err)
	}
	return out, nil
}

// vipsDownscaleMaxLongest scales down so the longest edge is at most maxEdge (never upscales).
func vipsDownscaleMaxLongest(raw []byte, maxEdge int) ([]byte, error) {
	if err := ensureVips(); err != nil {
		return nil, err
	}
	img, err := vipsLoadFromBuffer(raw)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	w, h := img.Width(), img.Height()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}
	longest := w
	if h > longest {
		longest = h
	}
	if longest <= maxEdge {
		return raw, nil
	}
	scale := float64(maxEdge) / float64(longest)
	if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
		return nil, fmt.Errorf("vips resize: %w", err)
	}
	return vipsExportPNG(img)
}

// vipsDownscaleMaxBox scales down to fit within maxW×maxH (see scaleDownToMaxBox).
func vipsDownscaleMaxBox(raw []byte, maxW, maxH int) ([]byte, error) {
	if err := ensureVips(); err != nil {
		return nil, err
	}
	img, err := vipsLoadFromBuffer(raw)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	w, h := img.Width(), img.Height()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}
	scale := 1.0
	if maxW > 0 && maxH <= 0 {
		longest := w
		if h > longest {
			longest = h
		}
		if longest > maxW {
			scale = float64(maxW) / float64(longest)
		}
	} else if maxW <= 0 && maxH > 0 {
		if h > maxH {
			scale = float64(maxH) / float64(h)
		}
	} else if maxW > 0 && maxH > 0 {
		if err := img.ThumbnailWithSize(maxW, maxH, vips.InterestingCentre, vips.SizeBoth); err != nil {
			return nil, fmt.Errorf("vips cover: %w", err)
		}
		return vipsExportPNG(img)
	} else {
		return raw, nil
	}
	if scale >= 1 {
		return raw, nil
	}
	if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
		return nil, fmt.Errorf("vips resize: %w", err)
	}
	return vipsExportPNG(img)
}

// vipsCoverExact fills dw×dh with a center-cropped cover fit.
func vipsCoverExact(raw []byte, dw, dh int) ([]byte, error) {
	if err := ensureVips(); err != nil {
		return nil, err
	}
	img, err := vipsLoadFromBuffer(raw)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	if err := img.ThumbnailWithSize(dw, dh, vips.InterestingCentre, vips.SizeBoth); err != nil {
		return nil, fmt.Errorf("vips thumbnail: %w", err)
	}
	return vipsExportPNG(img)
}

// vipsScaleToEdgeRange scales so the longest side lies in [minEdge, maxEdge].
func vipsScaleToEdgeRange(raw []byte, minEdge, maxEdge int) ([]byte, error) {
	if err := ensureVips(); err != nil {
		return nil, err
	}
	img, err := vipsLoadFromBuffer(raw)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	w, h := img.Width(), img.Height()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}
	longest := w
	if h > longest {
		longest = h
	}
	if longest >= minEdge && longest <= maxEdge {
		return raw, nil
	}
	target := longest
	if longest < minEdge {
		target = minEdge
	}
	if longest > maxEdge {
		target = maxEdge
	}
	scale := float64(target) / float64(longest)
	if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
		return nil, fmt.Errorf("vips resize: %w", err)
	}
	return vipsExportPNG(img)
}

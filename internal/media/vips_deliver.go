//go:build vips

package media

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

func vipsDeliverResize(raw []byte, maxWidth, maxHeight, quality int) ([]byte, string, error) {
	if err := ensureVips(); err != nil {
		return nil, "", err
	}
	img, err := vipsLoadFromBuffer(raw)
	if err != nil {
		return nil, "", err
	}
	defer img.Close()

	if maxWidth >= deliveryMinWidth && maxHeight >= deliveryMinWidth {
		if err := img.ThumbnailWithSize(maxWidth, maxHeight, vips.InterestingCentre, vips.SizeBoth); err != nil {
			return nil, "", fmt.Errorf("vips cover: %w", err)
		}
	} else if maxWidth >= deliveryMinWidth || maxHeight >= deliveryMinWidth {
		w, h := img.Width(), img.Height()
		if w > 0 && h > 0 {
			scale := 1.0
			if maxWidth >= deliveryMinWidth && maxHeight <= 0 {
				longest := w
				if h > longest {
					longest = h
				}
				if longest > maxWidth {
					scale = float64(maxWidth) / float64(longest)
				}
			} else if maxWidth <= 0 && maxHeight >= deliveryMinWidth {
				if h > maxHeight {
					scale = float64(maxHeight) / float64(h)
				}
			}
			if scale > 0 && scale < 1 {
				if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
					return nil, "", fmt.Errorf("vips resize: %w", err)
				}
			}
		}
	}
	q := quality
	if q <= 0 || q > 100 {
		q = defaultDeliveryQ
	}
	out, _, err := img.ExportWebp(&vips.WebpExportParams{
		Quality:  q,
		Lossless: false,
		Effort:   4,
	})
	if err != nil {
		return nil, "", fmt.Errorf("vips export webp: %w", err)
	}
	return out, ContentType, nil
}

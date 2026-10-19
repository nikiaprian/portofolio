package media

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	deliveryMaxWidth  = 4096
	deliveryMaxHeight = 4096
	deliveryMinWidth  = 1
	defaultDeliveryQ  = 82
)

// DeliverResize decodes raster bytes, resizes for /img delivery, encodes WebP.
// When both maxWidth and maxHeight are set, center-crops (cover) to that box so output matches the frontend slot.
// With only one dimension, downscales on that axis (width-only uses longest-edge cap).
func DeliverResize(raw []byte, inputMIME string, maxWidth, maxHeight, quality int) ([]byte, string, error) {
	if len(raw) == 0 {
		return nil, "", fmt.Errorf("empty image")
	}
	mime := strings.TrimSpace(inputMIME)
	if mime == "" {
		mime = http.DetectContentType(raw)
	}
	if maxWidth > deliveryMaxWidth {
		maxWidth = deliveryMaxWidth
	}
	if maxHeight > deliveryMaxHeight {
		maxHeight = deliveryMaxHeight
	}
	q := quality
	if q <= 0 || q > 100 {
		q = defaultDeliveryQ
	}

	if out, ct, err := vipsDeliverResize(raw, maxWidth, maxHeight, q); err == nil {
		return out, ct, nil
	}

	var err error
	if maxWidth >= deliveryMinWidth && maxHeight >= deliveryMinWidth {
		cropped, cropErr := ResizeRasterToCoverExact(raw, mime, maxWidth, maxHeight)
		if cropErr != nil {
			return nil, "", cropErr
		}
		raw, mime = cropped, "image/png"
	} else if maxWidth >= deliveryMinWidth || maxHeight >= deliveryMinWidth {
		raw, mime, err = DownscaleRasterMaxBox(raw, mime, maxWidth, maxHeight)
		if err != nil {
			return nil, "", err
		}
	}
	enc, err := EncodePostUploadWithStoreCap(raw, mime, 0, q)
	if err != nil {
		return nil, "", fmt.Errorf("encode delivery webp: %w", err)
	}
	return enc.Data, enc.ContentType, nil
}

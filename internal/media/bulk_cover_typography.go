package media

import "math"

const (
	// bulkCoverTextBaselineLongestPx is the approximate longest-edge size when title (24pt)
	// and watermark (14pt) were chosen for smaller covers (~450px wide layouts).
	bulkCoverTextBaselineLongestPx = 450
	bulkCoverTitlePtBaseline       = 24.0
	bulkCoverWatermarkPtBaseline   = 14.0
)

// BulkCoverTextLayout holds scaled typography for server-rendered bulk / scraper covers.
type BulkCoverTextLayout struct {
	TitleSizePt     float64
	WatermarkSizePt float64
	TitlePaddingX   int
	TitleLineGap    int
	WMPaddingX      int
	WMPaddingY      int
}

// BulkCoverTextLayoutForBounds scales fonts and padding from bulkCoverTextBaselineLongestPx
// using max(width, height), clamped so very large canvases do not use absurd point sizes.
func BulkCoverTextLayoutForBounds(width, height int) BulkCoverTextLayout {
	longest := width
	if height > longest {
		longest = height
	}
	if longest < 1 {
		longest = 1
	}
	scale := float64(longest) / float64(bulkCoverTextBaselineLongestPx)
	if scale < 1 {
		scale = 1
	}
	const maxScale = 2.35
	if scale > maxScale {
		scale = maxScale
	}
	return BulkCoverTextLayout{
		TitleSizePt:     bulkCoverTitlePtBaseline * scale,
		WatermarkSizePt: bulkCoverWatermarkPtBaseline * scale,
		TitlePaddingX:   max(12, int(math.Round(22*scale))),
		TitleLineGap:    max(4, int(math.Round(6*scale))),
		WMPaddingX:      max(8, int(math.Round(12*scale))),
		WMPaddingY:      max(6, int(math.Round(10*scale))),
	}
}

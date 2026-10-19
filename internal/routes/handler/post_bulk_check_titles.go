package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/helper/text"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"gorm.io/gorm"
)

const bulkTitleSimilarityThreshold = text.BulkTitleThreshold

type bulkCheckTitleRow struct {
	ID    uint   `gorm:"column:id"`
	Title string `gorm:"column:title"`
}

// BulkCheckTitles finds bulk list lines ≥90% similar to existing posts or earlier lines in the list.
func (h *PostHandler) BulkCheckTitles(c *gin.Context) {
	var req dto.BulkCheckTitlesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inLines := parseBulkTitleLinesForCheck(req.TitlesText)
	if len(inLines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Isi minimal satu baris judul."})
		return
	}
	if len(inLines) > bulkMaxArticleTitleLines {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maksimal 1000 baris judul."})
		return
	}

	dbTitles := loadPostTitlesForBulkCheck(h.db)
	matches := make([]dto.BulkTitleSimilarityMatch, 0)

	for i, newTitle := range inLines {
		bestDB := findBestDBTitleMatch(newTitle, dbTitles)
		if bestDB != nil {
			matches = append(matches, dto.BulkTitleSimilarityMatch{
				Index:      i,
				NewTitle:   newTitle,
				OldTitle:   bestDB.Title,
				Source:     "database",
				Similarity: bestDB.Ratio,
				PostID:     &bestDB.ID,
			})
			continue
		}
		if prev := findBestEarlierListMatch(newTitle, inLines, i); prev != nil {
			matches = append(matches, dto.BulkTitleSimilarityMatch{
				Index:      i,
				NewTitle:   newTitle,
				OldTitle:   prev.Title,
				Source:     "list",
				Similarity: prev.Ratio,
			})
		}
	}

	c.JSON(http.StatusOK, dto.BulkCheckTitlesResponse{
		Threshold: bulkTitleSimilarityThreshold,
		Count:     len(matches),
		Matches:   matches,
	})
}

func parseBulkTitleLinesForCheck(s string) []string {
	lines := splitBulkTitleLinesRaw(s)
	lines = trimTrailingBlankBulkLines(lines)
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func trimTrailingBlankBulkLines(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

func loadPostTitlesForBulkCheck(db *gorm.DB) []bulkCheckTitleRow {
	if db == nil {
		return nil
	}
	var rows []bulkCheckTitleRow
	_ = db.Table("posts").Select("id", "title").Where("TRIM(title) <> ''").Find(&rows).Error
	return rows
}

type titleMatchCandidate struct {
	Title string
	Ratio float64
	ID    uint
}

func findBestDBTitleMatch(newTitle string, dbRows []bulkCheckTitleRow) *titleMatchCandidate {
	var best *titleMatchCandidate
	for _, row := range dbRows {
		old := strings.TrimSpace(row.Title)
		if old == "" {
			continue
		}
		r := text.MatchScore(newTitle, old)
		if r < bulkTitleSimilarityThreshold {
			continue
		}
		if best == nil || r > best.Ratio {
			best = &titleMatchCandidate{Title: old, Ratio: r, ID: row.ID}
		}
	}
	return best
}

func findBestEarlierListMatch(newTitle string, lines []string, index int) *titleMatchCandidate {
	var best *titleMatchCandidate
	for j := 0; j < index; j++ {
		old := lines[j]
		r := text.MatchScore(newTitle, old)
		if r < bulkTitleSimilarityThreshold {
			continue
		}
		if best == nil || r > best.Ratio {
			best = &titleMatchCandidate{Title: old, Ratio: r}
		}
	}
	return best
}

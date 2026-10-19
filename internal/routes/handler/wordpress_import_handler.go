package handler

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/gin-gonic/gin"
)

const maxWPImportBytes = 256 << 20 // 256 MB

// WordPressImportHandler handles WordPress database import.
type WordPressImportHandler struct {
	db  *gorm.DB
	cfg appconfig.Config
}

// NewWordPressImportHandler creates a new WordPressImportHandler.
func NewWordPressImportHandler(db *gorm.DB, cfg appconfig.Config) *WordPressImportHandler {
	return &WordPressImportHandler{db: db, cfg: cfg}
}

// PreviewWordPress parses the uploaded SQL and returns counts without importing.
func (h *WordPressImportHandler) PreviewWordPress(c *gin.Context) {
	sqlBytes, err := h.readUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var logs []string
	logs = append(logs, fmt.Sprintf("File diterima: %d bytes", len(sqlBytes)))

	// Show first 500 chars for debugging
	sample := string(sqlBytes)
	if len(sample) > 500 {
		sample = sample[:500]
	}
	logs = append(logs, fmt.Sprintf("Sample awal SQL:\n%s", sample))

	data := ParseWpSQL(string(sqlBytes), &logs)
	if data == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal mem-parse file SQL WordPress", "logs": logs})
		return
	}

	// Count unique categories (taxonomy=category)
	catIDs := make(map[int]bool)
	tagIDs := make(map[int]bool)
	for _, tax := range data.TermTaxonomies {
		switch strings.ToLower(tax.Taxonomy) {
		case "category":
			catIDs[tax.TermID] = true
		case "post_tag":
			tagIDs[tax.TermID] = true
		}
	}

	posts := 0
	pages := 0
	attachments := 0
	for _, p := range data.Posts {
		if strings.TrimSpace(p.PostTitle) == "" {
			continue
		}
		switch p.PostType {
		case "post":
			posts++
		case "page":
			pages++
		case "attachment":
			attachments++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"preview": WpPreviewResult{
			Authors:    len(data.Users),
			Categories: len(catIDs),
			Tags:       len(tagIDs),
			Posts:      posts,
			Pages:      pages,
		},
		"attachments": attachments,
		"logs":        logs,
	})
}

// ImportWordPress parses the uploaded SQL and imports data into the database.
func (h *WordPressImportHandler) ImportWordPress(c *gin.Context) {
	sqlBytes, err := h.readUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var logs []string
	logs = append(logs, fmt.Sprintf("File diterima: %d bytes", len(sqlBytes)))

	data := ParseWpSQL(string(sqlBytes), &logs)
	if data == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal mem-parse file SQL WordPress", "logs": logs})
		return
	}

	// Validate we have something to import
	if len(data.Posts) == 0 && len(data.Users) == 0 && len(data.Terms) == 0 {
		logs = append(logs, fmt.Sprintf("VALIDATION GAGAL: posts=%d, users=%d, terms=%d, postmeta=%d, usermeta=%d",
			len(data.Posts), len(data.Users), len(data.Terms), len(data.PostMeta), len(data.UserMeta)))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tidak ditemukan data WordPress yang valid dalam file SQL.",
			"logs":  logs,
		})
		return
	}

	// Check download_images toggle
	downloadImages := strings.ToLower(strings.TrimSpace(c.PostForm("download_images"))) == "true"

	result, err := ImportWordPressData(h.db, data, downloadImages, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Import gagal",
			"detail": err.Error(),
			"logs":   logs,
		})
		return
	}

	msg := "Import WordPress selesai"
	if downloadImages {
		msg += " (gambar telah diunduh dan disimpan ke server)"
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": msg,
		"result":  result,
		"logs":    logs,
	})
}

func (h *WordPressImportHandler) readUpload(c *gin.Context) ([]byte, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, http.ErrMissingFile
	}
	if fh.Size <= 0 {
		return nil, io.ErrUnexpectedEOF
	}
	if fh.Size > maxWPImportBytes {
		return nil, io.ErrShortBuffer
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxWPImportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	if len(raw) > maxWPImportBytes {
		return nil, io.ErrShortBuffer
	}

	name := strings.ToLower(fh.Filename)
	if strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".sql.gz") {
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		decoded, err := io.ReadAll(io.LimitReader(gr, maxWPImportBytes+1))
		if err != nil {
			return nil, err
		}
		if len(decoded) > maxWPImportBytes {
			return nil, io.ErrShortBuffer
		}
		raw = decoded
	}

	return raw, nil
}

package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
)

const (
	bulkCoverTplMaxBytes = 5 * 1024 * 1024
	bulkCoverTplDir      = "public/static/uploads/bulk-covers"
	bulkCoverTplURLBase  = "/static/uploads/bulk-covers"
)

func randomBulkCoverFilename() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func bulkCoverTplRasterMIME(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

// UploadBulkCoverBackground accepts a raster image used as the background for server-rendered bulk post covers.
func (h *AISettingHandler) UploadBulkCoverBackground(c *gin.Context) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar wajib diunggah"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > bulkCoverTplMaxBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ukuran gambar harus antara 1 byte dan 5 MB"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}
	defer src.Close()

	sniff := make([]byte, 512)
	n, err := src.Read(sniff)
	if err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memvalidasi file yang diunggah"})
		return
	}
	mimeType := http.DetectContentType(sniff[:n])
	if !bulkCoverTplRasterMIME(mimeType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya gambar jpeg, png, webp, dan gif yang diizinkan"})
		return
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses file yang diunggah"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(src, bulkCoverTplMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}

	enc, err := media.EncodePostUploadForPost(raw, mimeType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	base, err := randomBulkCoverFilename()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat nama file"})
		return
	}
	outName := "bulk-bg-" + base + enc.Ext

	if store := storage.LoadR2ObjectStore(c.Request.Context(), h.db, h.cfg); store != nil {
		key := "bulk-covers/" + outName
		url, err := store.Put(c.Request.Context(), key, bytes.NewReader(enc.Data), enc.ContentType)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"image": url})
			return
		}
		log.Printf("ai-setting bulk-cover upload: R2 failed, using local disk: %v", err)
	}

	if err := os.MkdirAll(bulkCoverTplDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan folder unggahan"})
		return
	}
	targetPath := filepath.Join(bulkCoverTplDir, outName)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file yang diunggah"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": bulkCoverTplURLBase + "/" + outName})
}

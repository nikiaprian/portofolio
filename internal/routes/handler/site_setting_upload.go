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
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
)

const (
	siteAssetMaxBytes = 5 * 1024 * 1024
	siteAssetDir      = "public/static/uploads/site"
	siteAssetURLBase  = "/static/uploads/site"
)

func randomSiteAssetName() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func isICOHeader(b []byte) bool {
	if len(b) < 6 {
		return false
	}
	// ICONDIR: reserved 0, type 1 (icon), count >= 1
	return b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0 && b[4] >= 1
}

func siteAssetRasterMIME(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

// UploadImage stores a logo, favicon, or default meta image and returns a public URL.
// Form: multipart field "image"; PostForm "kind" = logo | favicon | default_image
func (h *SiteSettingHandler) UploadImage(c *gin.Context) {
	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
	switch kind {
	case "logo", "favicon", "default_image":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": `Jenis upload harus "logo", "favicon", atau "default_image"`})
		return
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar wajib diunggah"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > siteAssetMaxBytes {
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
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses file yang diunggah"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(src, siteAssetMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar kosong"})
		return
	}

	baseName, err := randomSiteAssetName()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat nama file"})
		return
	}

	var (
		outData     []byte
		outMIME     string
		outFilename string
	)

	switch kind {
	case "favicon":
		if siteAssetRasterMIME(mimeType) {
			icoData, encErr := media.EncodeFaviconICO(raw, mimeType)
			if encErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": encErr.Error()})
				return
			}
			outData = icoData
			outMIME = "image/x-icon"
			outFilename = "favicon-" + baseName + ".ico"
			break
		}
		if !isICOHeader(raw) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Favicon: gunakan JPEG, PNG, WebP, GIF, atau file ICO yang valid"})
			return
		}
		outData = raw
		outMIME = "image/x-icon"
		outFilename = "favicon-" + baseName + ".ico"

	case "logo":
		if !siteAssetRasterMIME(mimeType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya gambar jpeg, png, webp, dan gif yang diizinkan"})
			return
		}
		enc, err := media.EncodePostUpload(raw, mimeType)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		outData = enc.Data
		outMIME = enc.ContentType
		outFilename = "logo-" + baseName + enc.Ext

	case "default_image":
		if !siteAssetRasterMIME(mimeType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya gambar jpeg, png, webp, dan gif yang diizinkan"})
			return
		}
		enc, err := media.EncodePostUploadForPost(raw, mimeType)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		outData = enc.Data
		outMIME = enc.ContentType
		outFilename = "og-default-" + baseName + enc.Ext
	}

	if store := storage.LoadR2ObjectStore(c.Request.Context(), h.db, h.cfg); store != nil {
		key := "site/" + outFilename
		url, err := store.Put(c.Request.Context(), key, bytes.NewReader(outData), outMIME)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"image": url})
			return
		}
		log.Printf("site-setting upload-image: R2 failed, using local disk: %v", err)
	}

	if err := os.MkdirAll(siteAssetDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan folder unggahan"})
		return
	}
	targetPath := filepath.Join(siteAssetDir, outFilename)
	if err := os.WriteFile(targetPath, outData, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file yang diunggah"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": siteAssetURLBase + "/" + outFilename})
}

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/gsc"
)

func gscCredentials(db *gorm.DB) (entity.SiteConnection, error) {
	var sc entity.SiteConnection
	if db == nil {
		return sc, errors.New("database tidak tersedia")
	}
	if err := db.First(&sc).Error; err != nil {
		return sc, errors.New("gagal memuat konfigurasi Site Connection")
	}
	if strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return sc, errors.New("kredensial Google Service Account JSON belum dikonfigurasi")
	}
	return sc, nil
}

func gscAutoIndexEnabled(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	var sc entity.SiteConnection
	if err := db.First(&sc).Error; err != nil || !sc.GoogleIndexingEnabled {
		return false
	}
	return strings.TrimSpace(sc.GoogleServiceAccountJSON) != ""
}

func recordGSCIndexing(db *gorm.DB, model any, id uint, targetURL string) (string, error) {
	if db == nil || id == 0 {
		return "", errors.New("data tidak valid")
	}
	sc, err := gscCredentials(db)
	if err != nil {
		return "", err
	}
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return "", errors.New("URL target indexing tidak boleh kosong")
	}

	now := time.Now().UTC()
	save := func(status, msg string) {
		if len(msg) > 490 {
			msg = msg[:490]
		}
		_ = db.Model(model).Where("id = ?", id).Updates(map[string]interface{}{
			"indexing_status":  status,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
	}

	inspect, inspectErr := gsc.InspectURL(sc.GoogleServiceAccountJSON, "", targetURL)
	if inspectErr == nil && inspect != nil && inspect.IsIndexed {
		msg := "Terindeks di Google"
		if inspect.CoverageState != "" {
			msg = fmt.Sprintf("Terindeks di Google (%s)", inspect.CoverageState)
		}
		save(entity.IndexingStatusIndexed, msg)
		return targetURL, nil
	}

	resp, err := gsc.RequestURLIndexing(sc.GoogleServiceAccountJSON, targetURL, "URL_UPDATED")
	if err != nil {
		msg := err.Error()
		if inspect != nil && inspect.CoverageState != "" {
			msg = fmt.Sprintf("%s (%s)", inspect.CoverageState, err.Error())
		}
		save(entity.IndexingStatusFailed, msg)
		return targetURL, err
	}

	msg := "Terkirim ke Google Indexing API"
	if resp != nil && resp.URLNotificationMetadata.LatestUpdate.NotifyTime != "" {
		msg = fmt.Sprintf("Terkirim: %s", resp.URLNotificationMetadata.LatestUpdate.NotifyTime)
	}
	save(entity.IndexingStatusSubmitted, msg)
	return targetURL, nil
}

func (h *PostHandler) triggerGSCIndexing(p *entity.Post, reqCtx *gin.Context) (string, error) {
	if p == nil || p.ID == 0 {
		return "", errors.New("post tidak valid")
	}
	postURL := gsc.BuildPublicPostURL(h.db, p, gsc.PublicSiteOrigin(reqCtx))
	return recordGSCIndexing(h.db, &entity.Post{}, p.ID, postURL)
}

func (h *PostHandler) maybeAutoIndexGSC(p *entity.Post) {
	if h.db == nil || p == nil || p.ID == 0 {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(p.Status), "published") {
		return
	}
	if !gscAutoIndexEnabled(h.db) {
		return
	}
	go func(postID uint) {
		var current entity.Post
		if err := h.db.Preload("Category").Preload("Category.Parent").First(&current, postID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(p.ID)
}

func (h *PostHandler) IndexGSC(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID post tidak valid"})
		return
	}
	var p entity.Post
	if err := h.db.Preload("Category").Preload("Category.Parent").First(&p, uint(id64)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post tidak ditemukan"})
		return
	}
	uid, role := requesterFromContext(c)
	if role != "admin" && p.AuthorID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke post ini"})
		return
	}
	postURL, err := h.triggerGSCIndexing(&p, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Pengindeksan Google Search Console berhasil dikirim",
		"post_url": postURL,
	})
}

func (h *PostHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID post tidak valid"})
		return
	}
	uid, role := requesterFromContext(c)
	var posts []entity.Post
	query := h.db.Preload("Category").Preload("Category.Parent").Where("id IN ?", body.IDs)
	if role != "admin" {
		query = query.Where("author_id = ?", uid)
	}
	if err := query.Find(&posts).Error; err != nil || len(posts) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post tidak ditemukan"})
		return
	}
	submitted := 0
	var lastErr error
	for _, p := range posts {
		item := p
		if _, err := h.triggerGSCIndexing(&item, c); err == nil {
			submitted++
		} else {
			lastErr = err
		}
	}
	if submitted == 0 && lastErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": lastErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("%d dari %d post berhasil dikirim ke Google Indexing API", submitted, len(posts)),
		"submitted": submitted,
		"total":     len(posts),
	})
}

func (h *PageHandler) triggerGSCIndexing(p *entity.Page, reqCtx *gin.Context) (string, error) {
	if p == nil || p.ID == 0 {
		return "", errors.New("halaman tidak valid")
	}
	pageURL := gsc.BuildPublicPageURL(h.db, p, gsc.PublicSiteOrigin(reqCtx))
	return recordGSCIndexing(h.db, &entity.Page{}, p.ID, pageURL)
}

func (h *PageHandler) maybeAutoIndexGSC(p *entity.Page) {
	if h.db == nil || p == nil || p.ID == 0 || !gscAutoIndexEnabled(h.db) {
		return
	}
	go func(pageID uint) {
		var current entity.Page
		if err := h.db.First(&current, pageID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(p.ID)
}

func (h *PageHandler) IndexGSC(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID halaman tidak valid"})
		return
	}
	p, err := h.svc.Get(uint(id64))
	if err != nil || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Halaman tidak ditemukan"})
		return
	}
	pageURL, err := h.triggerGSCIndexing(p, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Pengindeksan Google Search Console berhasil dikirim",
		"page_url": pageURL,
	})
}

func (h *PageHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID halaman tidak valid"})
		return
	}
	var pages []entity.Page
	if err := h.db.Where("id IN ?", body.IDs).Find(&pages).Error; err != nil || len(pages) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Halaman tidak ditemukan"})
		return
	}
	submitted := 0
	var lastErr error
	for _, p := range pages {
		item := p
		if _, err := h.triggerGSCIndexing(&item, c); err == nil {
			submitted++
		} else {
			lastErr = err
		}
	}
	if submitted == 0 && lastErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": lastErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("%d dari %d halaman berhasil dikirim ke Google Indexing API", submitted, len(pages)),
		"submitted": submitted,
		"total":     len(pages),
	})
}

func (h *TagHandler) triggerGSCIndexing(tag *entity.Tag, reqCtx *gin.Context) (string, error) {
	if tag == nil || tag.ID == 0 {
		return "", errors.New("tag tidak valid")
	}
	tagURL := gsc.BuildPublicTagURL(h.db, tag, gsc.PublicSiteOrigin(reqCtx))
	return recordGSCIndexing(h.db, &entity.Tag{}, tag.ID, tagURL)
}

func (h *TagHandler) maybeAutoIndexGSC(t *entity.Tag) {
	if h.db == nil || t == nil || t.ID == 0 || !gscAutoIndexEnabled(h.db) {
		return
	}
	go func(tagID uint) {
		var current entity.Tag
		if err := h.db.First(&current, tagID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(t.ID)
}

func (h *TagHandler) IndexGSC(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID tag tidak valid"})
		return
	}
	tag, err := h.svc.Get(uint(id64))
	if err != nil || tag == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag tidak ditemukan"})
		return
	}
	tagURL, err := h.triggerGSCIndexing(tag, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Pengindeksan Google Search Console berhasil dikirim",
		"tag_url": tagURL,
	})
}

func (h *TagHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID tag tidak valid"})
		return
	}
	var tags []entity.Tag
	if err := h.db.Where("id IN ?", body.IDs).Find(&tags).Error; err != nil || len(tags) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag tidak ditemukan"})
		return
	}
	submitted := 0
	var lastErr error
	for _, tag := range tags {
		item := tag
		if _, err := h.triggerGSCIndexing(&item, c); err == nil {
			submitted++
		} else {
			lastErr = err
		}
	}
	if submitted == 0 && lastErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": lastErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("%d dari %d tag berhasil dikirim ke Google Indexing API", submitted, len(tags)),
		"submitted": submitted,
		"total":     len(tags),
	})
}

func (h *CategoryHandler) triggerGSCIndexing(cat *entity.Category, reqCtx *gin.Context) (string, error) {
	if cat == nil || cat.ID == 0 {
		return "", errors.New("kategori tidak valid")
	}
	catURL := gsc.BuildPublicCategoryURL(h.db, cat, gsc.PublicSiteOrigin(reqCtx))
	return recordGSCIndexing(h.db, &entity.Category{}, cat.ID, catURL)
}

func (h *CategoryHandler) maybeAutoIndexGSC(cat *entity.Category) {
	if h.db == nil || cat == nil || cat.ID == 0 || !gscAutoIndexEnabled(h.db) {
		return
	}
	go func(catID uint) {
		var current entity.Category
		if err := h.db.Preload("Parent").First(&current, catID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(cat.ID)
}

func (h *CategoryHandler) IndexGSC(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kategori tidak valid"})
		return
	}
	cat, err := h.svc.Get(uint(id64))
	if err != nil || cat == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}
	catURL, err := h.triggerGSCIndexing(cat, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      "Pengindeksan Google Search Console berhasil dikirim",
		"category_url": catURL,
	})
}

func (h *CategoryHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID kategori tidak valid"})
		return
	}
	var categories []entity.Category
	if err := h.db.Preload("Parent").Where("id IN ?", body.IDs).Find(&categories).Error; err != nil || len(categories) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}
	submitted := 0
	var lastErr error
	for _, cat := range categories {
		item := cat
		if _, err := h.triggerGSCIndexing(&item, c); err == nil {
			submitted++
		} else {
			lastErr = err
		}
	}
	if submitted == 0 && lastErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": lastErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("%d dari %d kategori berhasil dikirim ke Google Indexing API", submitted, len(categories)),
		"submitted": submitted,
		"total":     len(categories),
	})
}

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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/services"
	"github.com/nikiaprian/tirtayasa/internal/storage"
)

const (
	profileImageMaxBytes = 5 * 1024 * 1024
	profileImageDir      = "public/static/uploads/profiles"
	profileImageURLBase  = "/static/uploads/profiles"
)

// UserHandler struct
type UserHandler struct {
	svc services.UserService
	db  *gorm.DB
	cfg appconfig.Config
}

// function to create a new UserHandler
func NewUserHandler(svc services.UserService, db *gorm.DB, cfg appconfig.Config) *UserHandler {
	return &UserHandler{svc: svc, db: db, cfg: cfg}
}

// List
// @Summary      List users
// @Description  Get users with pagination
// @Tags         users
// @Produce      json
// @Param        page  query     int  false  "Page number" default(1)
// @Param        size  query     int  false  "Page size (max 500)" default(10)
// @Success      200  {object}  dto.PaginatedUsersResponse
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users [get]
func (h *UserHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	q := strings.TrimSpace(c.Query("q"))

	// list users with pagination
	users, total, err := h.svc.List(page, size, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat pengguna"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       users,
		"page":        page,
		"size":        size,
		"total":       total,
		"total_pages": totalPages(total, size),
	})
}

// GetByID
// @Summary      Get user by ID
// @Description  Get a single user by ID
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  entity.User
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	// get the user by ID
	id, _ := strconv.Atoi(c.Param("id"))
	user, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pengguna tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func publicUserSlug(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	prevDash := false
	for _, r := range v {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// GetPublicBySlug returns a public-safe author profile for client pages.
func (h *UserHandler) GetPublicBySlug(c *gin.Context) {
	slug := publicUserSlug(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	users, _, err := h.svc.List(1, 200, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		parts := []string{
			publicUserSlug(u.Username),
			publicUserSlug(u.Role),
			publicUserSlug(u.Name),
		}
		if e := strings.TrimSpace(u.Email); e != "" {
			parts = append(parts, publicUserSlug(strings.SplitN(e, "@", 2)[0]))
		}
		match := false
		for _, p := range parts {
			if p != "" && p == slug {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		c.JSON(http.StatusOK, gin.H{
			"id":       u.ID,
			"name":     u.Name,
			"username": u.Username,
			"role":     u.Role,
			"profile":  u.Profile,
			"image":    u.Image,
		})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

// Create
// @Summary      Create user
// @Description  Create a new user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      entity.User  true  "User payload"
// @Success      201   {object}  entity.User
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengguna tidak valid"})
		return
	}

	input := entity.User{
		Name:     req.Name,
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
		Profile:  req.Profile,
	}
	// Default enabled=true for new users unless explicitly disabled.
	if req.Enabled != nil {
		input.Enabled = *req.Enabled
	} else {
		input.Enabled = true
	}
	if s := strings.TrimSpace(req.Image); s != "" {
		input.Image = &s
	}

	if err := h.svc.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat pengguna"})
		return
	}
	c.JSON(http.StatusCreated, input)
}

// Update
// @Summary      Update user
// @Description  Update existing user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int         true  "User ID"
// @Param        body  body      entity.User true  "User payload"
// @Success      200   {object}  entity.User
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengguna tidak valid"})
		return
	}

	// Keep enabled status unless explicitly updated.
	current, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pengguna tidak ditemukan"})
		return
	}

	input := entity.User{
		Name:     req.Name,
		Username: current.Username,
		Email:    req.Email,
		Role:     req.Role,
		Profile:  "",
		Enabled:  current.Enabled,
	}
	oldImage := strings.TrimSpace(derefString(current.Image))
	newImage := oldImage
	if req.Username != nil {
		input.Username = *req.Username
	}
	if req.Enabled != nil {
		input.Enabled = *req.Enabled
	}
	if req.Profile != nil {
		input.Profile = *req.Profile
	}
	if req.Password != nil {
		input.Password = *req.Password
	}
	if req.Image != nil {
		v := strings.TrimSpace(*req.Image)
		input.Image = &v
		newImage = v
	}

	user, err := h.svc.Update(uint(id), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui pengguna"})
		return
	}
	if oldImage != "" && oldImage != newImage {
		removeUploadedFileIfLocal(oldImage, profileImageURLBase, profileImageDir)
	}
	c.JSON(http.StatusOK, user)
}

// Delete
// @Summary      Delete user
// @Description  Delete user by ID
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      204  "No Content"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	// delete the user
	id, _ := strconv.Atoi(c.Param("id"))
	userIDAny, ok := c.Get("userID")
	if ok {
		if requesterID, okCast := userIDAny.(uint); okCast && requesterID != 0 && requesterID == uint(id) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak bisa menghapus akun sendiri"})
			return
		}
	}
	existing, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pengguna tidak ditemukan"})
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus pengguna"})
		return
	}
	if existing != nil {
		removeUploadedFileIfLocal(derefString(existing.Image), profileImageURLBase, profileImageDir)
	}
	c.Status(http.StatusNoContent)
}

// Profile returns currently authenticated user profile.
func (h *UserHandler) Profile(c *gin.Context) {
	userIDAny, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, ok := userIDAny.(uint)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.svc.Get(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateProfile updates currently authenticated user profile fields.
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userIDAny, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, ok := userIDAny.(uint)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type reqBody struct {
		Name     string  `json:"name" binding:"required,min=3"`
		Email    string  `json:"email" binding:"required,email"`
		Profile  *string `json:"profile,omitempty"`
		Image    *string `json:"image,omitempty"`
		Password *string `json:"password,omitempty" binding:"omitempty,min=6"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := h.svc.Get(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	input := entity.User{
		Name:    req.Name,
		Email:   req.Email,
		Role:    current.Role,
		Enabled: current.Enabled,
		Profile: current.Profile,
		Image:   current.Image,
	}
	oldImage := strings.TrimSpace(derefString(current.Image))
	newImage := oldImage
	if req.Profile != nil {
		input.Profile = *req.Profile
	}
	if req.Image != nil {
		img := strings.TrimSpace(*req.Image)
		input.Image = &img
		newImage = img
	}
	if req.Password != nil {
		input.Password = *req.Password
	}

	updated, err := h.svc.Update(userID, &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if oldImage != "" && oldImage != newImage {
		removeUploadedFileIfLocal(oldImage, profileImageURLBase, profileImageDir)
	}
	c.JSON(http.StatusOK, updated)
}

// UploadProfileImage uploads and returns profile image URL.
func (h *UserHandler) UploadProfileImage(c *gin.Context) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar wajib diunggah"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > profileImageMaxBytes {
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
	if _, ok := allowedProfileImageExtension(mimeType); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya gambar jpeg, png, webp, dan gif yang diizinkan"})
		return
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses file yang diunggah"})
		return
	}

	raw, err := io.ReadAll(io.LimitReader(src, profileImageMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar kosong"})
		return
	}

	enc, err := media.EncodePostUpload(raw, mimeType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses gambar"})
		return
	}

	filename, err := newProfileImageFilename(enc.Ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat nama file"})
		return
	}

	if store := storage.LoadR2ObjectStore(c.Request.Context(), h.db, h.cfg); store != nil {
		key := "profiles/" + filename
		url, err := store.Put(c.Request.Context(), key, bytes.NewReader(enc.Data), enc.ContentType)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"image": url})
			return
		}
		log.Printf("profile upload-image: R2 failed, using local disk: %v", err)
	}

	if err := os.MkdirAll(profileImageDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan folder unggahan"})
		return
	}
	targetPath := filepath.Join(profileImageDir, filename)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file yang diunggah"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image": profileImageURLBase + "/" + filename})
}

func allowedProfileImageExtension(mimeType string) (string, bool) {
	switch mimeType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func newProfileImageFilename(ext string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]) + ext, nil
}

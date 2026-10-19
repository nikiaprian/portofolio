package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/ai"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
)

type layoutGenerateItemResult struct {
	Fragment string `json:"fragment"`
	Label    string `json:"label,omitempty"`
	Saved    bool   `json:"saved"`
	Error    string `json:"error,omitempty"`
}

type layoutGenerateTask struct {
	Fragment string
	Label    string
	ImageURL string
}

type layoutGenerateJob struct {
	ID           string                     `json:"job_id"`
	UserID       uint                       `json:"-"`
	LayoutID     uint                       `json:"layout_id"`
	LayoutName   string                     `json:"layout_name"`
	Mode         string                     `json:"mode"`
	Label        string                     `json:"label"`
	Status       string                     `json:"status"`
	Total        int                        `json:"total"`
	Done         int                        `json:"done"`
	Succeeded    int                        `json:"succeeded"`
	Failed       int                        `json:"failed"`
	CurrentLabel string                     `json:"current_label,omitempty"`
	Results      []layoutGenerateItemResult `json:"results,omitempty"`
	Error        string                     `json:"error,omitempty"`
	CreatedAt    time.Time                  `json:"created_at"`
	StartedAt    *time.Time                 `json:"started_at,omitempty"`
	FinishedAt   *time.Time                 `json:"finished_at,omitempty"`
	BaseURL      string                     `json:"-"`
	Request      dto.EnqueueLayoutGenerateRequest
	Ctx          context.Context    `json:"-"`
	Cancel       context.CancelFunc `json:"-"`
}

func (j *layoutGenerateJob) toResponse() map[string]any {
	return map[string]any{
		"job_id":        j.ID,
		"layout_id":     j.LayoutID,
		"layout_name":   j.LayoutName,
		"mode":          j.Mode,
		"label":         j.Label,
		"status":        j.Status,
		"total":         j.Total,
		"done":          j.Done,
		"succeeded":     j.Succeeded,
		"failed":        j.Failed,
		"current_label": j.CurrentLabel,
		"results":       j.Results,
		"error":         j.Error,
		"created_at":    j.CreatedAt,
		"started_at":    j.StartedAt,
		"finished_at":   j.FinishedAt,
	}
}

func (h *SiteLayoutHandler) initLayoutGenerateWorker() {
	h.genOnce.Do(func() {
		h.genJobsMu.Lock()
		if h.genJobs == nil {
			h.genJobs = make(map[string]*layoutGenerateJob)
		}
		h.genJobsMu.Unlock()
		h.genQueue = make(chan *layoutGenerateJob, 32)
		go func() {
			for job := range h.genQueue {
				h.runLayoutGenerateJob(job)
			}
		}()
	})
}

func (h *SiteLayoutHandler) newLayoutGenerateJobID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("lg_%x", b), nil
}

func siteLayoutFragmentLabel(fragment string) string {
	switch normalizeSiteLayoutFragment(fragment) {
	case "header":
		return "Header"
	case "footer":
		return "Footer"
	case "home_body":
		return "Home body"
	case "article_body":
		return "Article body"
	case "sidebar":
		return "Sidebar"
	case "rss":
		return "RSS"
	case "author_profile":
		return "Author profile"
	case "tag":
		return "Tag"
	case "kategori":
		return "Kategori"
	case "search_page":
		return "Search page"
	default:
		return fragment
	}
}

// EnqueueGenerateJob queues background AI layout generation and returns immediately.
func (h *SiteLayoutHandler) EnqueueGenerateJob(c *gin.Context) {
	if h.ai == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}

	var req dto.EnqueueLayoutGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	if req.LayoutID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Simpan layout terlebih dahulu sebelum generate background"})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "single" && mode != "all" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mode generate tidak valid"})
		return
	}
	if mode == "single" && normalizeSiteLayoutFragment(req.Fragment) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fragment tidak valid"})
		return
	}
	if strings.TrimSpace(req.LayoutName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama layout wajib diisi"})
		return
	}
	if len(trimNonEmptyStrings(req.ImageRefs)) == 0 && strings.TrimSpace(req.ImageURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unggah minimal satu gambar referensi"})
		return
	}
	if _, err := h.svc.Get(req.LayoutID); err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Layout tidak ditemukan"})
		return
	}

	requesterID, _ := requesterFromContext(c)
	if requesterID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	job, err := h.enqueueLayoutGenerateJob(requesterID, publicCanonicalSiteBaseURL(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengantrekan job generate"})
		return
	}
	log.Printf("[layout-ai] job queued job_id=%s layout_id=%d mode=%s user_id=%d", job.ID, job.LayoutID, job.Mode, job.UserID)
	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  job.ID,
		"status":  job.Status,
		"total":   job.Total,
		"label":   job.Label,
		"queued":  true,
		"message": "layout generate job queued",
	})
}

// ListGenerateJobs returns active and recent layout AI jobs for the current admin.
func (h *SiteLayoutHandler) ListGenerateJobs(c *gin.Context) {
	h.initLayoutGenerateWorker()
	requesterID, _ := requesterFromContext(c)
	layoutID := parseUintQuery(c.Query("layout_id"))
	cutoff := time.Now().UTC().Add(-2 * time.Hour)

	h.genJobsMu.RLock()
	items := make([]map[string]any, 0, 8)
	for _, job := range h.genJobs {
		if job == nil || job.UserID != requesterID {
			continue
		}
		if layoutID > 0 && job.LayoutID != layoutID {
			continue
		}
		st := strings.ToLower(strings.TrimSpace(job.Status))
		active := st == "queued" || st == "running"
		recent := job.FinishedAt != nil && job.FinishedAt.After(cutoff)
		if !active && !recent {
			continue
		}
		items = append(items, job.toResponse())
	}
	h.genJobsMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetGenerateJobStatus returns one background layout AI job.
func (h *SiteLayoutHandler) GetGenerateJobStatus(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID job tidak ditemukan"})
		return
	}
	requesterID, _ := requesterFromContext(c)
	job, ok := h.getLayoutGenerateJob(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job tidak ditemukan"})
		return
	}
	if job.UserID != requesterID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke job ini"})
		return
	}
	c.JSON(http.StatusOK, job.toResponse())
}

// CancelGenerateJob cancels a queued or running layout AI job.
func (h *SiteLayoutHandler) CancelGenerateJob(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID job tidak ditemukan"})
		return
	}
	requesterID, _ := requesterFromContext(c)
	job, ok, err := h.cancelLayoutGenerateJob(jobID, requesterID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, job.toResponse())
}

func (h *SiteLayoutHandler) enqueueLayoutGenerateJob(userID uint, baseURL string, req dto.EnqueueLayoutGenerateRequest) (*layoutGenerateJob, error) {
	if userID == 0 {
		return nil, errors.New("invalid requester")
	}
	h.initLayoutGenerateWorker()
	jobID, err := h.newLayoutGenerateJobID()
	if err != nil {
		return nil, err
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	label := "Semua fragment"
	total := 1
	if mode == "single" {
		frag := normalizeSiteLayoutFragment(req.Fragment)
		label = siteLayoutFragmentLabel(frag)
		total = 1
	} else {
		total = len(siteLayoutFragmentKeys)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &layoutGenerateJob{
		ID:         jobID,
		UserID:     userID,
		LayoutID:   req.LayoutID,
		LayoutName: strings.TrimSpace(req.LayoutName),
		Mode:       mode,
		Label:      label,
		Status:     "queued",
		Total:      total,
		Results:    make([]layoutGenerateItemResult, 0, total),
		CreatedAt:  time.Now().UTC(),
		BaseURL:    strings.TrimSpace(baseURL),
		Request:    req,
		Ctx:        ctx,
		Cancel:     cancel,
	}

	h.genJobsMu.Lock()
	h.genJobs[job.ID] = job
	if len(h.genJobs) > 200 {
		cutoff := time.Now().UTC().Add(-24 * time.Hour)
		for id, row := range h.genJobs {
			if row.FinishedAt != nil && row.FinishedAt.Before(cutoff) {
				delete(h.genJobs, id)
			}
		}
	}
	h.genJobsMu.Unlock()
	h.genQueue <- job
	return job, nil
}

func (h *SiteLayoutHandler) getLayoutGenerateJob(jobID string) (*layoutGenerateJob, bool) {
	h.genJobsMu.RLock()
	defer h.genJobsMu.RUnlock()
	job, ok := h.genJobs[jobID]
	return job, ok
}

func (h *SiteLayoutHandler) updateLayoutGenerateJob(jobID string, fn func(*layoutGenerateJob)) {
	h.genJobsMu.Lock()
	defer h.genJobsMu.Unlock()
	if job, ok := h.genJobs[jobID]; ok {
		fn(job)
	}
}

func (h *SiteLayoutHandler) cancelLayoutGenerateJob(jobID string, requesterID uint) (*layoutGenerateJob, bool, error) {
	h.initLayoutGenerateWorker()
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, false, errors.New("missing job id")
	}
	now := time.Now().UTC()
	h.genJobsMu.Lock()
	defer h.genJobsMu.Unlock()
	job, ok := h.genJobs[jobID]
	if !ok {
		return nil, false, nil
	}
	if job.UserID != requesterID {
		return job, true, errors.New("forbidden")
	}
	st := strings.ToLower(strings.TrimSpace(job.Status))
	if st == "done" || st == "failed" || st == "canceled" {
		return job, true, nil
	}
	job.Status = "canceled"
	job.Error = "generate layout dibatalkan"
	if job.FinishedAt == nil {
		job.FinishedAt = &now
	}
	if job.Cancel != nil {
		job.Cancel()
	}
	return job, true, nil
}

func (h *SiteLayoutHandler) runLayoutGenerateJob(job *layoutGenerateJob) {
	if job == nil {
		return
	}
	started := time.Now().UTC()
	shouldRun := true
	h.updateLayoutGenerateJob(job.ID, func(j *layoutGenerateJob) {
		if strings.EqualFold(strings.TrimSpace(j.Status), "canceled") || (j.Ctx != nil && j.Ctx.Err() != nil) {
			shouldRun = false
			if j.FinishedAt == nil {
				finished := time.Now().UTC()
				j.FinishedAt = &finished
			}
			return
		}
		j.Status = "running"
		j.StartedAt = &started
	})
	if !shouldRun {
		return
	}

	ctx := job.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	runErr := h.executeLayoutGenerateJob(ctx, job)
	finished := time.Now().UTC()
	h.updateLayoutGenerateJob(job.ID, func(j *layoutGenerateJob) {
		if strings.EqualFold(strings.TrimSpace(j.Status), "canceled") {
			if j.FinishedAt == nil {
				j.FinishedAt = &finished
			}
			return
		}
		if errors.Is(runErr, context.Canceled) {
			j.Status = "canceled"
			j.Error = "generate layout dibatalkan"
		} else if runErr != nil {
			j.Status = "failed"
			j.Error = runErr.Error()
		} else if j.Failed > 0 && j.Succeeded == 0 {
			j.Status = "failed"
			if j.Error == "" {
				j.Error = "semua fragment gagal digenerate"
			}
		} else {
			j.Status = "done"
		}
		j.FinishedAt = &finished
		j.CurrentLabel = ""
	})
}

func (h *SiteLayoutHandler) executeLayoutGenerateJob(ctx context.Context, job *layoutGenerateJob) error {
	if h.ai == nil {
		return errors.New("AI belum dikonfigurasi")
	}

	tasks, err := h.buildLayoutGenerateTasks(ctx, job)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return errors.New("tidak ada fragment yang bisa digenerate")
	}

	h.updateLayoutGenerateJob(job.ID, func(j *layoutGenerateJob) {
		j.Total = len(tasks)
	})

	req := job.Request
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "auto"
	}
	currentPrompt := strings.TrimSpace(req.CurrentPrompt)
	if currentPrompt == "" {
		currentPrompt = siteLayoutDefaultCurrentPrompt
	}
	templateName := siteLayoutTemplatePrefix(job.LayoutName)

	for i, task := range tasks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		h.updateLayoutGenerateJob(job.ID, func(j *layoutGenerateJob) {
			if strings.EqualFold(strings.TrimSpace(j.Status), "canceled") {
				return
			}
			j.CurrentLabel = task.Label
		})

		item := layoutGenerateItemResult{
			Fragment: task.Fragment,
			Label:    task.Label,
		}

		content, genErr := h.generateLayoutFragmentContent(ctx, layoutFragmentGenerateInput{
			Fragment:      task.Fragment,
			ImageURL:      task.ImageURL,
			BaseURL:       job.BaseURL,
			Provider:      provider,
			CurrentPrompt: currentPrompt,
			TemplateName:  templateName,
		})
		if genErr != nil {
			item.Error = genErr.Error()
			h.appendLayoutGenerateItem(job.ID, item, false)
			continue
		}

		if _, saveErr := h.svc.ApplyFragment(job.LayoutID, task.Fragment, content); saveErr != nil {
			item.Error = saveErr.Error()
			h.appendLayoutGenerateItem(job.ID, item, false)
			continue
		}

		item.Saved = true
		h.appendLayoutGenerateItem(job.ID, item, true)

		_ = i
	}
	return nil
}

func (h *SiteLayoutHandler) appendLayoutGenerateItem(jobID string, item layoutGenerateItemResult, ok bool) {
	h.updateLayoutGenerateJob(jobID, func(j *layoutGenerateJob) {
		if strings.EqualFold(strings.TrimSpace(j.Status), "canceled") {
			return
		}
		j.Done++
		if ok {
			j.Succeeded++
		} else {
			j.Failed++
		}
		j.Results = append(j.Results, item)
	})
}

func (h *SiteLayoutHandler) buildLayoutGenerateTasks(ctx context.Context, job *layoutGenerateJob) ([]layoutGenerateTask, error) {
	req := job.Request
	if strings.ToLower(strings.TrimSpace(job.Mode)) == "single" {
		frag := normalizeSiteLayoutFragment(req.Fragment)
		imageURL, err := h.resolveSiteLayoutImageURL(strings.TrimSpace(req.ImageURL), req.ImageRefs)
		if err != nil {
			return nil, err
		}
		return []layoutGenerateTask{{
			Fragment: frag,
			Label:    siteLayoutFragmentLabel(frag),
			ImageURL: imageURL,
		}}, nil
	}

	mapping, err := h.classifyLayoutImageRefs(ctx, job.BaseURL, req.ImageRefs, req.Provider)
	if err != nil {
		return nil, err
	}

	tasks := make([]layoutGenerateTask, 0, len(siteLayoutFragmentKeys))
	for _, frag := range siteLayoutFragmentKeys {
		imageURL := ""
		for u, key := range mapping {
			if key == frag {
				imageURL = u
				break
			}
		}
		if imageURL == "" {
			continue
		}
		tasks = append(tasks, layoutGenerateTask{
			Fragment: frag,
			Label:    siteLayoutFragmentLabel(frag),
			ImageURL: imageURL,
		})
	}
	return tasks, nil
}

type layoutFragmentGenerateInput struct {
	Fragment      string
	ImageURL      string
	BaseURL       string
	Provider      string
	CurrentPrompt string
	TemplateName  string
}

func (h *SiteLayoutHandler) generateLayoutFragmentContent(ctx context.Context, in layoutFragmentGenerateInput) (string, error) {
	imageRaw, mime, err := loadSiteLayoutRefImage(in.ImageURL, in.BaseURL)
	if err != nil {
		return "", err
	}
	refContent, err := readSiteLayoutReferenceTemplate(in.Fragment)
	if err != nil {
		return "", err
	}
	prompt := buildSiteLayoutFragmentPrompt(in.Fragment, in.TemplateName, in.CurrentPrompt, refContent)

	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if provider == "" {
		provider = "auto"
	}

	var text string
	switch provider {
	case "auto":
		text, _, err = h.ai.GenerateBlogContentWithImageAndProvider(ctx, prompt, mime, imageRaw)
	default:
		text, err = h.ai.GenerateWithImage(ctx, ai.Provider(provider), prompt, mime, imageRaw)
	}
	if err != nil {
		return "", err
	}
	content := extractSiteLayoutAIOutput(text)
	if content == "" {
		return "", errors.New("AI mengembalikan fragment kosong")
	}
	return content, nil
}

func (h *SiteLayoutHandler) classifyLayoutImageRefs(ctx context.Context, baseURL string, imageRefs []string, provider string) (map[string]string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "auto"
	}
	mapping := make(map[string]string)
	prompt := `Look at this website screenshot. Which ONE layout fragment type does it best represent?

Options (reply with ONLY the key, nothing else):
header
footer
home_body
article_body
sidebar
rss
author_profile
tag
kategori
search_page
unknown`

	for _, rawURL := range imageRefs {
		imageURL := strings.TrimSpace(rawURL)
		if imageURL == "" {
			continue
		}
		imageRaw, mime, err := loadSiteLayoutRefImage(imageURL, baseURL)
		if err != nil {
			return nil, err
		}
		var text string
		if provider == "auto" {
			text, _, err = h.ai.GenerateBlogContentWithImageAndProvider(ctx, prompt, mime, imageRaw)
		} else {
			text, err = h.ai.GenerateWithImage(ctx, ai.Provider(provider), prompt, mime, imageRaw)
		}
		if err != nil {
			return nil, err
		}
		key := normalizeSiteLayoutFragment(strings.TrimSpace(strings.ToLower(text)))
		if key != "" && key != "unknown" {
			mapping[imageURL] = key
		}
	}
	return mapping, nil
}

func trimNonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, raw := range items {
		if s := strings.TrimSpace(raw); s != "" {
			out = append(out, s)
		}
	}
	return out
}

package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/imagecaption"
	"github.com/nikiaprian/tirtayasa/internal/helper/posthtml"
	"github.com/nikiaprian/tirtayasa/internal/helper/postmeta"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/scraper"
	"github.com/nikiaprian/tirtayasa/internal/storage"
)

type bulkGenerateItemResult struct {
	Title string `json:"title"`
	ID    uint   `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type bulkGenerateJob struct {
	ID         string                   `json:"job_id"`
	UserID     uint                     `json:"-"`
	Status     string                   `json:"status"`
	Total      int                      `json:"total"`
	Done       int                      `json:"done"`
	Succeeded  int                      `json:"succeeded"`
	Failed     int                      `json:"failed"`
	Results    []bulkGenerateItemResult `json:"results,omitempty"`
	Error      string                   `json:"error,omitempty"`
	CreatedAt  time.Time                `json:"created_at"`
	StartedAt  *time.Time               `json:"started_at,omitempty"`
	FinishedAt *time.Time               `json:"finished_at,omitempty"`
	Request    dto.BulkGeneratePostsRequest
	Ctx        context.Context    `json:"-"`
	Cancel     context.CancelFunc `json:"-"`
}

func (j *bulkGenerateJob) toResponse() map[string]any {
	return map[string]any{
		"job_id":      j.ID,
		"status":      j.Status,
		"total":       j.Total,
		"done":        j.Done,
		"succeeded":   j.Succeeded,
		"failed":      j.Failed,
		"results":     j.Results,
		"error":       j.Error,
		"created_at":  j.CreatedAt,
		"started_at":  j.StartedAt,
		"finished_at": j.FinishedAt,
	}
}

func (h *PostHandler) initBulkGenerateWorker() {
	h.bulkOnce.Do(func() {
		h.bulkJobsMu.Lock()
		if h.bulkJobs == nil {
			h.bulkJobs = make(map[string]*bulkGenerateJob)
		}
		h.bulkJobsMu.Unlock()
		h.bulkQueue = make(chan *bulkGenerateJob, 32)
		go func() {
			for job := range h.bulkQueue {
				h.runBulkGenerateJob(job)
			}
		}()
	})
}

func (h *PostHandler) newBulkGenerateJobID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("bg_%x", b), nil
}

func (h *PostHandler) enqueueBulkGenerateJob(userID uint, req dto.BulkGeneratePostsRequest) (*bulkGenerateJob, error) {
	if userID == 0 {
		return nil, errors.New("invalid requester")
	}
	h.initBulkGenerateWorker()
	jobID, err := h.newBulkGenerateJobID()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &bulkGenerateJob{
		ID:        jobID,
		UserID:    userID,
		Status:    "queued",
		Total:     len(req.Titles),
		Results:   make([]bulkGenerateItemResult, 0, len(req.Titles)),
		CreatedAt: time.Now().UTC(),
		Request:   req,
		Ctx:       ctx,
		Cancel:    cancel,
	}
	h.bulkJobsMu.Lock()
	h.bulkJobs[job.ID] = job
	// keep in-memory job list bounded
	if len(h.bulkJobs) > 200 {
		cutoff := time.Now().UTC().Add(-24 * time.Hour)
		for id, row := range h.bulkJobs {
			if row.FinishedAt != nil && row.FinishedAt.Before(cutoff) {
				delete(h.bulkJobs, id)
			}
		}
	}
	h.bulkJobsMu.Unlock()
	h.bulkQueue <- job
	return job, nil
}

func (h *PostHandler) getBulkGenerateJob(jobID string) (*bulkGenerateJob, bool) {
	h.bulkJobsMu.RLock()
	defer h.bulkJobsMu.RUnlock()
	job, ok := h.bulkJobs[jobID]
	return job, ok
}

func (h *PostHandler) getActiveBulkGenerateJob(requesterID uint, requesterRole string) (*bulkGenerateJob, bool) {
	h.initBulkGenerateWorker()
	h.bulkJobsMu.RLock()
	defer h.bulkJobsMu.RUnlock()
	var active *bulkGenerateJob
	for _, job := range h.bulkJobs {
		if job == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(requesterRole), "writer") && job.UserID != requesterID {
			continue
		}
		st := strings.ToLower(strings.TrimSpace(job.Status))
		if st != "queued" && st != "running" {
			continue
		}
		if active == nil || job.CreatedAt.After(active.CreatedAt) {
			active = job
		}
	}
	return active, active != nil
}

func (h *PostHandler) updateBulkGenerateJob(jobID string, fn func(*bulkGenerateJob)) {
	h.bulkJobsMu.Lock()
	defer h.bulkJobsMu.Unlock()
	if job, ok := h.bulkJobs[jobID]; ok {
		fn(job)
	}
}

func (h *PostHandler) cancelBulkGenerateJob(jobID string, requesterID uint, requesterRole string) (*bulkGenerateJob, bool, error) {
	h.initBulkGenerateWorker()
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, false, errors.New("missing job id")
	}
	now := time.Now().UTC()
	h.bulkJobsMu.Lock()
	defer h.bulkJobsMu.Unlock()
	job, ok := h.bulkJobs[jobID]
	if !ok {
		return nil, false, nil
	}
	if strings.EqualFold(strings.TrimSpace(requesterRole), "writer") && job.UserID != requesterID {
		return job, true, errors.New("forbidden")
	}
	st := strings.ToLower(strings.TrimSpace(job.Status))
	if st == "done" || st == "failed" || st == "canceled" {
		return job, true, nil
	}
	job.Status = "canceled"
	job.Error = "bulk generation canceled by user"
	if job.FinishedAt == nil {
		job.FinishedAt = &now
	}
	if job.Cancel != nil {
		job.Cancel()
	}
	return job, true, nil
}

func (h *PostHandler) persistBulkPostLogByUser(userID uint, level, event, message string, categoryID *uint, postID *uint, title string, providerUsed string) {
	if h.db == nil || userID == 0 {
		return
	}
	row := entity.BulkPostLog{
		UserID:       userID,
		Level:        truncateBulkPostLogRunes(strings.TrimSpace(level), 16),
		Event:        truncateBulkPostLogRunes(strings.TrimSpace(event), 32),
		Message:      truncateBulkPostLogRunes(strings.TrimSpace(message), bulkPostLogMsgMaxRunes),
		CategoryID:   categoryID,
		PostID:       postID,
		Title:        truncateBulkPostLogRunes(strings.TrimSpace(title), 220),
		ProviderUsed: normalizeBulkPostProviderCol(providerUsed),
	}
	if row.Level == "" {
		row.Level = "info"
	}
	_ = h.db.Select("UserID", "Level", "Event", "Message", "CategoryID", "PostID", "Title", "ProviderUsed").Create(&row).Error
}

func (h *PostHandler) runBulkGenerateJob(job *bulkGenerateJob) {
	if job == nil {
		return
	}
	started := time.Now().UTC()
	shouldRun := true
	h.updateBulkGenerateJob(job.ID, func(j *bulkGenerateJob) {
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
	results, runErr := h.generateBulkPosts(ctx, job.UserID, job.Request, func(done, ok, fail int, item bulkGenerateItemResult) {
		h.updateBulkGenerateJob(job.ID, func(j *bulkGenerateJob) {
			if strings.EqualFold(strings.TrimSpace(j.Status), "canceled") {
				return
			}
			j.Done = done
			j.Succeeded = ok
			j.Failed = fail
			if item.Title != "" {
				j.Results = append(j.Results, item)
			}
		})
	})
	finished := time.Now().UTC()
	h.updateBulkGenerateJob(job.ID, func(j *bulkGenerateJob) {
		if strings.EqualFold(strings.TrimSpace(j.Status), "canceled") {
			j.Results = results
			j.Done = len(results)
			if j.FinishedAt == nil {
				j.FinishedAt = &finished
			}
			return
		}
		j.Results = results
		j.Done = len(results)
		j.Status = "done"
		if errors.Is(runErr, context.Canceled) {
			j.Status = "canceled"
			j.Error = "bulk generation canceled by user"
		} else if runErr != nil {
			j.Status = "failed"
			j.Error = runErr.Error()
		}
		j.FinishedAt = &finished
	})
}

func (h *PostHandler) generateBulkPosts(ctx context.Context, userID uint, req dto.BulkGeneratePostsRequest, onItem func(done, ok, fail int, item bulkGenerateItemResult)) ([]bulkGenerateItemResult, error) {
	if h.ai == nil || h.aiSet == nil {
		return nil, errors.New("AI is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// Load once up-front so we can fail fast if DB is unavailable,
	// but re-load inside the loop to always follow the latest saved prompts.
	row, err := h.aiSet.GetOrCreate()
	if err != nil {
		return nil, err
	}
	contentHint := strings.TrimSpace(req.ContentPlaceholder)
	if contentHint == "" {
		contentHint = strings.TrimSpace(req.LegacyIsiPlaceholder)
	}
	if contentHint == "" {
		contentHint = "Kembangkan isi yang informatif dan SEO-friendly berdasarkan judul dan kata kunci utama."
	}
	metaDescHint := strings.TrimSpace(req.MetaDescriptionPlaceholder)
	if metaDescHint == "" {
		metaDescHint = strings.TrimSpace(req.LegacyDeskripsiPlaceholder)
	}
	if metaDescHint == "" {
		metaDescHint = strings.TrimSpace(row.GeneratePromptMetaDescription)
	}
	if metaDescHint == "" {
		metaDescHint = "Ringkas meta description (sekitar 150–160 karakter) yang relevan dengan judul."
	}
	p := strings.ToLower(strings.TrimSpace(req.Provider))
	if p == "" {
		p = "auto"
	}
	coverMode := strings.ToLower(strings.TrimSpace(req.CoverImageMode))
	if coverMode == "" {
		coverMode = "template"
	}
	bulkTags := make([]entity.Tag, 0, len(req.Tags))
	for _, tid := range req.Tags {
		bulkTags = append(bulkTags, entity.Tag{ID: tid})
	}
	perCategory := len(req.CategoryIDs) == len(req.Titles)
	cid := req.CategoryID
	catPtr := &cid
	if cid == 0 {
		catPtr = nil
	}
	out := make([]bulkGenerateItemResult, 0, len(req.Titles))
	genDelayMin := req.GenerateDelayMinutes
	if genDelayMin <= 0 {
		genDelayMin = 1
	}
	h.persistBulkPostLogByUser(userID, "info", "generate_start", fmt.Sprintf("status=%s judul=%d kategori_id=%d per_kategori=%t penulis_id=%d interval_menit=%d jeda_generate_menit=%d", req.Status, len(req.Titles), req.CategoryID, perCategory, req.AuthorID, req.IntervalMinutes, genDelayMin), catPtr, nil, "", "")
	nOk, nFail := 0, 0
	for i, title := range req.Titles {
		itemCategoryID := req.CategoryID
		if perCategory {
			itemCategoryID = req.CategoryIDs[i]
		}
		itemCatPtr := &itemCategoryID
		if err := ctx.Err(); err != nil {
			h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, "", "")
			return out, err
		}
		// Always follow the latest saved bulk prompt templates (fixes stale/default prompt
		// when ai_setting has multiple rows or is updated between bulk runs).
		if latest, e := h.aiSet.GetOrCreate(); e == nil && latest != nil {
			row = latest
		}

		tpl := pickGeneratePromptContentTemplate(row, req.PromptContentVariant)

		if i > 0 && len(req.Titles) > 1 {
			log.Printf("[bulk-post] AI cooldown before item user_id=%d index=%d/%d delay_min=%d", userID, i+1, len(req.Titles), genDelayMin)
			timer := time.NewTimer(time.Duration(genDelayMin) * time.Minute)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan saat jeda setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, "", "")
				return out, ctx.Err()
			case <-timer.C:
			}
		}
		seedKeyword := strings.TrimSpace(req.Keyword)
		if seedKeyword == "" {
			seedKeyword = title
		}
		// Match manual post generation flow:
		// - HTML content uses blogDraftSystemPrompt + content template.
		// - Meta description/keywords are generated in separate calls with their own system prompts.
		contentUserPrompt := replaceBulkPostPlaceholders(
			tpl,
			bulkPlaceholderValues(title, seedKeyword, contentHint, metaDescHint),
		)
		fullPrompt := blogDraftSystemPrompt + contentUserPrompt

		perCallCtx, perCancel := context.WithTimeout(ctx, 12*time.Minute)
		tAI := time.Now()
		log.Printf("[bulk-post] AI API generate_item start user_id=%d index=%d/%d category_id=%d title=%.120q", userID, i+1, len(req.Titles), itemCategoryID, title)
		htmlRaw, used, err := h.generateAIText(perCallCtx, p, fullPrompt)
		perCancel()
		if err != nil {
			if bulkGenerationWasCanceled(ctx, err) {
				h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan saat membuat konten AI setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, title, used)
				return out, context.Canceled
			}
			log.Printf("[bulk-post] AI API generate_item failed user_id=%d index=%d/%d provider_used=%s duration_ms=%d err=%v", userID, i+1, len(req.Titles), used, time.Since(tAI).Milliseconds(), err)
			item := bulkGenerateItemResult{Title: title, Error: err.Error()}
			h.persistBulkPostLogByUser(userID, "warn", "generate_item_err", err.Error(), itemCatPtr, nil, title, used)
			out = append(out, item)
			nFail++
			if onItem != nil {
				onItem(len(out), nOk, nFail, item)
			}
			continue
		}
		html := strings.TrimSpace(htmlRaw)
		log.Printf("[bulk-post] AI API generate_item response user_id=%d index=%d/%d provider_used=%s duration_ms=%d response_runes=%d", userID, i+1, len(req.Titles), used, time.Since(tAI).Milliseconds(), utf8.RuneCountInString(html))
		if html == "" {
			item := bulkGenerateItemResult{Title: title, Error: "Respons AI kosong"}
			h.persistBulkPostLogByUser(userID, "warn", "generate_item_err", "Respons AI kosong", itemCatPtr, nil, title, used)
			out = append(out, item)
			nFail++
			if onItem != nil {
				onItem(len(out), nOk, nFail, item)
			}
			continue
		}

		html = posthtml.NormalizeParagraphRows(html)

		metaProv := strings.TrimSpace(used)
		if metaProv == "" {
			metaProv = p
		}
		tagNames := h.draftOrderedTagNames(ctx, req.Tags)
		tagKWCSV := strings.Join(tagNames, ", ")
		excerpt := postmeta.PlainExcerpt(html, 4000)
		titleLine := strings.TrimSpace(title)
		if titleLine == "" {
			titleLine = strings.TrimSpace(postmeta.PlainExcerpt(html, 120))
			if titleLine == "" {
				titleLine = "Blog article"
			}
		}

		metaDescTpl := defaultGeneratePromptMetaDescription
		metaKwTpl := defaultGeneratePromptMetaKeywords
		if v := strings.TrimSpace(row.GeneratePromptMetaDescription); v != "" {
			metaDescTpl = v
		}
		if v := strings.TrimSpace(row.GeneratePromptMetaKeywords); v != "" {
			metaKwTpl = v
		}
		vals := bulkPlaceholderValues(titleLine, tagKWCSV, excerpt, "")
		descUser := replaceBulkPostPlaceholders(metaDescTpl, vals)
		kwUser := replaceBulkPostPlaceholders(metaKwTpl, vals)

		metaDesc := ""
		metaKW := ""
		{
			mdCtx, mdCancel := context.WithTimeout(ctx, 90*time.Second)
			md, _, mdErr := h.generateAIText(mdCtx, metaProv, blogDraftMetaDescriptionSystemPrompt+descUser)
			mdCancel()
			if bulkGenerationWasCanceled(ctx, mdErr) {
				h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan saat membuat meta deskripsi AI setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, title, metaProv)
				return out, context.Canceled
			}
			if mdErr == nil {
				metaDesc = sanitizeDraftMetaDescription(md)
			}
		}
		{
			mkCtx, mkCancel := context.WithTimeout(ctx, 90*time.Second)
			mk, _, mkErr := h.generateAIText(mkCtx, metaProv, blogDraftMetaKeywordsSystemPrompt+kwUser)
			mkCancel()
			if bulkGenerationWasCanceled(ctx, mkErr) {
				h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan saat membuat meta kata kunci AI setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, title, metaProv)
				return out, context.Canceled
			}
			if mkErr == nil {
				metaKW = sanitizeDraftMetaKeywordsCSV(mk)
			}
		}

		tmp := &entity.Post{Title: titleLine, Content: html}
		postmeta.Apply(tmp, tagNames, "")
		if metaDesc == "" && tmp.MetaDesc != nil {
			metaDesc = strings.TrimSpace(*tmp.MetaDesc)
		}
		if metaKW == "" && tmp.MetaKeywords != nil {
			metaKW = strings.TrimSpace(*tmp.MetaKeywords)
		}

		slugHint := ""
		if i < len(req.Slugs) {
			slugHint = strings.TrimSpace(req.Slugs[i])
		}
		slugInput := slugHint
		if slugInput == "" {
			slugInput = slugifyPostTitleForBulk(title)
		}
		slug, err := h.ensureUniquePostSlug(slugInput)
		if err != nil {
			item := bulkGenerateItemResult{Title: title, Error: err.Error()}
			h.persistBulkPostLogByUser(userID, "warn", "generate_item_err", err.Error(), itemCatPtr, nil, title, used)
			out = append(out, item)
			nFail++
			if onItem != nil {
				onItem(len(out), nOk, nFail, item)
			}
			continue
		}

		var postStatus string
		var publishAt *time.Time
		switch req.Status {
		case "published":
			postStatus = "published"
		case "scheduled":
			postStatus = "scheduled"
			t := bulkScheduledPublishAtUTC(req.BatchBaseTime, req.TitleIndex+i, req.IntervalMinutes)
			publishAt = &t
		default:
			postStatus = "draft"
		}
		tagIDs := bulkTags
		// AI meta keywords → post tags (same site switch as scraper auto-tag).
		if repositories.ScraperAutoTagFromDB(h.db) && strings.TrimSpace(metaKW) != "" {
			merged, err := h.bulkTagsAppendKeywordsCSV(metaKW, tagIDs)
			if err != nil {
				item := bulkGenerateItemResult{Title: title, Error: err.Error()}
				h.persistBulkPostLogByUser(userID, "warn", "generate_item_err", err.Error(), itemCatPtr, nil, title, used)
				out = append(out, item)
				nFail++
				if onItem != nil {
					onItem(len(out), nOk, nFail, item)
				}
				continue
			}
			tagIDs = merged
		}

		post := entity.Post{
			Title:         title,
			Slug:          slug,
			Content:       html,
			IncludeSource: true,
			Status:        postStatus,
			PublishAt:     publishAt,
			AuthorID:      req.AuthorID,
			CategoryID:    itemCategoryID,
			Tags:          tagIDs,
		}
		if md := strings.TrimSpace(metaDesc); md != "" {
			v := md
			post.MetaDesc = &v
		}
		if mk := strings.TrimSpace(metaKW); mk != "" {
			v := mk
			post.MetaKeywords = &v
		}

		var imgURL string
		var imgProvider string
		var webRes *scraper.WebImageResult
		usedTemplateCover := false
		if coverMode == "generate_image" {
			imgCtx, imgCancel := context.WithTimeout(ctx, 2*time.Minute)
			imgURL, imgProvider, err = h.generateAICoverImage(imgCtx, title, req.ImagePrompt, p, slug, false)
			imgCancel()
		} else if coverMode == "web_search" {
			imgCtx, imgCancel := context.WithTimeout(context.Background(), 45*time.Second)
			store := storage.LoadR2ObjectStore(imgCtx, h.db, h.cfg)
			webSource := scraper.NormalizeWebSearchSource(req.CoverWebSearchSource)
			if webRes = scraper.BulkCoverImageResultFromWebSourcesAttemptSource(imgCtx, h.db, store, title, slug, 0, webSource); webRes != nil && strings.TrimSpace(webRes.StoredURL) != "" {
				imgURL = strings.TrimSpace(webRes.StoredURL)
			}
			imgCancel()
			if imgURL == "" {
				imgCtx, imgCancel = context.WithTimeout(context.Background(), 2*time.Minute)
				imgURL, err = h.generateBulkCoverImage(imgCtx, title, slug)
				imgCancel()
				usedTemplateCover = imgURL != ""
			}
		} else {
			imgCtx, imgCancel := context.WithTimeout(ctx, 2*time.Minute)
			imgURL, err = h.generateBulkCoverImage(imgCtx, title, slug)
			imgCancel()
		}
		if err != nil {
			if bulkGenerationWasCanceled(ctx, err) {
				h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan saat membuat gambar sampul setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, title, imageAIProviderForLog(imgProvider, used))
				return out, context.Canceled
			}
			item := bulkGenerateItemResult{Title: title, Error: err.Error()}
			msg := "cover: " + err.Error()
			if coverMode == "generate_image" {
				msg = imageAILogMessage("cover", imageAIProviderForLog(imgProvider, p), "") + " err=" + err.Error()
			}
			h.persistBulkPostLogByUser(userID, "warn", "generate_item_err", msg, itemCatPtr, nil, title, imageAIProviderForLog(imgProvider, used))
			out = append(out, item)
			nFail++
			if onItem != nil {
				onItem(len(out), nOk, nFail, item)
			}
			continue
		}
		if strings.TrimSpace(imgURL) != "" {
			post.Image = &imgURL
			assignBulkCoverImageOrigin(&post, coverMode, webRes, usedTemplateCover)
			if coverMode == "generate_image" {
				h.persistBulkPostLogByUser(userID, "info", "generate_item_image_ok", imageAILogMessage("cover", imgProvider, imgURL), itemCatPtr, nil, title, imgProvider)
			}
			if req.ImageToContent && coverMode == "generate_image" {
				// Like manual post flow: generate a second image for in-content insertion.
				contentSlug := slug + "-content"
				contentPrompt := strings.TrimSpace(req.ImagePrompt)
				if contentPrompt != "" {
					// If user provides a custom prompt, it would otherwise be identical for cover and in-content.
					// Add a strong nudge to produce a distinct composition for the in-article image.
					contentPrompt += "\n\nIN-ARTICLE IMAGE (not cover): generate a DIFFERENT image from the cover. Change composition and viewpoint; keep topic consistent. No text, logos, or watermarks."
				}
				img2Ctx, img2Cancel := context.WithTimeout(ctx, 2*time.Minute)
				contentImgURL, used2, img2Err := h.generateAICoverImage(img2Ctx, title, contentPrompt, p, contentSlug, true)
				img2Cancel()
				if bulkGenerationWasCanceled(ctx, img2Err) {
					h.persistBulkPostLogByUser(userID, "warn", "generate_canceled", fmt.Sprintf("Dihentikan saat membuat gambar dalam konten setelah %d/%d judul", len(out), len(req.Titles)), itemCatPtr, nil, title, used2)
					return out, context.Canceled
				}
				if img2Err == nil && strings.TrimSpace(contentImgURL) != "" {
					post.Content = injectImageIntoHTMLContent(post.Content, contentImgURL)
					h.persistBulkPostLogByUser(userID, "info", "generate_item_image_ok", imageAILogMessage("content", used2, contentImgURL), itemCatPtr, nil, title, used2)
				} else if img2Err != nil {
					h.persistBulkPostLogByUser(userID, "warn", "generate_item_warn", imageAILogMessage("content", imageAIProviderForLog(used2, p), "")+" err="+img2Err.Error(), itemCatPtr, nil, title, used2)
				} else {
					h.persistBulkPostLogByUser(userID, "warn", "generate_item_warn", imageAILogMessage("content", imageAIProviderForLog(used2, p), "")+" error=Respons kosong", itemCatPtr, nil, title, used2)
				}
			}
		} else if coverMode == "generate_image" {
			// AI cover generation returned empty (no error); log so operators can spot it.
			h.persistBulkPostLogByUser(userID, "warn", "generate_item_warn", imageAILogMessage("cover", imageAIProviderForLog(imgProvider, p), "")+" error=Respons kosong", itemCatPtr, nil, title, imgProvider)
		}
		if err := h.svc.Create(&post); err != nil {
			item := bulkGenerateItemResult{Title: title, Error: err.Error()}
			h.persistBulkPostLogByUser(userID, "warn", "generate_item_err", "save: "+err.Error(), itemCatPtr, nil, title, used)
			out = append(out, item)
			nFail++
			if onItem != nil {
				onItem(len(out), nOk, nFail, item)
			}
			continue
		}
		if h.autoPoster != nil && strings.EqualFold(strings.TrimSpace(post.Status), "published") {
			apCtx, apCancel := context.WithTimeout(ctx, 30*time.Second)
			_ = h.autoPoster.OnPublishedPost(apCtx, &post)
			apCancel()
		}
		pid := post.ID
		h.persistBulkPostLogByUser(userID, "info", "generate_item_ok", fmt.Sprintf("slug=%s status=%s", slug, postStatus), itemCatPtr, &pid, title, used)
		item := bulkGenerateItemResult{Title: title, ID: post.ID}
		out = append(out, item)
		nOk++
		if onItem != nil {
			onItem(len(out), nOk, nFail, item)
		}
	}

	h.persistBulkPostLogByUser(userID, "info", "generate_summary", fmt.Sprintf("dibuat=%d gagal=%d judul_batch=%d", nOk, nFail, len(req.Titles)), catPtr, nil, "", "")
	return out, nil
}

func bulkGenerationWasCanceled(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	// Some provider SDK/client layers return wrapped text without preserving
	// context.Canceled, e.g. "... error sending request ...: context canceled".
	return strings.Contains(strings.ToLower(err.Error()), "context canceled")
}

func injectImageIntoHTMLContent(html, imageURL string) string {
	html = strings.TrimSpace(html)
	imageURL = strings.TrimSpace(imageURL)
	if html == "" || imageURL == "" {
		return html
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"bulk-img-wrap\">" + html + "</div>"))
	if err != nil {
		return html
	}
	root := doc.Find("#bulk-img-wrap")
	if root.Length() == 0 {
		return html
	}
	if root.Find("img").FilterFunction(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		return strings.TrimSpace(src) == imageURL
	}).Length() > 0 {
		if out, e := root.Html(); e == nil {
			return strings.TrimSpace(out)
		}
		return html
	}

	imgHTML := `<p class="ai-inline-image-center" align="center"><img class="ai-inline-image" align="center" src="` + imageURL + `" alt="Generated image" loading="lazy"></p>`
	inserted := false
	h2s := root.Find("h2")
	if h2s.Length() >= 2 {
		h2s.Eq(1).AfterHtml(imgHTML)
		inserted = true
	}
	if !inserted {
		children := root.Children()
		if children.Length() == 0 {
			root.AppendHtml(imgHTML)
		} else {
			mid := children.Length() / 2
			children.Eq(mid).BeforeHtml(imgHTML)
		}
	}
	if out, e := root.Html(); e == nil {
		return strings.TrimSpace(out)
	}
	return html
}

func assignBulkCoverImageOrigin(post *entity.Post, coverMode string, webRes *scraper.WebImageResult, usedTemplateCover bool) {
	if post == nil || post.Image == nil || strings.TrimSpace(*post.Image) == "" {
		return
	}
	switch coverMode {
	case "web_search":
		if usedTemplateCover {
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginTemplate, "")
			return
		}
		detail := ""
		if webRes != nil {
			detail = webRes.Provider
		}
		imagecaption.SetPostImageOrigin(post, entity.ImageOriginWebSearch, detail)
	case "generate_image":
		imagecaption.SetPostImageOrigin(post, entity.ImageOriginAI, "")
	default:
		imagecaption.SetPostImageOrigin(post, entity.ImageOriginTemplate, "")
	}
}

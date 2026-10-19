package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/alsoread"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/autohref"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
	"gorm.io/gorm"
)

// ErrPostSlugTaken is returned when create/update would violate the unique posts.slug index.
var ErrPostSlugTaken = errors.New("a post with this slug already exists")

// post service interface
type PostService interface {
	List(page int, size int, q string, status string, categoryID uint, authorID uint) ([]entity.Post, int64, error)
	ListByAuthor(page int, size int, authorID uint, q string, status string, categoryID uint, filterAuthorID uint) ([]entity.Post, int64, error)
	ListPublished(page int, size int, skip int, categorySlug string, excludeCategorySlug string) ([]entity.Post, int64, error)
	ListPublishedByAuthor(page int, size int, skip int, authorID uint) ([]entity.Post, int64, error)
	ListPublishedByTag(page int, size int, skip int, tagSlug string) ([]entity.Post, int64, error)
	Get(id uint) (*entity.Post, error)
	GetPublishedBySlug(slug string) (*entity.Post, error)
	// RecordPublicView stores at most one unique view per viewerIP per UTC day per post.
	RecordPublicView(postID uint, viewerIP string) (inserted bool, err error)
	// CountPostViews returns total rows in post_views for the post (unique IP×day).
	CountPostViews(postID uint) (int64, error)
	// PopularPublishedSince returns published posts ordered by views since cutoff.
	PopularPublishedSince(cutoff time.Time, limit int) ([]entity.Post, error)
	Create(p *entity.Post) error
	SlugExists(slug string, excludePostID uint) (bool, error)
	Update(id uint, p *entity.Post) (*entity.Post, error)
	BulkUpdateStatus(ids []uint, status string, requesterID uint, requesterRole string) (int64, error)
	Delete(id uint) error
}

// post service struct
type postService struct {
	postRepo     repositories.PostRepository
	categoryRepo repositories.CategoryRepository
	tagRepo      repositories.TagRepository
	userRepo     repositories.UserRepository
	db           *gorm.DB
	cfg          appconfig.Config
}

// function to create a new post service
func NewPostService(
	postRepo repositories.PostRepository,
	categoryRepo repositories.CategoryRepository,
	tagRepo repositories.TagRepository,
	userRepo repositories.UserRepository,
	db *gorm.DB,
	cfg appconfig.Config,
) PostService {
	return &postService{
		postRepo:     postRepo,
		categoryRepo: categoryRepo,
		tagRepo:      tagRepo,
		userRepo:     userRepo,
		db:           db,
		cfg:          cfg,
	}
}

// function to list paginated posts
func (s *postService) List(page int, size int, q string, status string, categoryID uint, authorID uint) ([]entity.Post, int64, error) {
	return s.postRepo.FindPaginated(page, size, q, status, categoryID, authorID)
}

func (s *postService) ListByAuthor(page int, size int, authorID uint, q string, status string, categoryID uint, filterAuthorID uint) ([]entity.Post, int64, error) {
	return s.postRepo.FindPaginatedByAuthor(page, size, authorID, q, status, categoryID, filterAuthorID)
}

func (s *postService) ListPublished(page int, size int, skip int, categorySlug string, excludeCategorySlug string) ([]entity.Post, int64, error) {
	return s.postRepo.FindPublishedPaginated(page, size, skip, categorySlug, excludeCategorySlug)
}

func (s *postService) ListPublishedByAuthor(page int, size int, skip int, authorID uint) ([]entity.Post, int64, error) {
	return s.postRepo.FindPublishedPaginatedByAuthorID(page, size, skip, authorID)
}

func (s *postService) ListPublishedByTag(page int, size int, skip int, tagSlug string) ([]entity.Post, int64, error) {
	tag, err := s.tagRepo.FindBySlug(strings.TrimSpace(tagSlug))
	if err != nil {
		return nil, 0, err
	}
	return s.postRepo.FindPublishedPaginatedByTagID(page, size, skip, tag.ID)
}

// function to get a post by id
func (s *postService) Get(id uint) (*entity.Post, error) {
	return s.postRepo.FindByID(id)
}

// function to get a published post by slug (public)
func (s *postService) GetPublishedBySlug(slug string) (*entity.Post, error) {
	return s.postRepo.FindPublishedBySlug(slug)
}

func (s *postService) RecordPublicView(postID uint, viewerIP string) (bool, error) {
	day := time.Now().UTC().Format("2006-01-02")
	return s.postRepo.RecordUniqueView(postID, viewerIP, day)
}

func (s *postService) CountPostViews(postID uint) (int64, error) {
	return s.postRepo.CountViewsForPost(postID)
}

func (s *postService) PopularPublishedSince(cutoff time.Time, limit int) ([]entity.Post, error) {
	return s.postRepo.FindPopularPublishedSince(cutoff, limit)
}

// function to create a new post
func (s *postService) Create(p *entity.Post) error {
	if p.AuthorID == 0 || p.CategoryID == 0 {
		return errors.New("author_id and category_id required")
	}
	applyPostImageFields(p, p.Image, p.ImageOrigin, p.ImageOriginDetail, nil)
	taken, err := s.postRepo.SlugExists(p.Slug, 0)
	if err != nil {
		return err
	}
	if taken {
		return ErrPostSlugTaken
	}
	switch strings.ToLower(strings.TrimSpace(p.Status)) {
	case "scheduled":
		if p.PublishAt == nil || p.PublishAt.IsZero() {
			return errors.New("scheduled posts require publish_at")
		}
	default:
		p.PublishAt = nil
	}
	manualDesc := cloneNonEmptyMetaPtr(p.MetaDesc)
	manualKw := cloneNonEmptyMetaPtr(p.MetaKeywords)
	s.applyAutoMeta(p)
	if manualDesc != nil {
		p.MetaDesc = manualDesc
	}
	if manualKw != nil {
		p.MetaKeywords = manualKw
	}
	if err := s.applyTagAutoLinks(p); err != nil {
		return err
	}
	s.applyAutoHref(p)
	if err := alsoread.ApplyForPost(s.db, p); err != nil {
		return err
	}
	return s.postRepo.Create(p)
}

func (s *postService) SlugExists(slug string, excludePostID uint) (bool, error) {
	return s.postRepo.SlugExists(slug, excludePostID)
}

// function to update a post
func (s *postService) Update(id uint, p *entity.Post) (*entity.Post, error) {
	existing, err := s.postRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	taken, err := s.postRepo.SlugExists(p.Slug, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrPostSlugTaken
	}
	previousStatus := strings.ToLower(strings.TrimSpace(existing.Status))
	nextStatus := strings.ToLower(strings.TrimSpace(p.Status))
	previousUpdatedAt := existing.UpdatedAt
	existing.Title = p.Title
	existing.Slug = p.Slug
	existing.Content = p.Content
	existing.Status = p.Status
	if existing.IndexingStatus != "" && existing.IndexingStatus != entity.IndexingStatusNotIndexed {
		existing.IndexingStatus = entity.IndexingStatusNotIndexed
		existing.IndexedAt = nil
		existing.IndexingMessage = nil
	}
	switch strings.ToLower(strings.TrimSpace(p.Status)) {
	case "scheduled":
		if p.PublishAt != nil && !p.PublishAt.IsZero() {
			existing.PublishAt = p.PublishAt
		}
	default:
		existing.PublishAt = nil
	}
	existing.CategoryID = p.CategoryID
	existing.AuthorID = p.AuthorID
	existing.Source = p.Source
	existing.IncludeSource = p.IncludeSource
	applyPostImageFields(existing, p.Image, p.ImageOrigin, p.ImageOriginDetail, existing.Image)
	existing.Tags = p.Tags

	manualDesc := cloneNonEmptyMetaPtr(p.MetaDesc)
	manualKw := cloneNonEmptyMetaPtr(p.MetaKeywords)
	s.applyAutoMeta(existing)
	if manualDesc != nil {
		existing.MetaDesc = manualDesc
	}
	if manualKw != nil {
		existing.MetaKeywords = manualKw
	}
	s.applyAutoHref(existing)
	if err := alsoread.ApplyForPost(s.db, existing); err != nil {
		return nil, err
	}
	if previousStatus == nextStatus {
		return existing, s.postRepo.UpdatePreservingUpdatedAt(existing, previousUpdatedAt)
	}
	return existing, s.postRepo.Update(existing)
}

func (s *postService) BulkUpdateStatus(ids []uint, status string, requesterID uint, requesterRole string) (int64, error) {
	st := strings.ToLower(strings.TrimSpace(status))
	if st != "draft" && st != "published" {
		return 0, errors.New("invalid status")
	}
	clean := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return 0, nil
	}
	role := strings.ToLower(strings.TrimSpace(requesterRole))
	var authorID *uint
	if role == "writer" {
		authorID = &requesterID
	}
	n, err := s.postRepo.BulkUpdateStatus(clean, st, authorID)
	if err != nil {
		return n, err
	}
	for _, id := range clean {
		row, getErr := s.postRepo.FindByID(id)
		if getErr != nil || row == nil {
			continue
		}
		if applyErr := alsoread.ApplyForPost(s.db, row); applyErr != nil {
			continue
		}
		_ = s.postRepo.Update(row)
	}
	return n, nil
}

func (s *postService) applyAutoMeta(p *entity.Post) {
	ids := make([]uint, 0, len(p.Tags))
	for _, t := range p.Tags {
		if t.ID != 0 {
			ids = append(ids, t.ID)
		}
	}
	tagNames, _ := s.tagRepo.PluckNamesOrderedByIDs(ids)
	categoryName := ""
	if p.CategoryID != 0 {
		if c, err := s.categoryRepo.FindByID(p.CategoryID); err == nil && c != nil {
			categoryName = strings.TrimSpace(c.Name)
		}
	}
	postutil.Apply(p, tagNames, categoryName)
}

func (s *postService) applyAutoHref(p *entity.Post) {
	if p == nil || strings.TrimSpace(p.Content) == "" {
		return
	}
	ss := repositories.SiteSettingOrDefault(s.db)
	if !ss.AutoHrefEnabled {
		return
	}
	p.Content = autohref.Apply(s.db, ss, p.ID, p.Content)
}

func (s *postService) applyTagAutoLinks(p *entity.Post) error {
	if !repositories.ScraperAutoTagFromDB(s.db) {
		return nil
	}
	if p == nil || strings.TrimSpace(p.Content) == "" {
		return nil
	}
	cands, err := s.collectTagCandidatesForAutoLink(p)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		return nil
	}
	p.Content = postutil.AutoLinkHTML(p.Content, cands, postutil.DefaultMaxTagLinks)
	return nil
}

func (s *postService) collectTagCandidatesForAutoLink(p *entity.Post) ([]entity.Tag, error) {
	ids := make([]uint, 0, len(p.Tags))
	for _, t := range p.Tags {
		if t.ID != 0 {
			ids = append(ids, t.ID)
		}
	}
	byID := make(map[uint]entity.Tag)
	postTags, err := s.tagRepo.FindByIDs(ids)
	if err != nil {
		return nil, err
	}
	for _, t := range postTags {
		byID[t.ID] = t
	}
	// When site "Scraper auto-tag" is off: only autolink tags explicitly attached to the post
	// (no meta_keywords expansion, no site-wide tag pool). Matches bulk/scraper behaviour.
	if repositories.ScraperAutoTagFromDB(s.db) {
		if p.MetaKeywords != nil {
			var parts []string
			for _, part := range strings.Split(*p.MetaKeywords, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					parts = append(parts, part)
				}
			}
			if len(parts) > 0 {
				kwTags, err := s.tagRepo.FindByNamesCaseInsensitive(parts)
				if err != nil {
					return nil, err
				}
				for _, t := range kwTags {
					if _, ok := byID[t.ID]; !ok {
						byID[t.ID] = t
					}
				}
			}
		}
		pool, err := s.tagRepo.FindAllForAutoLink(300)
		if err != nil {
			return nil, err
		}
		for _, t := range pool {
			if _, ok := byID[t.ID]; !ok {
				byID[t.ID] = t
			}
		}
	}
	out := make([]entity.Tag, 0, len(byID))
	for _, t := range byID {
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].Name) > len(out[j].Name)
	})
	return out, nil
}

// function to delete a post
func (s *postService) Delete(id uint) error {
	p, err := s.postRepo.FindByID(id)
	if err != nil {
		return err
	}
	ctx := context.Background()
	store := storage.PostAssetStore(ctx, s.db, s.cfg)
	storage.DeletePostObjectURLs(ctx, store, p)
	return s.postRepo.Delete(id)
}

func cloneNonEmptyMetaPtr(in *string) *string {
	if in == nil {
		return nil
	}
	v := strings.TrimSpace(*in)
	if v == "" {
		return nil
	}
	return &v
}

// applyPostImageFields normalizes image URL and cover origin metadata on create/update.
func applyPostImageFields(post *entity.Post, image *string, origin, detail string, previousImage *string) {
	if post == nil {
		return
	}
	if image != nil {
		t := strings.TrimSpace(*image)
		if t == "" {
			post.Image = nil
			postutil.ClearPostImageOrigin(post)
			return
		}
		post.Image = &t
	}
	if post.Image == nil || strings.TrimSpace(*post.Image) == "" {
		postutil.ClearPostImageOrigin(post)
		return
	}
	unchanged := false
	if previousImage != nil && strings.TrimSpace(*previousImage) == strings.TrimSpace(*post.Image) {
		unchanged = true
	}
	o := postutil.NormalizeOrigin(origin)
	if o != "" {
		postutil.SetPostImageOrigin(post, o, detail)
		return
	}
	if unchanged {
		return
	}
	postutil.SetPostImageOrigin(post, entity.ImageOriginManual, "")
}

package services

import (
	"errors"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
)

// ErrPageSlugTaken is returned when create/update would violate the unique pages.slug index.
var ErrPageSlugTaken = errors.New("a page with this slug already exists")

// page service interface
type PageService interface {
	List(page int, size int, q string) ([]entity.Page, int64, error)
	ListPublished(page int, size int, skip int) ([]entity.Page, int64, error)
	Get(id uint) (*entity.Page, error)
	GetPublishedBySlug(slug string) (*entity.Page, error)
	Create(p *entity.Page) error
	Update(id uint, p *entity.Page) (*entity.Page, error)
	BulkUpdateStatus(ids []uint, status string) (int64, error)
	Delete(id uint) error
}

// page service struct
type pageService struct {
	pageRepo repositories.PageRepository
}

// function to create a new page service
func NewPageService(
	pageRepo repositories.PageRepository,
) PageService {
	return &pageService{
		pageRepo: pageRepo,
	}
}

// function to list paginated pages
func (s *pageService) List(page int, size int, q string) ([]entity.Page, int64, error) {
	return s.pageRepo.FindPaginated(page, size, q)
}

func (s *pageService) ListPublished(page int, size int, skip int) ([]entity.Page, int64, error) {
	return s.pageRepo.FindPublishedPaginated(page, size, skip)
}

// function to get a page by id
func (s *pageService) Get(id uint) (*entity.Page, error) {
	return s.pageRepo.FindByID(id)
}

// function to get a published page by slug (public)
func (s *pageService) GetPublishedBySlug(slug string) (*entity.Page, error) {
	return s.pageRepo.FindPublishedBySlug(slug)
}

// function to create a new page
func (s *pageService) Create(p *entity.Page) error {
	taken, err := s.pageRepo.SlugExists(p.Slug, 0)
	if err != nil {
		return err
	}
	if taken {
		return ErrPageSlugTaken
	}
	return s.pageRepo.Create(p)
}

// function to update a page
func (s *pageService) Update(id uint, p *entity.Page) (*entity.Page, error) {
	existing, err := s.pageRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	taken, err := s.pageRepo.SlugExists(p.Slug, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrPageSlugTaken
	}
	existing.Title = p.Title
	existing.Slug = p.Slug
	existing.Content = p.Content
	existing.Status = p.Status

	return existing, s.pageRepo.Update(existing)
}

func (s *pageService) BulkUpdateStatus(ids []uint, status string) (int64, error) {
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
	return s.pageRepo.BulkUpdateStatus(clean, st)
}

// function to delete a page
func (s *pageService) Delete(id uint) error {
	return s.pageRepo.Delete(id)
}

package services

import (
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

// tag service interface
type TagService interface {
	List(page int, size int, q string) ([]entity.Tag, int64, error)
	Get(id uint) (*entity.Tag, error)
	GetBySlug(slug string) (*entity.Tag, error)
	Create(t *entity.Tag) error
	Update(id uint, t *entity.Tag) (*entity.Tag, error)
	Delete(id uint) error
}

// tag service struct
type tagService struct {
	repo repositories.TagRepository
}

// function to create a new tag service
func NewTagService(r repositories.TagRepository) TagService {
	return &tagService{repo: r}
}

// function to list paginated tags
func (s *tagService) List(page int, size int, q string) ([]entity.Tag, int64, error) {
	return s.repo.FindPaginated(page, size, q)
}

// function to get a tag by id
func (s *tagService) Get(id uint) (*entity.Tag, error) {
	return s.repo.FindByID(id)
}

func (s *tagService) GetBySlug(slug string) (*entity.Tag, error) {
	return s.repo.FindBySlug(slug)
}

// function to create a new tag
func (s *tagService) Create(t *entity.Tag) error {
	return s.repo.Create(t)
}

// function to update a tag
func (s *tagService) Update(id uint, t *entity.Tag) (*entity.Tag, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	existing.Name = t.Name
	existing.Slug = t.Slug
	if existing.IndexingStatus != "" && existing.IndexingStatus != entity.IndexingStatusNotIndexed {
		existing.IndexingStatus = entity.IndexingStatusNotIndexed
		existing.IndexedAt = nil
		existing.IndexingMessage = ""
	}
	return existing, s.repo.Update(existing)
}

// function to delete a tag
func (s *tagService) Delete(id uint) error {
	return s.repo.Delete(id)
}

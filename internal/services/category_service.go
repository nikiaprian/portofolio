package services

import (
	"context"
	"errors"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
	"gorm.io/gorm"
)

// category service interface
type CategoryService interface {
	List(page int, size int, q string, onlyEnabled bool) ([]entity.Category, int64, error)
	Get(id uint) (*entity.Category, error)
	Create(c *entity.Category) error
	Update(id uint, c *entity.Category) (*entity.Category, error)
	Delete(id uint) error
}

// category service struct
type categoryService struct {
	repo     repositories.CategoryRepository
	postRepo repositories.PostRepository
	db       *gorm.DB
	cfg      appconfig.Config
}

// function to create a new category service
func NewCategoryService(r repositories.CategoryRepository, postRepo repositories.PostRepository, db *gorm.DB, cfg appconfig.Config) CategoryService {
	return &categoryService{repo: r, postRepo: postRepo, db: db, cfg: cfg}
}

// function to list paginated categories
func (s *categoryService) List(page int, size int, q string, onlyEnabled bool) ([]entity.Category, int64, error) {
	return s.repo.FindPaginated(page, size, q, onlyEnabled)
}

// function to get a category by id
func (s *categoryService) Get(id uint) (*entity.Category, error) {
	return s.repo.FindByID(id)
}

// function to create a new category
func (s *categoryService) Create(c *entity.Category) error {
	normalizeCategoryParentID(c)
	if err := s.validateParent(0, c.ParentID); err != nil {
		return err
	}
	return s.repo.Create(c)
}

// function to update a category
func (s *categoryService) Update(id uint, c *entity.Category) (*entity.Category, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	normalizeCategoryParentID(c)
	if err := s.validateParent(id, c.ParentID); err != nil {
		return nil, err
	}
	existing.Name = c.Name
	existing.Slug = c.Slug
	existing.Sort = c.Sort
	existing.Title = c.Title
	existing.Keyword = c.Keyword
	existing.Description = c.Description
	existing.ParentID = c.ParentID
	existing.Enabled = c.Enabled
	if existing.IndexingStatus != "" && existing.IndexingStatus != entity.IndexingStatusNotIndexed {
		existing.IndexingStatus = entity.IndexingStatusNotIndexed
		existing.IndexedAt = nil
		existing.IndexingMessage = ""
	}
	return existing, s.repo.Update(existing)
}

// function to delete a category
func (s *categoryService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return err
	}
	n, err := s.repo.CountChildren(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrCategoryHasChildren
	}
	posts, err := s.postRepo.FindByCategoryIDForAssetCleanup(id)
	if err != nil {
		return err
	}
	ctx := context.Background()
	store := storage.PostAssetStore(ctx, s.db, s.cfg)
	for i := range posts {
		storage.DeletePostObjectURLs(ctx, store, &posts[i])
	}
	return s.repo.Delete(id)
}

func normalizeCategoryParentID(c *entity.Category) {
	if c == nil || c.ParentID == nil || *c.ParentID == 0 {
		c.ParentID = nil
	}
}

func (s *categoryService) validateParent(categoryID uint, parentID *uint) error {
	if parentID == nil {
		return nil
	}
	if categoryID != 0 && *parentID == categoryID {
		return ErrCategoryInvalidParent
	}
	parent, err := s.repo.FindByID(*parentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCategoryParentNotFound
		}
		return err
	}
	if !parent.IsRoot() {
		return ErrCategoryMaxDepth
	}
	if categoryID != 0 {
		n, err := s.repo.CountChildren(categoryID)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrCategoryHasChildren
		}
	}
	return nil
}

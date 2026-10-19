package services

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

// SiteLayoutService manages custom layout templates.
type SiteLayoutService interface {
	List(page int, size int, q string) ([]entity.SiteLayout, int64, error)
	Get(id uint) (*entity.SiteLayout, error)
	Create(row *entity.SiteLayout) error
	Update(id uint, row *entity.SiteLayout) (*entity.SiteLayout, error)
	ApplyFragment(id uint, fragment, content string) (*entity.SiteLayout, error)
	// SetUsedExclusive marks one layout as active and clears is_used on all others.
	SetUsedExclusive(id uint) (*entity.SiteLayout, error)
	Duplicate(id uint, layoutName string) (*entity.SiteLayout, error)
	Delete(id uint) error
}

type siteLayoutService struct {
	repo repositories.SiteLayoutRepository
}

func NewSiteLayoutService(r repositories.SiteLayoutRepository) SiteLayoutService {
	return &siteLayoutService{repo: r}
}

func (s *siteLayoutService) List(page int, size int, q string) ([]entity.SiteLayout, int64, error) {
	return s.repo.FindPaginated(page, size, q)
}

func (s *siteLayoutService) Get(id uint) (*entity.SiteLayout, error) {
	row, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteLayoutNotFound
		}
		return nil, err
	}
	return row, nil
}

func (s *siteLayoutService) Create(row *entity.SiteLayout) error {
	if row == nil {
		return errors.New("layout required")
	}
	row.LayoutName = strings.TrimSpace(row.LayoutName)
	if row.LayoutName == "" {
		return errors.New("layout name required")
	}
	row.IsUsed = false
	return s.repo.Create(row)
}

func (s *siteLayoutService) Update(id uint, row *entity.SiteLayout) (*entity.SiteLayout, error) {
	if row == nil {
		return nil, errors.New("layout required")
	}
	existing, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(row.LayoutName)
	if name == "" {
		return nil, errors.New("layout name required")
	}
	existing.LayoutName = name
	existing.Header = row.Header
	existing.Footer = row.Footer
	existing.HomeBody = row.HomeBody
	existing.ArticleBody = row.ArticleBody
	existing.Sidebar = row.Sidebar
	existing.RSS = row.RSS
	existing.AuthorProfile = row.AuthorProfile
	existing.Tag = row.Tag
	existing.Kategori = row.Kategori
	existing.SearchPage = row.SearchPage
	existing.Misc = row.Misc
	existing.LatestPrompt = row.LatestPrompt
	existing.CurrentPrompt = row.CurrentPrompt
	existing.ImageRef = row.ImageRef
	existing.CSSData = row.CSSData
	existing.JSData = row.JSData
	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func applyFragmentContent(row *entity.SiteLayout, fragment, content string) error {
	switch strings.TrimSpace(fragment) {
	case "header":
		row.Header = content
	case "footer":
		row.Footer = content
	case "home_body":
		row.HomeBody = content
	case "article_body":
		row.ArticleBody = content
	case "sidebar":
		row.Sidebar = content
	case "rss":
		row.RSS = content
	case "author_profile":
		row.AuthorProfile = content
	case "tag":
		row.Tag = content
	case "kategori":
		row.Kategori = content
	case "search_page":
		row.SearchPage = content
	default:
		return errors.New("fragment tidak valid")
	}
	return nil
}

func (s *siteLayoutService) ApplyFragment(id uint, fragment, content string) (*entity.SiteLayout, error) {
	existing, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if err := applyFragmentContent(existing, fragment, content); err != nil {
		return nil, err
	}
	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *siteLayoutService) SetUsedExclusive(id uint) (*entity.SiteLayout, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	if err := s.repo.SetUsedExclusive(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSiteLayoutNotFound
		}
		return nil, err
	}
	return s.Get(id)
}

func (s *siteLayoutService) Duplicate(id uint, layoutName string) (*entity.SiteLayout, error) {
	src, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	layoutName = strings.TrimSpace(layoutName)
	if layoutName == "" {
		return nil, errors.New("layout name required")
	}
	dup := entity.SiteLayout{
		LayoutName:    layoutName,
		Header:        src.Header,
		Footer:        src.Footer,
		HomeBody:      src.HomeBody,
		ArticleBody:   src.ArticleBody,
		Sidebar:       src.Sidebar,
		RSS:           src.RSS,
		AuthorProfile: src.AuthorProfile,
		Tag:           src.Tag,
		Kategori:      src.Kategori,
		SearchPage:    src.SearchPage,
		Misc:          src.Misc,
		LatestPrompt:  src.LatestPrompt,
		CurrentPrompt: src.CurrentPrompt,
		ImageRef:      src.ImageRef,
		CSSData:       src.CSSData,
		JSData:        src.JSData,
		IsUsed:        false,
	}
	if err := s.repo.Create(&dup); err != nil {
		return nil, err
	}
	return &dup, nil
}

func (s *siteLayoutService) Delete(id uint) error {
	row, err := s.Get(id)
	if err != nil {
		return err
	}
	if row.IsUsed {
		return ErrSiteLayoutInUse
	}
	return s.repo.Delete(id)
}

package services

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
)

// SiteCloudStorageService reads and updates cloud storage settings.
type SiteCloudStorageService interface {
	Get() (*entity.SiteCloudStorage, error)
	Update(req *dto.UpdateSiteCloudStorageRequest) (*entity.SiteCloudStorage, error)
}

type siteCloudStorageService struct {
	repo repositories.SiteCloudStorageRepository
}

// NewSiteCloudStorageService constructs SiteCloudStorageService.
func NewSiteCloudStorageService(r repositories.SiteCloudStorageRepository) SiteCloudStorageService {
	return &siteCloudStorageService{repo: r}
}

func (s *siteCloudStorageService) Get() (*entity.SiteCloudStorage, error) {
	row, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d := entity.DefaultSiteCloudStorage()
			return &d, nil
		}
		return nil, err
	}
	return row, nil
}

func (s *siteCloudStorageService) Update(req *dto.UpdateSiteCloudStorageRequest) (*entity.SiteCloudStorage, error) {
	r2Bucket := strings.TrimSpace(req.R2Bucket)
	if r2Bucket == "" {
		r2Bucket = "media"
	}
	r2Prefix := strings.TrimSpace(req.R2Prefix)
	if r2Prefix == "" {
		r2Prefix = "uploads"
	}

	existing, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := &entity.SiteCloudStorage{
				R2AccountID:       strings.TrimSpace(req.R2AccountID),
				R2AccessKeyID:     strings.TrimSpace(req.R2AccessKeyID),
				R2SecretAccessKey: strings.TrimSpace(req.R2SecretAccessKey),
				R2Bucket:          r2Bucket,
				R2PublicBaseURL:   strings.TrimSpace(req.R2PublicBaseURL),
				R2Prefix:          r2Prefix,
			}
			if err := s.repo.Create(created); err != nil {
				return nil, err
			}
			return created, nil
		}
		return nil, err
	}

	existing.R2AccountID = strings.TrimSpace(req.R2AccountID)
	existing.R2AccessKeyID = strings.TrimSpace(req.R2AccessKeyID)
	existing.R2SecretAccessKey = strings.TrimSpace(req.R2SecretAccessKey)
	existing.R2Bucket = r2Bucket
	existing.R2PublicBaseURL = strings.TrimSpace(req.R2PublicBaseURL)
	existing.R2Prefix = r2Prefix

	if err := s.repo.Save(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
)

// SiteConnectionService reads and updates webmaster verification codes.
type SiteConnectionService interface {
	Get() (*entity.SiteConnection, error)
	Update(req *dto.UpdateSiteConnectionRequest) (*entity.SiteConnection, error)
}

type siteConnectionService struct {
	repo repositories.SiteConnectionRepository
}

type oauthDefaults struct {
	tokenURL   string
	publishURL string
	scope      string
}

var hardcodedOAuthDefaults = map[string]oauthDefaults{
	"facebook": {
		tokenURL:   "https://graph.facebook.com/v24.0/oauth/access_token",
		publishURL: "https://graph.facebook.com/v24.0/{account_id}/feed",
		scope:      "pages_manage_posts pages_read_engagement",
	},
	"instagram": {
		tokenURL:   "https://graph.facebook.com/v24.0/oauth/access_token",
		publishURL: "https://graph.facebook.com/v24.0/{account_id}/media",
		scope:      "instagram_basic instagram_content_publish pages_show_list",
	},
	"x": {
		tokenURL:   "https://api.twitter.com/2/oauth2/token",
		publishURL: "https://api.twitter.com/2/tweets",
		scope:      "tweet.write users.read offline.access",
	},
	"tiktok": {
		tokenURL:   "https://open.tiktokapis.com/v2/oauth/token/",
		publishURL: "https://open.tiktokapis.com/v2/post/publish/content/init/",
		scope:      "user.info.basic video.publish",
	},
}

// NewSiteConnectionService constructs SiteConnectionService.
func NewSiteConnectionService(r repositories.SiteConnectionRepository) SiteConnectionService {
	return &siteConnectionService{repo: r}
}

func (s *siteConnectionService) Get() (*entity.SiteConnection, error) {
	sc, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entity.SiteConnection{}, nil
		}
		return nil, err
	}
	return sc, nil
}

func (s *siteConnectionService) Update(req *dto.UpdateSiteConnectionRequest) (*entity.SiteConnection, error) {
	applyHardcodedOAuthDefaults(req)
	if err := validateOAuthConnection(req); err != nil {
		return nil, err
	}
	existing, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := &entity.SiteConnection{
				Ahrefs:                req.Ahrefs,
				Google:                req.Google,
				Bing:                  req.Bing,
				Yandex:                req.Yandex,
				AutoPostEnabled:       req.AutoPostEnabled,
				AutoPostTemplate:      strings.TrimSpace(req.AutoPostTemplate),
				FacebookEnabled:       req.FacebookEnabled,
				FacebookClientID:      strings.TrimSpace(req.FacebookClientID),
				FacebookClientSecret:  strings.TrimSpace(req.FacebookClientSecret),
				FacebookTokenURL:      strings.TrimSpace(req.FacebookTokenURL),
				FacebookPublishURL:    strings.TrimSpace(req.FacebookPublishURL),
				FacebookScope:         strings.TrimSpace(req.FacebookScope),
				FacebookAccountID:     strings.TrimSpace(req.FacebookAccountID),
				XEnabled:              req.XEnabled,
				XClientID:             strings.TrimSpace(req.XClientID),
				XClientSecret:         strings.TrimSpace(req.XClientSecret),
				XTokenURL:             strings.TrimSpace(req.XTokenURL),
				XPublishURL:           strings.TrimSpace(req.XPublishURL),
				XScope:                strings.TrimSpace(req.XScope),
				XAccountID:            strings.TrimSpace(req.XAccountID),
				TiktokEnabled:         req.TiktokEnabled,
				TiktokClientID:        strings.TrimSpace(req.TiktokClientID),
				TiktokClientSecret:    strings.TrimSpace(req.TiktokClientSecret),
				TiktokTokenURL:        strings.TrimSpace(req.TiktokTokenURL),
				TiktokPublishURL:      strings.TrimSpace(req.TiktokPublishURL),
				TiktokScope:           strings.TrimSpace(req.TiktokScope),
				TiktokAccountID:       strings.TrimSpace(req.TiktokAccountID),
				InstagramEnabled:      req.InstagramEnabled,
				InstagramClientID:     strings.TrimSpace(req.InstagramClientID),
				InstagramClientSecret: strings.TrimSpace(req.InstagramClientSecret),
				InstagramTokenURL:     strings.TrimSpace(req.InstagramTokenURL),
				InstagramPublishURL:   strings.TrimSpace(req.InstagramPublishURL),
				InstagramScope:        strings.TrimSpace(req.InstagramScope),
				InstagramAccountID:    strings.TrimSpace(req.InstagramAccountID),
				GoogleIndexingEnabled:    req.GoogleIndexingEnabled,
				GoogleServiceAccountJSON: strings.TrimSpace(req.GoogleServiceAccountJSON),
			}
			if err := s.repo.Create(created); err != nil {
				return nil, err
			}
			return created, nil
		}
		return nil, err
	}

	existing.Ahrefs = req.Ahrefs
	existing.Google = req.Google
	existing.Bing = req.Bing
	existing.Yandex = req.Yandex
	existing.AutoPostEnabled = req.AutoPostEnabled
	existing.AutoPostTemplate = strings.TrimSpace(req.AutoPostTemplate)
	existing.FacebookEnabled = req.FacebookEnabled
	existing.FacebookClientID = strings.TrimSpace(req.FacebookClientID)
	existing.FacebookClientSecret = strings.TrimSpace(req.FacebookClientSecret)
	existing.FacebookTokenURL = strings.TrimSpace(req.FacebookTokenURL)
	existing.FacebookPublishURL = strings.TrimSpace(req.FacebookPublishURL)
	existing.FacebookScope = strings.TrimSpace(req.FacebookScope)
	existing.FacebookAccountID = strings.TrimSpace(req.FacebookAccountID)
	existing.XEnabled = req.XEnabled
	existing.XClientID = strings.TrimSpace(req.XClientID)
	existing.XClientSecret = strings.TrimSpace(req.XClientSecret)
	existing.XTokenURL = strings.TrimSpace(req.XTokenURL)
	existing.XPublishURL = strings.TrimSpace(req.XPublishURL)
	existing.XScope = strings.TrimSpace(req.XScope)
	existing.XAccountID = strings.TrimSpace(req.XAccountID)
	existing.TiktokEnabled = req.TiktokEnabled
	existing.TiktokClientID = strings.TrimSpace(req.TiktokClientID)
	existing.TiktokClientSecret = strings.TrimSpace(req.TiktokClientSecret)
	existing.TiktokTokenURL = strings.TrimSpace(req.TiktokTokenURL)
	existing.TiktokPublishURL = strings.TrimSpace(req.TiktokPublishURL)
	existing.TiktokScope = strings.TrimSpace(req.TiktokScope)
	existing.TiktokAccountID = strings.TrimSpace(req.TiktokAccountID)
	existing.InstagramEnabled = req.InstagramEnabled
	existing.InstagramClientID = strings.TrimSpace(req.InstagramClientID)
	existing.InstagramClientSecret = strings.TrimSpace(req.InstagramClientSecret)
	existing.InstagramTokenURL = strings.TrimSpace(req.InstagramTokenURL)
	existing.InstagramPublishURL = strings.TrimSpace(req.InstagramPublishURL)
	existing.InstagramScope = strings.TrimSpace(req.InstagramScope)
	existing.InstagramAccountID = strings.TrimSpace(req.InstagramAccountID)
	existing.GoogleIndexingEnabled = req.GoogleIndexingEnabled
	existing.GoogleServiceAccountJSON = strings.TrimSpace(req.GoogleServiceAccountJSON)

	if err := s.repo.Save(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func applyHardcodedOAuthDefaults(req *dto.UpdateSiteConnectionRequest) {
	req.FacebookTokenURL = hardcodedOAuthDefaults["facebook"].tokenURL
	req.FacebookPublishURL = hardcodedOAuthDefaults["facebook"].publishURL
	req.FacebookScope = hardcodedOAuthDefaults["facebook"].scope
	req.InstagramTokenURL = hardcodedOAuthDefaults["instagram"].tokenURL
	req.InstagramPublishURL = hardcodedOAuthDefaults["instagram"].publishURL
	req.InstagramScope = hardcodedOAuthDefaults["instagram"].scope
	req.XTokenURL = hardcodedOAuthDefaults["x"].tokenURL
	req.XPublishURL = hardcodedOAuthDefaults["x"].publishURL
	req.XScope = hardcodedOAuthDefaults["x"].scope
	req.TiktokTokenURL = hardcodedOAuthDefaults["tiktok"].tokenURL
	req.TiktokPublishURL = hardcodedOAuthDefaults["tiktok"].publishURL
	req.TiktokScope = hardcodedOAuthDefaults["tiktok"].scope
}

func validateOAuthConnection(req *dto.UpdateSiteConnectionRequest) error {
	type oauthInput struct {
		enabled      bool
		name         string
		clientID     string
		clientSecret string
		tokenURL     string
		publishURL   string
	}
	items := []oauthInput{
		{
			enabled:      req.FacebookEnabled,
			name:         "facebook",
			clientID:     req.FacebookClientID,
			clientSecret: req.FacebookClientSecret,
			tokenURL:     req.FacebookTokenURL,
			publishURL:   req.FacebookPublishURL,
		},
		{
			enabled:      req.InstagramEnabled,
			name:         "instagram",
			clientID:     req.InstagramClientID,
			clientSecret: req.InstagramClientSecret,
			tokenURL:     req.InstagramTokenURL,
			publishURL:   req.InstagramPublishURL,
		},
		{
			enabled:      req.XEnabled,
			name:         "x",
			clientID:     req.XClientID,
			clientSecret: req.XClientSecret,
			tokenURL:     req.XTokenURL,
			publishURL:   req.XPublishURL,
		},
		{
			enabled:      req.TiktokEnabled,
			name:         "tiktok",
			clientID:     req.TiktokClientID,
			clientSecret: req.TiktokClientSecret,
			tokenURL:     req.TiktokTokenURL,
			publishURL:   req.TiktokPublishURL,
		},
	}
	for _, it := range items {
		if !it.enabled {
			continue
		}
		missing := make([]string, 0, 4)
		if strings.TrimSpace(it.clientID) == "" {
			missing = append(missing, "client_id")
		}
		if strings.TrimSpace(it.clientSecret) == "" {
			missing = append(missing, "client_secret")
		}
		if strings.TrimSpace(it.tokenURL) == "" {
			missing = append(missing, "token_url")
		}
		if strings.TrimSpace(it.publishURL) == "" {
			missing = append(missing, "publish_url")
		}
		if len(missing) > 0 {
			return fmt.Errorf("%s oauth requires: %s", it.name, strings.Join(missing, ", "))
		}
	}
	return nil
}

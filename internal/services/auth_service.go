package services

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/middleware"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
)

// auth service interface
type AuthService interface {
	Register(req dto.RegisterRequest) (*entity.User, error)
	Login(req dto.LoginRequest, clientIP, userAgent string) (string, *entity.User, error)
}

// auth service struct
type authService struct {
	userRepo  repositories.UserRepository
	jwtManger *middleware.JWTManager
	db        *gorm.DB
}

// function to create a new auth service
func NewAuthService(userRepo repositories.UserRepository, jwtMgr *middleware.JWTManager, db *gorm.DB) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtManger: jwtMgr,
		db:        db,
	}
}

// function to register a new user
func (s *authService) Register(req dto.RegisterRequest) (*entity.User, error) {
	if existing, _ := s.userRepo.FindByEmail(req.Email); existing != nil {
		return nil, errors.New("email already in use")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Name:     req.Name,
		Username: strings.Split(strings.TrimSpace(req.Email), "@")[0],
		Email:    req.Email,
		Password: string(hashed),
		Role:     "writer",
		Enabled:  true,
		Profile:  "",
	}
	user.Role = strings.ToLower(strings.TrimSpace(user.Role))

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func normalizeLoginEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func truncateLoginLogStr(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func (s *authService) recordLoginLog(email string, userID *uint, success bool, failureReason, ip, userAgent string) {
	if s.db == nil {
		return
	}
	row := entity.LoginLog{
		Email:         truncateLoginLogStr(email, 255),
		UserID:        userID,
		Success:       success,
		FailureReason: truncateLoginLogStr(failureReason, 500),
		IP:            truncateLoginLogStr(ip, 64),
		UserAgent:     truncateLoginLogStr(userAgent, 2000),
	}
	if err := s.db.Create(&row).Error; err != nil {
		// Non-fatal: login must still complete; avoid import cycle with log package noise in tests
		_ = err
	}
}

// function to login a user
func (s *authService) Login(req dto.LoginRequest, clientIP, userAgent string) (string, *entity.User, error) {
	emailTrim := strings.TrimSpace(req.Email)
	email := normalizeLoginEmail(emailTrim)
	user, err := s.userRepo.FindByEmail(emailTrim)
	if err != nil {
		reason := "invalid email or password"
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			reason = "lookup failed"
		}
		s.recordLoginLog(email, nil, false, reason, clientIP, userAgent)
		return "", nil, errors.New("invalid email or password")
	}
	if !user.Enabled {
		uid := user.ID
		s.recordLoginLog(email, &uid, false, "account disabled", clientIP, userAgent)
		return "", nil, errors.New("account disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		uid := user.ID
		s.recordLoginLog(email, &uid, false, "invalid email or password", clientIP, userAgent)
		return "", nil, errors.New("invalid email or password")
	}

	token, err := s.jwtManger.Generate(user.ID)
	if err != nil {
		uid := user.ID
		s.recordLoginLog(email, &uid, false, "token generation failed", clientIP, userAgent)
		return "", nil, err
	}

	uid := user.ID
	s.recordLoginLog(email, &uid, true, "", clientIP, userAgent)
	return token, user, nil
}

package services

import (
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
)

func sanitizeUsername(raw string) string {
	u := strings.ToLower(strings.TrimSpace(raw))
	if u == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range u {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 100 {
		out = out[:100]
		out = strings.Trim(out, "-")
	}
	return out
}

// user service interface
type UserService interface {
	List(page int, size int, q string) ([]entity.User, int64, error)
	Get(id uint) (*entity.User, error)
	Create(u *entity.User) error
	Update(id uint, u *entity.User) (*entity.User, error)
	Delete(id uint) error
}

// user service struct
type userService struct {
	repo repositories.UserRepository
}

// function to create a new user service
func NewUserService(r repositories.UserRepository) UserService {
	return &userService{repo: r}
}

// function to list paginated users
func (s *userService) List(page int, size int, q string) ([]entity.User, int64, error) {
	return s.repo.FindPaginated(page, size, q)
}

// function to get a user by id
func (s *userService) Get(id uint) (*entity.User, error) {
	return s.repo.FindByID(id)
}

// function to create a new user
func (s *userService) Create(u *entity.User) error {
	u.Role = strings.ToLower(strings.TrimSpace(u.Role))
	if u.Role == "" {
		u.Role = "writer"
	}
	if u.Role != "admin" && u.Role != "writer" {
		u.Role = "writer"
	}
	u.Profile = strings.TrimSpace(u.Profile)
	if u.Image != nil {
		img := strings.TrimSpace(*u.Image)
		if img == "" {
			u.Image = nil
		} else {
			u.Image = &img
		}
	}
	if u.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u.Password = string(hashed)
	}
	if u.Username = sanitizeUsername(u.Username); u.Username == "" {
		base := u.Email
		if i := strings.IndexByte(base, '@'); i > 0 {
			base = base[:i]
		}
		if x := sanitizeUsername(base); x != "" {
			u.Username = x
		} else {
			u.Username = sanitizeUsername(u.Name)
		}
	}
	return s.repo.Create(u)
}

// function to update a user
func (s *userService) Update(id uint, u *entity.User) (*entity.User, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	existing.Name = u.Name
	if x := sanitizeUsername(u.Username); x != "" {
		existing.Username = x
	}
	existing.Email = u.Email
	if r := strings.ToLower(strings.TrimSpace(u.Role)); r == "admin" || r == "writer" {
		existing.Role = r
	}
	// Handler preserves existing.Enabled when req.enabled is omitted, so it's safe to assign here.
	existing.Enabled = u.Enabled
	existing.Profile = strings.TrimSpace(u.Profile)
	if u.Image != nil {
		img := strings.TrimSpace(*u.Image)
		if img == "" {
			existing.Image = nil
		} else {
			existing.Image = &img
		}
	}
	if u.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		existing.Password = string(hashed)
	}
	return existing, s.repo.Update(existing)
}

// function to delete a user
func (s *userService) Delete(id uint) error {
	return s.repo.Delete(id)
}

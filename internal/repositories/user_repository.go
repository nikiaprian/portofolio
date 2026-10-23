package repositories

import (
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// user repository interface
type UserRepository interface {
	FindPaginated(page int, size int, q string) ([]entity.User, int64, error)
	FindByID(id uint) (*entity.User, error)
	FindByEmail(email string) (*entity.User, error)
	Create(u *entity.User) error
	Update(u *entity.User) error
	Delete(id uint) error
}

// user repository struct
type userRepository struct {
	db *gorm.DB
}

// function to create a new user repository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// function to find paginated users
func (r *userRepository) FindPaginated(page int, size int, q string) ([]entity.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.User{})
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		base = base.Where(
			"LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(role) LIKE ?",
			like, like, like,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []entity.User
	err := base.Order("updated_at desc").Limit(size).Offset(offset).Find(&users).Error
	return users, total, err
}

// function to find a user by id
func (r *userRepository) FindByID(id uint) (*entity.User, error) {
	var user entity.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// function to find a user by email
func (r *userRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	email = strings.TrimSpace(email)
	if err := r.db.Where("LOWER(email) = LOWER(?)", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// function to create a new user
func (r *userRepository) Create(u *entity.User) error {
	return r.db.Create(u).Error
}

// function to update a user
func (r *userRepository) Update(u *entity.User) error {
	return r.db.Save(u).Error
}

// function to delete a user
func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&entity.User{}, id).Error
}

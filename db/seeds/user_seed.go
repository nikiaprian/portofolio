package seeds

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedUsers(db *gorm.DB) (*entity.User, error) {
	hashedAdmin, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	admin := &entity.User{
		Name:     "Super Admin",
		Email:    "admin@bebii.com",
		Password: string(hashedAdmin),
		Role:     "admin",
		Username: "admin",
		Profile:  "Administrator account",
	}
	var existingAdmin entity.User
	if err := db.Where("email = ?", admin.Email).First(&existingAdmin).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err := db.Create(admin).Error; err != nil {
			return nil, err
		}
	} else {
		admin = &existingAdmin
	}

	hashedWriter, err := bcrypt.GenerateFromPassword([]byte("writer123"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	writer := &entity.User{
		Name:     "Writer User",
		Email:    "writer@bebii.com",
		Password: string(hashedWriter),
		Role:     "writer",
		Username: "writer",
		Profile:  "Writer account",
	}
	var existingWriter entity.User
	if err := db.Where("email = ?", writer.Email).First(&existingWriter).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err := db.Create(writer).Error; err != nil {
			return nil, err
		}
	}
	return admin, nil
}

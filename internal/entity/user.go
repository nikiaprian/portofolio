package entity

import "time"

// user entity model
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Username  string    `gorm:"size:100;index;not null;default:''" json:"username"`
	Email     string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Role      string    `gorm:"size:20;not null;default:writer" json:"role"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	Profile   string    `gorm:"type:text" json:"profile"`
	Image     *string   `gorm:"size:500" json:"image,omitempty"`
	Posts     []Post    `gorm:"foreignKey:AuthorID" json:"posts,omitempty" swaggerignore:"true"`
	CreatedAt time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt time.Time `json:"updated_at" swaggerignore:"true"`
}

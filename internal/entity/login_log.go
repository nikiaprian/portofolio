package entity

import "time"

// LoginLog records a sign-in attempt (success or failure) for admin audit.
type LoginLog struct {
	ID            uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	UserID        *uint     `gorm:"index" json:"user_id,omitempty"`
	Email         string    `gorm:"size:255;index;not null" json:"email"`
	Success       bool      `gorm:"not null;default:false;index" json:"success"`
	FailureReason string    `gorm:"size:500" json:"failure_reason,omitempty"`
	IP            string    `gorm:"size:64" json:"ip,omitempty"`
	UserAgent     string    `gorm:"type:text" json:"user_agent,omitempty"`
	CreatedAt     time.Time `gorm:"index" json:"created_at" swaggerignore:"true"`
}

// TableName maps LoginLog to login_logs.
func (LoginLog) TableName() string {
	return "login_logs"
}

package entity

import "time"

// ManualPostAILog records AI generate actions from manual post form
// (generate draft/content and generate image).
type ManualPostAILog struct {
	ID            uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	UserID        uint      `gorm:"index;not null" json:"user_id"`
	Level         string    `gorm:"size:16;not null;default:info;index" json:"level"`
	Event         string    `gorm:"size:32;index" json:"event"`
	Message       string    `gorm:"type:text;not null" json:"message"`
	PostID        *uint     `gorm:"index" json:"post_id,omitempty"`
	Title         string    `gorm:"size:220" json:"title,omitempty"`
	ProviderUsed  string    `gorm:"size:16;index" json:"provider_used,omitempty"`
	ProviderInput string    `gorm:"size:16;index" json:"provider_input,omitempty"`
	CreatedAt     time.Time `gorm:"index" json:"created_at" swaggerignore:"true"`
	User          User      `gorm:"foreignKey:UserID;references:ID" json:"-" swaggerignore:"true"`
}

// TableName maps ManualPostAILog to manual_post_ai_logs.
func (ManualPostAILog) TableName() string {
	return "manual_post_ai_logs"
}

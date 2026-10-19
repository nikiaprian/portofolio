package entity

import "time"

// PostView records one viewer (IP) visiting a post on a calendar day (UTC).
// Composite uniqueness is enforced for "unique IP per post per day".
type PostView struct {
	ID        uint      `gorm:"primaryKey" json:"-" swaggerignore:"true"`
	PostID    uint      `gorm:"not null;uniqueIndex:ux_post_view" json:"-" swaggerignore:"true"`
	Post      Post      `gorm:"foreignKey:PostID;references:ID" json:"-" swaggerignore:"true"`
	ViewerIP  string    `gorm:"size:45;not null;uniqueIndex:ux_post_view" json:"-" swaggerignore:"true"`
	ViewDate  string    `gorm:"size:10;not null;uniqueIndex:ux_post_view" json:"-" swaggerignore:"true"` // YYYY-MM-DD (UTC)
	CreatedAt time.Time `json:"-" swaggerignore:"true"`
}

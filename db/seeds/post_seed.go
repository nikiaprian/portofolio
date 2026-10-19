package seeds

import (
	"errors"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedPosts(db *gorm.DB, admin *entity.User, categories []entity.Category, tags []entity.Tag) error {
	defaults := []entity.Post{
		{
			Title:      "Post 1",
			Slug:       "post-1",
			Content:    "Content of post 1",
			Status:     "published",
			AuthorID:   admin.ID,
			CategoryID: categories[0].ID,
			Tags:       tags,
		},
		{
			Title:      "Post 2",
			Slug:       "post-2",
			Content:    "Content of post 2",
			Status:     "published",
			AuthorID:   admin.ID,
			CategoryID: categories[0].ID,
			Tags:       []entity.Tag{tags[0], tags[4]},
		},
		{
			Title:      "Post 3",
			Slug:       "post-3",
			Content:    "Content of post 3",
			Status:     "draft",
			AuthorID:   admin.ID,
			CategoryID: categories[3].ID,
			Tags:       []entity.Tag{tags[4]},
		},
	}

	for _, def := range defaults {
		var existing entity.Post
		if err := db.Where("slug = ?", def.Slug).First(&existing).Error; err == nil {
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		post := def
		if err := db.Create(&post).Error; err != nil {
			return err
		}
	}

	return nil
}

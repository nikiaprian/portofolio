package seeds

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedCategories(db *gorm.DB) ([]entity.Category, error) {
	defaults := []entity.Category{
		{Sort: 1, Name: "Politik", Slug: "politik"},
		{Sort: 2, Name: "Ekonomi", Slug: "ekonomi"},
		{Sort: 3, Name: "Teknologi", Slug: "teknologi"},
		{Sort: 4, Name: "Olahraga", Slug: "olahraga"},
		{Sort: 5, Name: "Hiburan", Slug: "hiburan"},
		{Sort: 6, Name: "Pendidikan", Slug: "pendidikan"},
		{Sort: 7, Name: "Kesehatan", Slug: "kesehatan"},
		{Sort: 8, Name: "Internasional", Slug: "internasional"},
		{Sort: 9, Name: "Nasional", Slug: "nasional"},
		{Sort: 10, Name: "Gaya Hidup", Slug: "gaya-hidup"},
	}

	categories := make([]entity.Category, 0, len(defaults))
	for _, def := range defaults {
		cat := def
		if err := db.Where("slug = ?", def.Slug).FirstOrCreate(&cat).Error; err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}

	return categories, nil
}

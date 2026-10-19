package seeds

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedTags(db *gorm.DB) ([]entity.Tag, error) {
	defaults := []entity.Tag{
		{Name: "AI", Slug: "ai"},
		{Name: "Startup", Slug: "startup"},
		{Name: "Smartphone", Slug: "smartphone"},
		{Name: "Internet", Slug: "internet"},
		{Name: "Keamanan Data", Slug: "keamanan-data"},
		{Name: "Pemilu 2024", Slug: "pemilu-2024"},
		{Name: "Kebijakan Publik", Slug: "kebijakan-publik"},
		{Name: "Sepak Bola", Slug: "sepak-bola"},
		{Name: "Liga Inggris", Slug: "liga-inggris"},
		{Name: "Piala Dunia", Slug: "piala-dunia"},
		{Name: "Kesehatan Mental", Slug: "kesehatan-mental"},
		{Name: "Gaya Hidup", Slug: "gaya-hidup"},
	}

	tags := make([]entity.Tag, 0, len(defaults))
	for _, def := range defaults {
		tag := def
		if err := db.Where("slug = ?", def.Slug).FirstOrCreate(&tag).Error; err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

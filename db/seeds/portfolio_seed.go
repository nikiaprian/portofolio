package seeds

import (
	"errors"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// EnsureDemoPortfolio inserts one active sample site so the sales homepage can link to it.
// Existing rows are left unchanged.
func EnsureDemoPortfolio(db *gorm.DB) error {
	var existing entity.Portfolio
	err := db.Where("subdomain = ?", "contoh").First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	site := entity.Portfolio{
		Subdomain:   "contoh",
		Status:      entity.PortfolioActive,
		DisplayName: "Contoh Kreator",
		Headline:    "Desainer yang membuat produk digital terasa jelas",
		Bio:         "Ini contoh portfolio yang aktif setelah pembayaran. Pembeli mengisi foto, bio, keahlian, proyek, dan kontak dari halaman kelola.",
		Email:       "halo@contoh.test",
		WhatsApp:    "+6281234567890",
		Links: entity.PortfolioLinks{
			Website:  "https://example.com",
			LinkedIn: "https://www.linkedin.com",
		},
		Skills: []entity.PortfolioSkill{
			{Name: "Riset pengguna"},
			{Name: "Antarmuka"},
			{Name: "Prototipe"},
		},
		Projects: []entity.PortfolioProject{
			{Title: "Situs jurnal baca", Summary: "Membuat alur baca harian untuk orang yang baru belajar menata waktu.", URL: "https://example.com"},
			{Title: "Halaman jual produk", Summary: "Menyusun cerita produk, harga, dan ajakan beli dalam satu halaman.", URL: "https://example.com"},
		},
	}
	return db.Create(&site).Error
}

package portfolio

import "strings"

// Template is one portfolio layout a buyer purchases and keeps.
type Template struct {
	Slug    string
	Name    string
	Summary string
	Detail  string
	Tone    string
}

// Templates is the product catalog. The homepage shows the first six.
func Templates() []Template {
	return []Template{
		{
			Slug:    "klasik",
			Name:    "Klasik",
			Summary: "Foto, nama, dan karya dalam susunan yang tenang.",
			Detail:  "Cocok kalau halaman ingin langsung terbaca: profil di atas, lalu keahlian dan proyek.",
			Tone:    "lilac",
		},
		{
			Slug:    "studio",
			Name:    "Studio",
			Summary: "Profil menonjol dengan latar yang lebih dalam.",
			Detail:  "Nama dan jabatan duduk besar di atas, karya mengikuti di bawahnya.",
			Tone:    "ink",
		},
		{
			Slug:    "editorial",
			Name:    "Editorial",
			Summary: "Judul besar untuk tulisan dan proyek.",
			Detail:  "Terasa seperti halaman majalah: satu kalimat pembuka, lalu daftar karya.",
			Tone:    "paper",
		},
		{
			Slug:    "galeri",
			Name:    "Galeri",
			Summary: "Karya ditampilkan sebagai kartu yang bisa dibuka.",
			Detail:  "Foto dan proyek menjadi bagian utama, profil tetap ada di atas.",
			Tone:    "blush",
		},
		{
			Slug:    "lamaran",
			Name:    "Lamaran",
			Summary: "Ringkas, enak dilampirkan saat melamar.",
			Detail:  "Pendidikan, keahlian, dan kontak tersusun rapat dalam satu layar.",
			Tone:    "moss",
		},
		{
			Slug:    "kreator",
			Name:    "Kreator",
			Summary: "Keahlian dan tautan sosial di depan.",
			Detail:  "Chip keahlian dan tombol kontak mudah dilihat dari bio media sosial.",
			Tone:    "amber",
		},
	}
}

// HomeTemplates returns at most six templates for the homepage.
func HomeTemplates() []Template {
	all := Templates()
	if len(all) > 6 {
		return all[:6]
	}
	return all
}

// TemplateBySlug finds one catalog item.
func TemplateBySlug(slug string) (Template, bool) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	for _, item := range Templates() {
		if item.Slug == slug {
			return item, true
		}
	}
	return Template{}, false
}

// NormalizeTemplate returns a known slug, or empty when the choice is not for sale.
func NormalizeTemplate(slug string) string {
	item, ok := TemplateBySlug(slug)
	if !ok {
		return ""
	}
	return item.Slug
}

// ThemeClass is the body class for a published portfolio page.
func ThemeClass(slug string) string {
	switch NormalizeTemplate(slug) {
	case "studio":
		return "pf-body-studio"
	case "editorial":
		return "pf-body-editorial"
	case "galeri":
		return "pf-body-galeri"
	case "lamaran":
		return "pf-body-lamaran"
	case "kreator":
		return "pf-body-kreator"
	default:
		return "pf-body-klasik"
	}
}

// SamplePortfolio is the preview shown on a product page before someone buys it.
func SamplePortfolio(slug string) map[string]any {
	if NormalizeTemplate(slug) == "" {
		slug = "klasik"
	} else {
		slug = NormalizeTemplate(slug)
	}
	return map[string]any{
		"Template":    slug,
		"DisplayName": "Nama Anda",
		"Headline":    "Desainer produk",
		"Bio":         "Membuat produk yang enak dipakai, dari riset sampai tampilan.",
		"PhotoURL":    "/static/img/preview-avatar.jpg",
		"Initials":    "NA",
		"Email":       "nama@domain.id",
		"WhatsAppURL": "https://wa.me/6281200000000",
		"Skills": []struct{ Name string }{
			{"Riset"},
			{"Antarmuka"},
			{"Prototipe"},
		},
		"Projects": []struct{ Title, Summary, URL string }{
			{"Situs jurnal", "Arsip tulisan yang rapi", "https://example.com"},
			{"Aplikasi catatan", "Catatan yang tinggal diklik", "https://example.com"},
			{"Identitas merek", "Warna dan logo singkat", "https://example.com"},
		},
		"Website":   "https://example.com",
		"Instagram": "https://instagram.com",
		"LinkedIn":  "https://linkedin.com",
		"GitHub":    "https://github.com",
	}
}

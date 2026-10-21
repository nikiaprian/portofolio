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

type sampleSkill struct{ Name string }
type sampleProject struct{ Title, Summary, URL string }

// SamplePortfolio is the preview shown on a product page before someone buys it.
func SamplePortfolio(slug string) map[string]any {
	if NormalizeTemplate(slug) == "" {
		slug = "klasik"
	} else {
		slug = NormalizeTemplate(slug)
	}
	page := map[string]any{
		"Template":    slug,
		"PhotoURL":    "/static/img/preview-avatar.jpg",
		"WhatsAppURL": "https://wa.me/6281200000000",
		"Website":     "https://example.com",
		"Instagram":   "https://instagram.com",
		"LinkedIn":    "https://linkedin.com",
		"GitHub":      "https://github.com",
	}
	switch slug {
	case "studio":
		page["DisplayName"] = "Bima Santoso"
		page["Headline"] = "Desainer identitas"
		page["Bio"] = "Membangun wajah merek yang tetap terbaca, dari kartu nama sampai situs."
		page["Initials"] = "BS"
		page["Email"] = "bima@domain.id"
		page["Skills"] = []sampleSkill{{"Identitas"}, {"Tipografi"}, {"Arah seni"}}
		page["Projects"] = []sampleProject{
			{"Identitas kedai", "Warna, huruf, dan tanda yang bisa dipakai ulang.", "https://example.com"},
			{"Buku tahunan", "Sampul dan kisi halaman untuk satu edisi.", "https://example.com"},
			{"Pameran kecil", "Ruang, poster, dan urutan baca.", "https://example.com"},
		}
	case "editorial":
		page["DisplayName"] = "Dimas Arya"
		page["Headline"] = "Penulis dan editor"
		page["Bio"] = "Menyusun cerita yang cukup untuk dibaca sekali duduk, dan cukup jelas untuk dikutip."
		page["Initials"] = "DA"
		page["Email"] = "dimas@domain.id"
		page["Skills"] = []sampleSkill{{"Laporan"}, {"Suntingan"}, {"Wawancara"}}
		page["Projects"] = []sampleProject{
			{"Catatan kota", "Serangkaian esai tentang jalan dan warung.", "https://example.com"},
			{"Wawancara kerja", "Percakapan dengan orang yang baru pindah bidang.", "https://example.com"},
			{"Panduan singkat", "Cara menulis lamaran tanpa kalimat yang hampa.", "https://example.com"},
		}
	case "galeri":
		page["DisplayName"] = "Lala Kusuma"
		page["Headline"] = "Fotografer"
		page["Bio"] = "Memotret orang dan ruang dengan cahaya yang tidak dibuat-buat."
		page["Initials"] = "LK"
		page["Email"] = "lala@domain.id"
		page["Skills"] = []sampleSkill{{"Potret"}, {"Ruang"}, {"Suntingan"}}
		page["Projects"] = []sampleProject{
			{"Potret keluarga", "Sesi di rumah, cahaya dari jendela.", "https://example.com"},
			{"Dapur pagi", "Seri benda dan tangan yang sedang bekerja.", "https://example.com"},
			{"Pameran kampus", "Dua puluh foto dalam satu lorong.", "https://example.com"},
			{"Catatan jalan", "Foto singkat dari perjalanan harian.", "https://example.com"},
		}
	case "lamaran":
		page["DisplayName"] = "Andi Pratama"
		page["Headline"] = "Analis data"
		page["Bio"] = "Mengubah angka yang berserakan menjadi catatan yang bisa dipakai tim untuk memutuskan."
		page["Initials"] = "AP"
		page["Email"] = "andi@domain.id"
		page["Skills"] = []sampleSkill{{"SQL"}, {"Visualisasi"}, {"Ringkasan"}}
		page["Projects"] = []sampleProject{
			{"Dasbor penjualan", "Angka mingguan yang bisa dibaca dalam satu layar.", "https://example.com"},
			{"Laporan musim", "Ringkasan tiga bulan untuk rapat pimpinan.", "https://example.com"},
			{"Pembersihan data", "Nama ganda dan tanggal yang tidak konsisten.", "https://example.com"},
		}
	case "kreator":
		page["DisplayName"] = "Rina Putri"
		page["Headline"] = "Kreator resep"
		page["Bio"] = "Berbagi resep singkat dan catatan dapur yang bisa diikuti tanpa peralatan khusus."
		page["Initials"] = "RP"
		page["Email"] = "rina@domain.id"
		page["Skills"] = []sampleSkill{{"Resep"}, {"Foto makanan"}, {"Cerita"}}
		page["Projects"] = []sampleProject{
			{"Menu seminggu", "Tujuh masakan dari belanja satu kali.", "https://example.com"},
			{"Catatan bumbu", "Takaran yang dipakai berulang.", "https://example.com"},
			{"Video singkat", "Satu langkah yang sering keliru.", "https://example.com"},
		}
	default:
		page["DisplayName"] = "Sari Wulandari"
		page["Headline"] = "Desainer produk"
		page["Bio"] = "Merancang alur yang tenang, dari riset pengguna sampai tampilan yang enak dipakai sehari-hari."
		page["Initials"] = "SW"
		page["Email"] = "sari@domain.id"
		page["Skills"] = []sampleSkill{{"Riset"}, {"Antarmuka"}, {"Prototipe"}, {"Menulis"}}
		page["Projects"] = []sampleProject{
			{"Situs jurnal", "Arsip tulisan yang mudah dijelajahi.", "https://example.com"},
			{"Aplikasi catatan", "Catatan yang tinggal diketik lalu dikirim.", "https://example.com"},
			{"Peta layanan", "Alur pendaftaran yang lebih pendek.", "https://example.com"},
		}
	}
	return page
}

package portfolio

// Template is one ready-made portfolio layout a buyer can start from.
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
	for _, item := range Templates() {
		if item.Slug == slug {
			return item, true
		}
	}
	return Template{}, false
}

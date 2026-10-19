package seeds

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedPages(db *gorm.DB) error {
	pages := []entity.Page{
		{
			Title: "Kebijakan Privasi",
			Slug:  "kebijakan-privasi",
			Content: `<p>Kami menghargai privasi pengunjung situs ini. Halaman ini menjelaskan bagaimana data dikumpulkan, digunakan, dan dilindungi.</p>
					<h2>Informasi yang Kami Kumpulkan</h2>
					<ul>
					<li>Data teknis seperti alamat IP, jenis browser, dan halaman yang diakses.</li>
					<li>Data yang Anda kirimkan melalui formulir, seperti nama dan email.</li>
					</ul>
					<h2>Penggunaan Informasi</h2>
					<p>Informasi digunakan untuk meningkatkan layanan, merespons pertanyaan, dan menganalisis performa konten.</p>
					<h2>Cookie</h2>
					<p>Situs ini dapat menggunakan cookie untuk pengalaman pengguna yang lebih baik. Anda bisa menonaktifkan cookie melalui pengaturan browser.</p>
					<h2>Kontak</h2>
					<p>Jika ada pertanyaan terkait privasi, silakan hubungi kami melalui halaman kontak.</p>`,
			Status: "published",
		},
		{
			Title: "Syarat dan Ketentuan",
			Slug:  "syarat-ketentuan",
			Content: `<p>Dengan mengakses situs ini, Anda setuju untuk mematuhi syarat dan ketentuan berikut.</p>
					<h2>Penggunaan Konten</h2>
					<p>Seluruh konten disediakan untuk informasi umum. Dilarang menyalin ulang konten tanpa izin tertulis.</p>
					<h2>Tanggung Jawab Pengguna</h2>
					<ul>
					<li>Tidak menyalahgunakan situs untuk aktivitas melanggar hukum.</li>
					<li>Tidak menyebarkan spam, malware, atau konten berbahaya.</li>
					</ul>
					<h2>Perubahan Layanan</h2>
					<p>Kami berhak mengubah, menangguhkan, atau menghentikan layanan kapan saja tanpa pemberitahuan sebelumnya.</p>
					<h2>Batasan Tanggung Jawab</h2>
					<p>Kami tidak bertanggung jawab atas kerugian langsung maupun tidak langsung akibat penggunaan situs.</p>`,
			Status: "published",
		},
		{
			Title: "Tentang Kami",
			Slug:  "tentang-kami",
			Content: `<p>Kami adalah tim editorial yang berfokus menyajikan artikel informatif, relevan, dan mudah dipahami.</p>
					<h2>Misi</h2>
					<p>Menyediakan konten yang cepat, akurat, dan bermanfaat bagi pembaca.</p>
					<h2>Nilai Kami</h2>
					<ul>
					<li>Integritas dalam publikasi.</li>
					<li>Konsistensi kualitas konten.</li>
					<li>Perbaikan berkelanjutan berdasarkan masukan pembaca.</li>
					</ul>`,
			Status: "published",
		},
		{
			Title: "Kontak",
			Slug:  "kontak",
			Content: `<p>Kami senang menerima masukan, pertanyaan, dan kerja sama.</p>
					<h2>Hubungi Kami</h2>
					<p>Email: <a href="mailto:halo@example.com">halo@example.com</a></p>
					<p>Jam operasional: Senin - Jumat, 09.00 - 17.00 WIB.</p>`,
			Status: "published",
		},
		{
			Title: "Disclaimer",
			Slug:  "disclaimer",
			Content: `<p>Informasi di situs ini disediakan sebagaimana adanya untuk tujuan informasi umum.</p>
					<h2>Akurasi Informasi</h2>
					<p>Kami berupaya menjaga akurasi, namun tidak menjamin konten selalu bebas dari kesalahan atau keterlambatan pembaruan.</p>
					<h2>Tautan Eksternal</h2>
					<p>Situs dapat memuat tautan ke pihak ketiga. Kami tidak bertanggung jawab atas konten pada situs eksternal tersebut.</p>`,
			Status: "published",
		},
	}
	return db.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"title",
				"content",
				"status",
				"updated_at",
			}),
		}).
		Create(&pages).Error
}

package seeds

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func seedPages(db *gorm.DB) error {
	pages := []entity.Page{
		{
			Title: "Kebijakan Privasi",
			Slug:  "kebijakan-privasi",
			Content: `<p>Tirtayasa (tirtayasa.id) menghargai privasi pengunjung. Halaman ini menjelaskan data yang dikumpulkan, termasuk cookie iklan pihak ketiga.</p>
					<h2>Informasi yang Kami Kumpulkan</h2>
					<ul>
					<li>Data teknis seperti alamat IP, jenis browser, dan halaman yang diakses.</li>
					<li>Data yang Anda kirimkan melalui formulir, seperti nama dan email.</li>
					</ul>
					<h2>Cookie dan iklan</h2>
					<p>Situs ini dapat memakai cookie untuk mengingat preferensi dan mengukur kunjungan. Jika iklan tampil, mitra iklan seperti Google AdSense dapat menempatkan cookie untuk menayangkan dan mengukur iklan. Anda dapat menolak cookie iklan yang dipersonalisasi lewat pengaturan iklan Google dan pengaturan browser.</p>
					<h2>Penggunaan Informasi</h2>
					<p>Informasi digunakan untuk menjalankan situs, menjawab pertanyaan, dan memahami bacaan mana yang berguna.</p>
					<h2>Kontak</h2>
					<p>Pertanyaan privasi: <a href="mailto:redaksi@tirtayasa.id">redaksi@tirtayasa.id</a>.</p>`,
			Status: "published",
		},
		{
			Title: "Syarat dan Ketentuan",
			Slug:  "syarat-ketentuan",
			Content: `<p>Dengan mengakses tirtayasa.id, Anda setuju pada syarat berikut dari redaksi Tirtayasa.</p>
					<h2>Penggunaan Konten</h2>
					<p>Artikel disediakan untuk dibaca. Menyalin ulang tulisan Tirtayasa tanpa izin tertulis tidak diperbolehkan.</p>
					<h2>Tanggung Jawab Pengguna</h2>
					<ul>
					<li>Tidak menyalahgunakan situs untuk aktivitas yang melanggar hukum.</li>
					<li>Tidak menyebarkan spam, malware, atau konten berbahaya.</li>
					</ul>
					<h2>Perubahan Layanan</h2>
					<p>Redaksi dapat mengubah atau menghentikan bagian layanan sewaktu-waktu.</p>
					<h2>Batasan Tanggung Jawab</h2>
					<p>Tirtayasa tidak bertanggung jawab atas kerugian yang timbul dari penggunaan informasi di situs ini.</p>`,
			Status: "published",
		},
		{
			Title: "Tentang Kami",
			Slug:  "tentang-kami",
			Content: `<p>Tirtayasa adalah redaksi di tirtayasa.id. Kami menulis dan menyunting artikel sendiri agar pembaca mendapat bacaan yang jelas, bukan salinan dari situs lain.</p>
					<h2>Misi</h2>
					<p>Menyajikan tulisan yang bermanfaat, mudah diikuti, dan bisa dipertanggungjawabkan.</p>
					<h2>Nilai Kami</h2>
					<ul>
					<li>Artikel orisinal atas nama redaksi Tirtayasa.</li>
					<li>Koreksi jika ada kekeliruan fakta.</li>
					<li>Kontak yang bisa dihubungi pembaca.</li>
					</ul>`,
			Status: "published",
		},
		{
			Title: "Kontak",
			Slug:  "kontak",
			Content: `<p>Redaksi Tirtayasa menerima masukan, pertanyaan, dan kerja sama melalui email.</p>
					<h2>Hubungi Kami</h2>
					<p>Email: <a href="mailto:redaksi@tirtayasa.id">redaksi@tirtayasa.id</a></p>
					<p>Situs: <a href="https://tirtayasa.id">tirtayasa.id</a></p>
					<p>Jam operasional: Senin–Jumat, 09.00–17.00 WIB.</p>`,
			Status: "published",
		},
		{
			Title: "Disclaimer",
			Slug:  "disclaimer",
			Content: `<p>Tulisan di tirtayasa.id disediakan untuk informasi umum oleh redaksi Tirtayasa.</p>
					<h2>Akurasi Informasi</h2>
					<p>Kami berusaha menjaga ketepatan fakta, tetapi tidak menjamin setiap halaman selalu bebas dari kesalahan atau keterlambatan pembaruan.</p>
					<h2>Tautan Eksternal</h2>
					<p>Situs dapat menautkan sumber lain. Tirtayasa tidak mengendalikan isi situs pihak ketiga tersebut.</p>
					<h2>Iklan</h2>
					<p>Ruang iklan, bila ada, ditayangkan oleh mitra pihak ketiga dan tidak selalu mencerminkan pandangan redaksi.</p>`,
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

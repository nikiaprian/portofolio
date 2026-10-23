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
			Content: `<p>Tirtayasa (tirtayasa.id) menjelaskan di halaman ini data apa yang terkumpul saat Anda membaca panduan layanan warga, termasuk cookie iklan pihak ketiga.</p>
					<h2>Informasi yang kami kumpulkan</h2>
					<ul>
					<li>Data teknis seperti alamat IP, jenis browser, dan halaman yang dibuka.</li>
					<li>Pesan yang Anda kirim ke redaksi, seperti nama dan alamat email.</li>
					</ul>
					<p>Situs ini tidak meminta NIK, kata sandi bank, PIN, atau OTP. Jangan mengirim data itu lewat email.</p>
					<h2>Cookie dan iklan</h2>
					<p>Situs dapat memakai cookie untuk mengingat preferensi dan mengukur kunjungan. Jika iklan tampil, mitra iklan seperti Google AdSense dapat menempatkan cookie untuk menayangkan dan mengukur iklan. Anda dapat menolak cookie iklan yang dipersonalisasi lewat pengaturan iklan Google dan pengaturan browser.</p>
					<h2>Penggunaan informasi</h2>
					<p>Informasi dipakai untuk menjalankan situs, membalas pertanyaan, dan memperbaiki panduan yang keliru.</p>
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
			Content: `<p>Tirtayasa adalah redaksi di tirtayasa.id. Kami menulis panduan mengurus layanan warga lewat jalur pemerintah: bansos dan DTSEN, BPJS, SKCK, NISN, serta urusan kerja. Setiap panduan menyebut berkas, situs resmi, dan hal yang tidak mempercepat proses.</p>
					<h2>Bukan calo</h2>
					<p>Tirtayasa bukan instansi pemerintah, bukan bank, dan tidak mengurus pencairan bantuan. Kami tidak menjanjikan dana cair, tidak meminta biaya, dan tidak meminta OTP.</p>
					<h2>Redaksi</h2>
					<p>Naskah disunting redaksi Tirtayasa. Pertanyaan dan koreksi dibalas di <a href="mailto:redaksi@tirtayasa.id">redaksi@tirtayasa.id</a>.</p>
					<p>Jika aturan berubah, kami memperbaiki halaman dan mencatat tanggal pemeriksaan. Lihat juga <a href="/kebijakan-koreksi">kebijakan koreksi</a>.</p>`,
			Status: "published",
		},
		{
			Title: "Kontak",
			Slug:  "kontak",
			Content: `<p>Redaksi membalas email yang masuk ke alamat di bawah. Pesan tentang kekeliruan fakta, tautan resmi yang mati, atau pertanyaan pembaca ditanggapi pada hari kerja.</p>
					<h2>Email redaksi</h2>
					<p><a href="mailto:redaksi@tirtayasa.id">redaksi@tirtayasa.id</a></p>
					<p>Situs: <a href="https://tirtayasa.id">tirtayasa.id</a></p>
					<p>Jam balasan: Senin–Jumat, 09.00–17.00 WIB.</p>
					<p>Sertakan tautan halaman dan kalimat yang ingin diperbaiki. Jangan kirim NIK, PIN, OTP, atau kata sandi.</p>`,
			Status: "published",
		},
		{
			Title: "Disclaimer",
			Slug:  "disclaimer",
			Content: `<p>Tulisan di tirtayasa.id adalah panduan umum dari redaksi Tirtayasa, bukan keputusan instansi.</p>
					<h2>Akurasi informasi</h2>
					<p>Kami menautkan situs pemerintah dan mencatat tanggal pemeriksaan. Aturan bisa berubah. Status bansos, iuran BPJS, atau data NISN hanya sah di layanan resmi, bukan di halaman ini.</p>
					<h2>Tautan eksternal</h2>
					<p>Situs dapat menautkan domain pemerintah dan sumber lain. Tirtayasa tidak mengendalikan isi situs pihak ketiga.</p>
					<h2>Iklan</h2>
					<p>Ruang iklan, bila ada, ditayangkan oleh mitra pihak ketiga dan tidak mencerminkan pandangan redaksi. Iklan yang menjanjikan pencairan instan atau meminta OTP bukan bagian dari panduan ini.</p>`,
			Status: "published",
		},
		{
			Title: "Kebijakan Koreksi",
			Slug:  "kebijakan-koreksi",
			Content: `<p>Jika sebuah panduan keliru, redaksi Tirtayasa memperbaikinya. Kirim koreksi ke <a href="mailto:redaksi@tirtayasa.id">redaksi@tirtayasa.id</a>.</p>
					<h2>Yang kami perbaiki</h2>
					<ul>
					<li>Nama lembaga, alamat situs resmi, atau syarat berkas yang sudah berubah.</li>
					<li>Angka, tanggal, atau langkah yang tidak lagi sesuai pengumuman resmi.</li>
					<li>Kalimat yang terdengar seperti janji pencairan atau jasa calo.</li>
					</ul>
					<h2>Cara mengirim</h2>
					<p>Sertakan tautan halaman, kalimat yang salah, dan tautan pengumuman resmi bila ada. Kami membalas pada hari kerja.</p>
					<h2>Setelah diperbaiki</h2>
					<p>Halaman yang diubah mencantumkan tanggal pemeriksaan baru. Koreksi fakta tidak dihapus diam-diam dari catatan redaksi.</p>`,
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

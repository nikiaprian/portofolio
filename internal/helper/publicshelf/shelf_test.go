package publicshelf

import "testing"

func TestMatchesShelfTopics(t *testing.T) {
	cases := []struct {
		title string
		slug  string
		want  bool
	}{
		{"Cara cek bansos di situs Kemensos", "cek-bansos", true},
		{"Surat desa untuk form DTSEN", "form-dtsen", true},
		{"Cek status BPJS Kesehatan", "cek-bpjs", true},
		{"Urus SKCK online", "urus-skck", true},
		{"NISN tidak ditemukan", "cek-nisn", true},
		{"Aktivasi Identitas Kependudukan Digital", "cara-aktivasi-ikd", true},
		{"Cara Cek PIP 2026 Lewat HP", "cara-cek-pip-2026", true},
		{"Cara Cek KIP 2026", "cara-cek-kip-2026", true},
		{"Cek PKH dan BSU", "cek-pkh-bsu", true},
		{"Daftar Prakerja dan CPNS", "daftar-prakerja", true},
		{"Aktivasi IKD di Dukcapil", "aktivasi-dukcapil", true},
		{"Kiper Real Madrid gemilang", "kiper-real-madrid", false},
		{"Kiprah Barcelona musim ini", "kiprah-barcelona", false},
		{"Aplikasi edit video terbaik", "aplikasi-edit-video", false},
		{"Isi perjanjian kerja", "perjanjian-kerja", true},
		{"Pertanyaan wawancara kerja", "wawancara-hrd", true},
		{"Reset PIN BRImo", "reset-pin-brimo", false},
		{"Social spy WhatsApp", "social-spy-whatsapp", false},
		{"Harga gift TikTok", "gift-tiktok", false},
	}
	for _, tc := range cases {
		if got := Matches(tc.title, tc.slug); got != tc.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", tc.title, tc.slug, got, tc.want)
		}
	}
}

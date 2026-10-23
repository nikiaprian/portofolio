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

package publicshelf

import "testing"

func TestActiveIsOffForNewsSite(t *testing.T) {
	for _, layout := range []string{"", "default", "detik", "neo", "tirtayasa", "TIRTAYASA"} {
		if Active(layout) {
			t.Errorf("Active(%q) = true, want false", layout)
		}
	}
}

func TestMatchesIndexesEveryArticle(t *testing.T) {
	cases := []struct {
		title string
		slug  string
	}{
		{"Cara cek bansos di situs Kemensos", "cek-bansos"},
		{"Kiper Real Madrid gemilang", "kiper-real-madrid"},
		{"Harga gift TikTok", "gift-tiktok"},
		{"", ""},
	}
	for _, tc := range cases {
		if !Matches(tc.title, tc.slug) {
			t.Errorf("Matches(%q, %q) = false, want true", tc.title, tc.slug)
		}
	}
}

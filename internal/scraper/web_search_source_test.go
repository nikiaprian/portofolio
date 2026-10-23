package scraper

import "testing"

func TestNormalizeWebSearchSource(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"pexels", "pexels"},
		{" PEXELS ", "pexels"},
		{"", "auto"},
		{"auto", "auto"},
		{"invalid", "auto"},
	}
	for _, tc := range tests {
		if got := NormalizeWebSearchSource(tc.in); got != tc.want {
			t.Errorf("NormalizeWebSearchSource(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

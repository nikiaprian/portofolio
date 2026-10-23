package entity

import "testing"

func TestWebImageProviderAutoOrder(t *testing.T) {
	want := []string{
		WebImageProviderPexels,
		WebImageProviderUnsplash,
		WebImageProviderPixabay,
		WebImageProviderOpenverse,
		WebImageProviderWikimedia,
		WebImageProviderWordPress,
	}
	if len(WebImageProviderAutoOrder) != len(want) {
		t.Fatalf("len(WebImageProviderAutoOrder) = %d, want %d", len(WebImageProviderAutoOrder), len(want))
	}
	for i := range want {
		if WebImageProviderAutoOrder[i] != want[i] {
			t.Errorf("WebImageProviderAutoOrder[%d] = %q, want %q", i, WebImageProviderAutoOrder[i], want[i])
		}
	}
}

package entity

import "testing"

func TestNormalizeSidebarListLimits(t *testing.T) {
	legacyOnly := SiteSetting{SidebarListLimit: 8}
	NormalizeSidebarListLimits(&legacyOnly)
	if legacyOnly.SidebarListLimitPopular != 8 || legacyOnly.SidebarListLimitLatest != 8 {
		t.Fatalf("legacy fallback: popular=%d latest=%d, want 8/8",
			legacyOnly.SidebarListLimitPopular, legacyOnly.SidebarListLimitLatest)
	}

	bothSet := SiteSetting{
		SidebarListLimit:        3,
		SidebarListLimitPopular: 4,
		SidebarListLimitLatest:  6,
	}
	NormalizeSidebarListLimits(&bothSet)
	if bothSet.SidebarListLimitPopular != 4 || bothSet.SidebarListLimitLatest != 6 {
		t.Fatalf("explicit limits: popular=%d latest=%d, want 4/6",
			bothSet.SidebarListLimitPopular, bothSet.SidebarListLimitLatest)
	}

	clamped := SiteSetting{SidebarListLimitPopular: 99, SidebarListLimitLatest: 0, SidebarListLimit: 7}
	NormalizeSidebarListLimits(&clamped)
	if clamped.SidebarListLimitPopular != MaxSidebarListLimit {
		t.Fatalf("clamp popular: got %d want %d", clamped.SidebarListLimitPopular, MaxSidebarListLimit)
	}
	if clamped.SidebarListLimitLatest != 7 {
		t.Fatalf("latest from legacy: got %d want 7", clamped.SidebarListLimitLatest)
	}
}

func TestEffectiveSidebarListLimit(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, DefaultSidebarListLimit},
		{5, 5},
		{20, 20},
		{21, MaxSidebarListLimit},
		{-1, DefaultSidebarListLimit},
	}
	for _, tc := range tests {
		if got := EffectiveSidebarListLimit(tc.in); got != tc.want {
			t.Errorf("EffectiveSidebarListLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

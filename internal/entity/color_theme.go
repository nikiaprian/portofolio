package entity

import "strings"

// Preset tema warna untuk frontend (disimpan di site_settings.color_theme).
const (
	ColorThemeDefault = "default" // aksen merah (default Bebii)
	ColorThemeOcean   = "ocean"   // biru-teal dingin
	ColorThemeForest  = "forest"  // hijau alam
	ColorThemeSunset  = "sunset"  // coral / oranye hangat
	ColorThemeSlate   = "slate"   // abu-biru netral
	ColorThemeViolet  = "violet"  // ungu modern
	ColorThemeAmber   = "amber"   // emas hangat
	ColorThemeDark    = "dark"    // mode gelap (latar gelap, teks terang)
	ColorThemeCustom  = "custom"  // 8 warna diisi manual di admin
)

// SiteColorValues holds the eight theme tokens used in CSS variables.
type SiteColorValues struct {
	TextPrimary         string
	TextSecondary       string
	BackgroundPrimary   string
	BackgroundSecondary string
	PrimaryButton       string
	SecondaryButton     string
	AccentColor         string
	BorderColor         string
}

// NormalizeColorTheme maps stored/requested values to a known preset key.
func NormalizeColorTheme(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ColorThemeOcean, ColorThemeForest, ColorThemeSunset, ColorThemeSlate, ColorThemeViolet, ColorThemeAmber, ColorThemeDark, ColorThemeCustom:
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ColorThemeDefault
	}
}

// SiteColorPreset returns the fixed palette for a non-custom theme.
// For unknown keys, returns the default (merah) palette.
func SiteColorPreset(theme string) SiteColorValues {
	t := NormalizeColorTheme(theme)
	if t == ColorThemeCustom {
		t = ColorThemeDefault
	}
	presets := map[string]SiteColorValues{
		ColorThemeDefault: {
			// Latar putih penuh; teks slate agar kontras dan nyaman dibaca.
			TextPrimary: "#1f2937", TextSecondary: "#64748b",
			BackgroundPrimary: "#ffffff", BackgroundSecondary: "#ffffff",
			PrimaryButton: "#0d6efd", SecondaryButton: "#6c757d",
			AccentColor: "#c62828", BorderColor: "#e5e7eb",
		},
		ColorThemeOcean: {
			TextPrimary: "#0d2137", TextSecondary: "#4a6572",
			BackgroundPrimary: "#f5fbff", BackgroundSecondary: "#e3f2fd",
			PrimaryButton: "#0277bd", SecondaryButton: "#546e7a",
			AccentColor: "#006064", BorderColor: "#b3e5fc",
		},
		ColorThemeForest: {
			TextPrimary: "#1b2e1b", TextSecondary: "#4a6352",
			BackgroundPrimary: "#f7fcf8", BackgroundSecondary: "#e8f5e9",
			PrimaryButton: "#2e7d32", SecondaryButton: "#5d4037",
			AccentColor: "#1b5e20", BorderColor: "#c8e6c9",
		},
		ColorThemeSunset: {
			TextPrimary: "#3e2723", TextSecondary: "#6d4c41",
			BackgroundPrimary: "#fffaf5", BackgroundSecondary: "#fff3e0",
			PrimaryButton: "#ef6c00", SecondaryButton: "#8d6e63",
			AccentColor: "#d84315", BorderColor: "#ffccbc",
		},
		ColorThemeSlate: {
			TextPrimary: "#1e293b", TextSecondary: "#64748b",
			BackgroundPrimary: "#f8fafc", BackgroundSecondary: "#f1f5f9",
			PrimaryButton: "#334155", SecondaryButton: "#64748b",
			AccentColor: "#0f172a", BorderColor: "#cbd5e1",
		},
		ColorThemeViolet: {
			TextPrimary: "#1a1a2e", TextSecondary: "#5c5470",
			BackgroundPrimary: "#faf8ff", BackgroundSecondary: "#f3e8ff",
			PrimaryButton: "#6d28d9", SecondaryButton: "#7c3aed",
			AccentColor: "#5b21b6", BorderColor: "#ddd6fe",
		},
		ColorThemeAmber: {
			TextPrimary: "#292524", TextSecondary: "#78716c",
			BackgroundPrimary: "#fffbeb", BackgroundSecondary: "#fef3c7",
			PrimaryButton: "#d97706", SecondaryButton: "#a8a29e",
			AccentColor: "#b45309", BorderColor: "#fde68a",
		},
		ColorThemeDark: {
			TextPrimary: "#e2e8f0", TextSecondary: "#94a3b8",
			BackgroundPrimary: "#0f172a", BackgroundSecondary: "#1e293b",
			PrimaryButton: "#38bdf8", SecondaryButton: "#64748b",
			AccentColor: "#0026bd", BorderColor: "#334155",
		},
	}
	v, ok := presets[t]
	if !ok {
		return presets[ColorThemeDefault]
	}
	return v
}

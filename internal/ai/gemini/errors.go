package gemini

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// ErrQuotaOrRateLimit is returned when Gemini returns HTTP 429, RESOURCE_EXHAUSTED, or similar.
var ErrQuotaOrRateLimit = errors.New("Batas Google Gemini: kuota atau rate limit tercapai. Tunggu beberapa menit, kurangi frekuensi permintaan, atau periksa kuota di Google AI Studio / Google Cloud Console.")

// IsQuotaOrRateLimited detects quota / rate-limit style failures from the Gemini API.
func IsQuotaOrRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 429:
			return true
		case 503:
			msg := strings.ToLower(apiErr.Message + " " + apiErr.Status)
			if strings.Contains(msg, "overload") || strings.Contains(msg, "unavailable") {
				return true
			}
		case 403:
			msg := strings.ToLower(apiErr.Message)
			if strings.Contains(msg, "quota") || strings.Contains(msg, "billing") || strings.Contains(msg, "exceeded") {
				return true
			}
		}
		msg := strings.ToLower(apiErr.Message + " " + apiErr.Status)
		if strings.Contains(msg, "resource_exhausted") || strings.Contains(msg, "resource exhausted") {
			return true
		}
		if strings.Contains(msg, "rate limit") || strings.Contains(msg, "ratelimit") {
			return true
		}
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "resource_exhausted") || strings.Contains(s, "resource exhausted") {
		return true
	}
	if strings.Contains(s, "429") && (strings.Contains(s, "quota") || strings.Contains(s, "rate") || strings.Contains(s, "limit")) {
		return true
	}
	return false
}

// WrapGenerateError maps known API failures to sentinel errors.
func WrapGenerateError(err error) error {
	if err == nil {
		return nil
	}
	if IsQuotaOrRateLimited(err) {
		return ErrQuotaOrRateLimit
	}
	return fmt.Errorf("gemini generate: %w", err)
}

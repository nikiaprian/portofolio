package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrQuotaOrRateLimit is returned when Anthropic responds with HTTP 429 or 402 (billing / quota
// or sustained high request rate). Use errors.Is to detect it.
var ErrQuotaOrRateLimit = errors.New("Batas Anthropic Claude: kuota habis atau rate limit tercapai. Periksa billing di https://console.anthropic.com/settings/organization atau tunggu dan coba lagi.")

// httpAPIError records a non-success Anthropic HTTP response for classification and messages.
type httpAPIError struct {
	statusCode int
	body       string
}

type apiErrorEnvelope struct {
	Type  string `json:"type"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseAPIErrorMessage(raw []byte) string {
	var env apiErrorEnvelope
	if json.Unmarshal(raw, &env) != nil || env.Error == nil {
		return ""
	}
	return strings.TrimSpace(env.Error.Message)
}

func (e *httpAPIError) Error() string {
	if e == nil {
		return ""
	}
	msg := strings.TrimSpace(parseAPIErrorMessage([]byte(e.body)))
	if msg == "" {
		msg = strings.TrimSpace(e.body)
	}
	if len(msg) > 800 {
		msg = msg[:800] + "…"
	}
	if msg == "" {
		return fmt.Sprintf("claude: API %s", http.StatusText(e.statusCode))
	}
	return fmt.Sprintf("claude: API %d %s: %s", e.statusCode, http.StatusText(e.statusCode), msg)
}

// IsQuotaOrRateLimited reports whether err is an Anthropic HTTP 429/402 or a wrapped quota signal.
func IsQuotaOrRateLimited(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrQuotaOrRateLimit) {
		return true
	}
	var he *httpAPIError
	if errors.As(err, &he) && he != nil {
		if he.statusCode == http.StatusTooManyRequests || he.statusCode == http.StatusPaymentRequired {
			return true
		}
		var env apiErrorEnvelope
		if json.Unmarshal([]byte(he.body), &env) == nil && env.Error != nil {
			t := strings.ToLower(env.Error.Type)
			if t == "rate_limit_error" || t == "overloaded_error" {
				return true
			}
		}
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "rate_limit_error") || strings.Contains(s, "rate_limit") {
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
	return fmt.Errorf("claude generate: %w", err)
}

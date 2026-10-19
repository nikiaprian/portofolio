package opencode

import (
	"errors"
	"fmt"

	oai "github.com/sashabaranov/go-openai"
)

// ErrQuotaOrRateLimit is returned when OpenCode Zen responds with HTTP 429.
var ErrQuotaOrRateLimit = errors.New("Batas OpenCode Zen: rate limit atau kuota tercapai. Buka console OpenCode Zen, tunggu beberapa saat, lalu coba lagi.")

// ErrAllFreeTokensUsed is returned when every free OpenCode local model is rate-limited/exhausted.
var ErrAllFreeTokensUsed = errors.New("Semua kuota model gratis OpenCode sudah habis (all free-model tokens used). Coba lagi nanti atau pilih model lain.")

// IsQuotaOrRateLimited reports whether err is an HTTP 429 from the OpenCode Zen API.
func IsQuotaOrRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var reqErr *oai.RequestError
	if errors.As(err, &reqErr) && (reqErr.HTTPStatusCode == 429 || reqErr.HTTPStatusCode == 402) {
		return true
	}
	var apiErr *oai.APIError
	if errors.As(err, &apiErr) && (apiErr.HTTPStatusCode == 429 || apiErr.HTTPStatusCode == 402) {
		return true
	}
	return false
}

func wrapChatCompletionError(err error) error {
	if err == nil {
		return nil
	}
	if IsQuotaOrRateLimited(err) {
		return fmt.Errorf("%w", ErrQuotaOrRateLimit)
	}
	return fmt.Errorf("opencode chat completion: %w", err)
}

package groq

import (
	"errors"
	"fmt"

	oai "github.com/sashabaranov/go-openai"
)

// ErrQuotaOrRateLimit is returned when Groq responds with HTTP 429.
var ErrQuotaOrRateLimit = errors.New("Batas Groq: rate limit atau kuota tercapai. Buka console Groq, tunggu beberapa saat, lalu coba lagi.")

// IsQuotaOrRateLimited reports whether err is an HTTP 429 from the Groq API.
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
	return fmt.Errorf("groq chat completion: %w", err)
}

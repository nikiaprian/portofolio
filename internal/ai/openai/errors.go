package openai

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	oai "github.com/sashabaranov/go-openai"
)

// ErrQuotaOrRateLimit is returned when OpenAI responds with HTTP 429 (billing/quota
// or sustained high request rate). Use errors.Is to detect it.
var ErrQuotaOrRateLimit = errors.New("Batas OpenAI: kuota habis atau rate limit tercapai. Tambah kredit / periksa billing di https://platform.openai.com/account/billing atau tunggu dan coba lagi.")

// IsQuotaOrRateLimited reports whether err is an OpenAI HTTP 429 from the SDK.
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
	return wrapOpenAIAPIError(err)
}

// wrapOpenAIAPIError normalizes OpenAI HTTP errors for chat, images, and other endpoints.
func wrapOpenAIAPIError(err error) error {
	if err == nil {
		return nil
	}
	if IsQuotaOrRateLimited(err) {
		return fmt.Errorf("%w", ErrQuotaOrRateLimit)
	}
	var reqErr *oai.RequestError
	if errors.As(err, &reqErr) {
		return shortOpenAIRequestError(reqErr)
	}
	return fmt.Errorf("openai API: %w", err)
}

// shortOpenAIRequestError avoids dumping multi‑KB HTML bodies (e.g. Cloudflare 502 pages) into logs and API responses.
func shortOpenAIRequestError(e *oai.RequestError) error {
	if e == nil {
		return errors.New("openai: nil request error")
	}
	code := e.HTTPStatusCode
	statusLine := strings.TrimSpace(e.HTTPStatus)
	if statusLine == "" && code > 0 {
		statusLine = http.StatusText(code)
	}
	if statusLine == "" {
		statusLine = "error"
	}

	body := bytes.TrimSpace(e.Body)
	if len(body) > 0 && body[0] == '<' {
		return fmt.Errorf("openai: HTTP %d %s (upstream returned an HTML error page instead of JSON; usually a temporary gateway/CDN or API outage — retry later or use another provider in auto mode)", code, statusLine)
	}
	if code >= 500 {
		return fmt.Errorf("openai: HTTP %d %s (upstream server error)", code, statusLine)
	}

	msg := ""
	if e.Err != nil {
		msg = strings.TrimSpace(e.Err.Error())
	}
	if len(msg) > 500 {
		msg = msg[:500] + "…"
	}
	if msg == "" {
		return fmt.Errorf("openai: HTTP %d %s", code, statusLine)
	}
	return fmt.Errorf("openai: HTTP %d %s: %s", code, statusLine, msg)
}

package media

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const remoteImageMaxBytes = 8 << 20 // 8 MiB

var remoteImageClient = &http.Client{
	Timeout: 45 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// IsExternalImageURL reports whether src is an http(s) URL we may fetch and resize.
func IsExternalImageURL(src string) bool {
	s := strings.TrimSpace(StripResizePrefixes(src))
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u == nil || u.Host == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return remoteHostAllowed(u.Hostname())
}

// FetchRemoteImageBytes downloads an external image after SSRF checks.
func FetchRemoteImageBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !IsExternalImageURL(rawURL) {
		return nil, "", fmt.Errorf("unsupported remote image url")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", err
	}
	if err := assertRemoteHostResolvable(ctx, u.Hostname()); err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bebii-blog-image-proxy/1.0)")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	res, err := remoteImageClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, "", fmt.Errorf("remote image status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, remoteImageMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("empty remote image body")
	}
	if len(body) > remoteImageMaxBytes {
		return nil, "", fmt.Errorf("remote image too large")
	}

	ct := strings.TrimSpace(res.Header.Get("Content-Type"))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "" {
		ct = http.DetectContentType(body)
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		return nil, "", fmt.Errorf("remote response is not an image")
	}
	return body, ct, nil
}

// RemoteResizeCacheKey builds a cache key for a remote URL and resize params.
func RemoteResizeCacheKey(imageURL string, maxW, maxH, qual int) string {
	return ResizeCacheKey(imageURL, 0, 0, maxW, maxH, qual)
}

func remoteHostAllowed(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return false
	}
	switch h {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1", "metadata.google.internal":
		return false
	}
	if strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return false
	}
	return true
}

func assertRemoteHostResolvable(ctx context.Context, host string) error {
	if !remoteHostAllowed(host) {
		return fmt.Errorf("remote host not allowed")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("remote host lookup failed")
	}
	if len(ips) == 0 {
		return fmt.Errorf("remote host has no addresses")
	}
	for _, ia := range ips {
		if ipBlocked(ia.IP) {
			return fmt.Errorf("remote host not allowed")
		}
	}
	return nil
}

func ipBlocked(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	return false
}

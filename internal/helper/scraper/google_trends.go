package scraputil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	reAFInitData  = regexp.MustCompile(`(?s)AF_initDataCallback\(\{key:\s*'(?:ds:0|ds:1|ds:2)'.*?data:(.*?), sideChannel:`)
	reDataKeyword = regexp.MustCompile(`data-keyword="([^"]+)"`)
)

// FetchGoogleTrendsTitles fetches trending search queries from Google Trends URL or RSS fallback.
func FetchGoogleTrendsTitles(client *http.Client, targetURL string) ([]string, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		targetURL = "https://trends.google.co.id/trending?geo=ID&hours=4"
	}

	u, err := ValidateFeedURL(targetURL)
	if err != nil {
		return nil, err
	}

	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("Google Trends HTTP status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	// 1. Try XML parsing directly (if user entered an RSS URL)
	if xmlTitles, err := ExtractFromBytes(body); err == nil && len(xmlTitles) > 0 {
		return xmlTitles, nil
	}

	// 2. Try HTML AF_initDataCallback parsing
	if titles := extractGoogleTrendsFromHTML(body); len(titles) > 0 {
		return titles, nil
	}

	// 3. Try data-keyword attribute regex matching
	if titles := extractDataKeywordFromHTML(body); len(titles) > 0 {
		return titles, nil
	}

	// 4. Try RSS feed fallback (both google.co.id and google.com)
	geo := strings.TrimSpace(u.Query().Get("geo"))
	if geo == "" {
		geo = "ID"
	}

	rssURLs := []string{
		fmt.Sprintf("https://trends.google.co.id/trending/rss?geo=%s", geo),
		fmt.Sprintf("https://trends.google.com/trending/rss?geo=%s", geo),
	}

	for _, rssURL := range rssURLs {
		if rssTitles, err := FetchAndExtract(client, rssURL); err == nil && len(rssTitles) > 0 {
			return rssTitles, nil
		}
	}

	return nil, fmt.Errorf("tidak dapat mengambil keyword dari Google Trends")
}

func extractGoogleTrendsFromHTML(body []byte) []string {
	matches := reAFInitData.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var titles []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		var rawData []interface{}
		if err := json.Unmarshal(match[1], &rawData); err != nil {
			continue
		}

		if len(rawData) < 2 {
			continue
		}

		items, ok := rawData[1].([]interface{})
		if !ok {
			continue
		}

		for _, it := range items {
			arr, ok := it.([]interface{})
			if !ok || len(arr) == 0 {
				continue
			}
			kw, ok := arr[0].(string)
			if !ok {
				continue
			}
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			key := strings.ToLower(kw)
			if !seen[key] {
				seen[key] = true
				titles = append(titles, kw)
			}
		}
	}

	return titles
}

func extractDataKeywordFromHTML(body []byte) []string {
	matches := reDataKeyword.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var titles []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		kw := strings.TrimSpace(string(match[1]))
		if kw == "" {
			continue
		}
		key := strings.ToLower(kw)
		if !seen[key] {
			seen[key] = true
			titles = append(titles, kw)
		}
	}

	return titles
}

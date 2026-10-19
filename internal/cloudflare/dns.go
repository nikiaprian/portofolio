package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the Cloudflare DNS API for one zone.
type Client struct {
	token    string
	zoneID   string
	kind     string
	content  string
	proxied  bool
	baseURL  string
	http     *http.Client
}

func New(token, zoneID, recordType, content string, proxied bool) *Client {
	kind := strings.ToUpper(strings.TrimSpace(recordType))
	if kind != "A" && kind != "CNAME" {
		kind = "CNAME"
	}
	return &Client{
		token:   strings.TrimSpace(token),
		zoneID:  strings.TrimSpace(zoneID),
		kind:    kind,
		content: strings.TrimSpace(content),
		proxied: proxied,
		baseURL: "https://api.cloudflare.com/client/v4",
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBase overrides the API origin. Tests use this.
func (c *Client) WithBase(base string) *Client {
	if c == nil {
		return nil
	}
	c.baseURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return c
}

func (c *Client) ready() bool {
	return c != nil && c.token != "" && c.zoneID != "" && c.content != ""
}

// Exists reports whether a DNS record for name is already in the zone.
func (c *Client) Exists(name string) (bool, error) {
	if !c.ready() {
		return false, fmt.Errorf("cloudflare is not configured")
	}
	endpoint := c.baseURL + "/zones/" + url.PathEscape(c.zoneID) + "/dns_records?name=" + url.QueryEscape(name)
	body, err := c.do(http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	var parsed struct {
		Success bool `json:"success"`
		Result  []struct {
			ID string `json:"id"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, err
	}
	if !parsed.Success {
		return false, fmt.Errorf("%s", firstError(parsed.Errors))
	}
	return len(parsed.Result) > 0, nil
}

// Create adds a DNS record and returns its id.
func (c *Client) Create(name string) (string, error) {
	if !c.ready() {
		return "", fmt.Errorf("cloudflare is not configured")
	}
	payload, _ := json.Marshal(map[string]any{
		"type":    c.kind,
		"name":    name,
		"content": c.content,
		"proxied": c.proxied,
		"ttl":     1,
	})
	endpoint := c.baseURL + "/zones/" + url.PathEscape(c.zoneID) + "/dns_records"
	body, err := c.do(http.MethodPost, endpoint, payload)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Success bool `json:"success"`
		Result  struct {
			ID string `json:"id"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if !parsed.Success || parsed.Result.ID == "" {
		return "", fmt.Errorf("%s", firstError(parsed.Errors))
	}
	return parsed.Result.ID, nil
}

// Delete removes a DNS record. A missing record is not an error.
func (c *Client) Delete(recordID string) error {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" || !c.ready() {
		return nil
	}
	endpoint := c.baseURL + "/zones/" + url.PathEscape(c.zoneID) + "/dns_records/" + url.PathEscape(recordID)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("cloudflare %s: %s", res.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (c *Client) do(method, endpoint string, payload []byte) ([]byte, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudflare %s: %s", res.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func firstError(errs []struct {
	Message string `json:"message"`
}) string {
	if len(errs) == 0 || strings.TrimSpace(errs[0].Message) == "" {
		return "cloudflare menolak permintaan"
	}
	return errs[0].Message
}

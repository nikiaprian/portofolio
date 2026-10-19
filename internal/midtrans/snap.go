package midtrans

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls Midtrans Snap.
type Client struct {
	serverKey string
	clientKey string
	isProd    bool
	http      *http.Client
}

func NewClient(serverKey, clientKey string, isProd bool) *Client {
	return &Client{
		serverKey: strings.TrimSpace(serverKey),
		clientKey: strings.TrimSpace(clientKey),
		isProd:    isProd,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.serverKey != "" && c.clientKey != ""
}

func (c *Client) ServerKey() string { return c.serverKey }
func (c *Client) ClientKey() string { return c.clientKey }

func (c *Client) SnapJSURL() string {
	if c.isProd {
		return "https://app.midtrans.com/snap/snap.js"
	}
	return "https://app.sandbox.midtrans.com/snap/snap.js"
}

func (c *Client) snapURL() string {
	if c.isProd {
		return "https://app.midtrans.com/snap/v1/transactions"
	}
	return "https://app.sandbox.midtrans.com/snap/v1/transactions"
}

// SnapRequest is the subset of the Snap payload this product uses.
type SnapRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	ItemDetails        []ItemDetail       `json:"item_details,omitempty"`
	CustomerDetails    *CustomerDetails   `json:"customer_details,omitempty"`
	Callbacks          *SnapCallbacks     `json:"callbacks,omitempty"`
}

type TransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type ItemDetail struct {
	ID       string `json:"id"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
}

type CustomerDetails struct {
	FirstName string `json:"first_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

type SnapCallbacks struct {
	Finish string `json:"finish"`
}

type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

func (c *Client) CreateSnapToken(req SnapRequest) (*SnapResponse, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("midtrans is not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.snapURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.serverKey+":")))

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("midtrans snap error (%d): %s", resp.StatusCode, string(raw))
	}
	var out SnapResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Token) == "" {
		return nil, fmt.Errorf("midtrans snap failed: empty token")
	}
	return &out, nil
}

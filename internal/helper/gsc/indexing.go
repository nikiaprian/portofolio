package gsc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	GoogleIndexingScope    = "https://www.googleapis.com/auth/indexing https://www.googleapis.com/auth/webmasters.readonly"
	GoogleTokenURL         = "https://oauth2.googleapis.com/token"
	GoogleIndexingAPI      = "https://indexing.googleapis.com/v3/urlNotifications:publish"
	GoogleURLInspectionAPI = "https://searchconsole.googleapis.com/v1/urlInspection/index:inspect"
)

type ServiceAccountCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

type IndexingResponse struct {
	URLNotificationMetadata struct {
		LatestUpdate struct {
			URL        string `json:"url"`
			Type       string `json:"type"`
			NotifyTime string `json:"notifyTime"`
		} `json:"latestUpdate"`
	} `json:"urlNotificationMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type InspectionResult struct {
	IsIndexed     bool   `json:"is_indexed"`
	Verdict       string `json:"verdict"`
	CoverageState string `json:"coverage_state"`
	LastCrawlTime string `json:"last_crawl_time"`
}

type urlInspectionResponse struct {
	InspectionResult struct {
		IndexStatusResult struct {
			Verdict       string `json:"verdict"`
			CoverageState string `json:"coverageState"`
			IndexingState string `json:"indexingState"`
			LastCrawlTime string `json:"lastCrawlTime"`
		} `json:"indexStatusResult"`
	} `json:"inspectionResult"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// ParseServiceAccountJSON parses and validates Google Service Account JSON bytes.
func ParseServiceAccountJSON(rawJSON string) (*ServiceAccountCredentials, error) {
	rawJSON = strings.TrimSpace(rawJSON)
	if rawJSON == "" {
		return nil, fmt.Errorf("kredensial Service Account JSON kosong")
	}
	var creds ServiceAccountCredentials
	if err := json.Unmarshal([]byte(rawJSON), &creds); err != nil {
		return nil, fmt.Errorf("format Service Account JSON tidak valid: %w", err)
	}
	if creds.ClientEmail == "" || creds.PrivateKey == "" {
		return nil, fmt.Errorf("JSON Service Account harus memiliki client_email dan private_key")
	}
	return &creds, nil
}

// GetAccessToken obtains an OAuth2 access token for Google Indexing API using RSA-signed JWT.
func GetAccessToken(creds *ServiceAccountCredentials) (string, error) {
	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(creds.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("gagal membaca private_key Service Account: %w", err)
	}

	tokenURI := creds.TokenURI
	if tokenURI == "" {
		tokenURI = GoogleTokenURL
	}

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":   creds.ClientEmail,
		"sub":   creds.ClientEmail,
		"aud":   tokenURI,
		"scope": GoogleIndexingScope,
		"iat":   now.Unix(),
		"exp":   now.Add(1 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedJWT, err := token.SignedString(parsedKey)
	if err != nil {
		return "", fmt.Errorf("gagal menandatangani JWT assertion: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signedJWT)

	req, err := http.NewRequest("POST", tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("gagal membuat request token Google: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi OAuth Google: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OAuth Google mengembalikan status %d: %s", res.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("gagal membaca token OAuth Google: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("access_token Google kosong")
	}
	return tokenResp.AccessToken, nil
}

// RequestURLIndexing submits a URL to Google Indexing API with notification type (e.g. "URL_UPDATED").
func RequestURLIndexing(rawJSON string, targetURL string, notificationType string) (*IndexingResponse, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, fmt.Errorf("URL target indexing tidak boleh kosong")
	}
	if notificationType == "" {
		notificationType = "URL_UPDATED"
	}

	creds, err := ParseServiceAccountJSON(rawJSON)
	if err != nil {
		return nil, err
	}

	accessToken, err := GetAccessToken(creds)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{
		"url":  targetURL,
		"type": notificationType,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun JSON payload indexing: %w", err)
	}

	req, err := http.NewRequest("POST", GoogleIndexingAPI, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("gagal membuat HTTP request Google Indexing API: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal mengirimkan request ke Google Indexing API: %w", err)
	}
	defer res.Body.Close()

	resBody, _ := io.ReadAll(res.Body)
	var idxResp IndexingResponse
	_ = json.Unmarshal(resBody, &idxResp)

	if res.StatusCode != http.StatusOK {
		if idxResp.Error != nil && idxResp.Error.Message != "" {
			errMsg := idxResp.Error.Message
			if strings.Contains(errMsg, "Permission denied") || strings.Contains(errMsg, "ownership") {
				errMsg = "Permission denied (403): Email Service Account belum ditambahkan sebagai Pemilik (Owner) di Google Search Console."
			} else if strings.Contains(errMsg, "has not been used") || strings.Contains(errMsg, "disabled") {
				errMsg = "API Disabled (403): Silakan aktifkan Web Search Indexing API di Google Cloud Console."
			}
			return nil, fmt.Errorf("Google Indexing API (%d): %s", idxResp.Error.Code, errMsg)
		}
		return nil, fmt.Errorf("Google Indexing API status %d: %s", res.StatusCode, string(resBody))
	}

	return &idxResp, nil
}

// InspectURL queries Google Search Console URL Inspection API to check real-time indexing status.
func InspectURL(rawJSON string, siteURL string, targetURL string) (*InspectionResult, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, fmt.Errorf("target URL tidak boleh kosong")
	}

	creds, err := ParseServiceAccountJSON(rawJSON)
	if err != nil {
		return nil, err
	}

	accessToken, err := GetAccessToken(creds)
	if err != nil {
		return nil, err
	}

	siteURL = strings.TrimSpace(siteURL)
	var candidates []string
	if siteURL != "" {
		candidates = append(candidates, siteURL)
	}
	if u, err := url.Parse(targetURL); err == nil && u.Host != "" {
		h := u.Host
		scheme := u.Scheme
		if scheme == "" {
			scheme = "https"
		}
		c1 := scheme + "://" + h + "/"
		c2 := scheme + "://" + h
		c3 := "sc-domain:" + h
		for _, c := range []string{c1, c2, c3} {
			if c != siteURL {
				candidates = append(candidates, c)
			}
		}
		parts := strings.Split(h, ".")
		if len(parts) >= 2 {
			domainOnly := strings.Join(parts[len(parts)-2:], ".")
			c4 := "sc-domain:" + domainOnly
			if c4 != siteURL && c4 != c3 {
				candidates = append(candidates, c4)
			}
		}
	}

	var lastErr error
	for _, cand := range candidates {
		payload := map[string]string{
			"inspectionUrl": targetURL,
			"siteUrl":       cand,
		}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			continue
		}

		req, err := http.NewRequest("POST", GoogleURLInspectionAPI, bytes.NewBuffer(jsonPayload))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 15 * time.Second}
		res, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("gagal menghubungi Google URL Inspection API: %w", err)
			continue
		}

		resBody, _ := io.ReadAll(res.Body)
		res.Body.Close()

		var inspectResp urlInspectionResponse
		_ = json.Unmarshal(resBody, &inspectResp)

		if res.StatusCode != http.StatusOK {
			if inspectResp.Error != nil && inspectResp.Error.Message != "" {
				lastErr = fmt.Errorf("Google URL Inspection API (%d): %s", inspectResp.Error.Code, inspectResp.Error.Message)
			} else {
				lastErr = fmt.Errorf("Google URL Inspection API status %d: %s", res.StatusCode, string(resBody))
			}
			continue
		}

		resIdx := inspectResp.InspectionResult.IndexStatusResult
		verdict := strings.ToUpper(strings.TrimSpace(resIdx.Verdict))
		covState := strings.ToLower(strings.TrimSpace(resIdx.CoverageState))
		isIndexed := verdict == "PASS" || strings.Contains(covState, "indexed")

		return &InspectionResult{
			IsIndexed:     isIndexed,
			Verdict:       resIdx.Verdict,
			CoverageState: resIdx.CoverageState,
			LastCrawlTime: resIdx.LastCrawlTime,
		}, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("gagal menginspeksi URL di Google Search Console")
}

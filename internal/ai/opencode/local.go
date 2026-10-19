package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefaultLocalPort is OpenCode serve/web default.
	DefaultLocalPort = "4096"
	// DefaultLocalHost is used when host is empty but port is set.
	DefaultLocalHost = "http://127.0.0.1"
	// LocalModelAuto selects free models with automatic fallback on quota/rate-limit.
	LocalModelAuto = "auto"

	// localHTTPTimeout is per-request (status/list/async accept). Long generation uses polling.
	localHTTPTimeout = 45 * time.Second
	// localGenerateMaxWait caps how long we wait for one article (matches bulk AI budget).
	localGenerateMaxWait = 15 * time.Minute
	// localContinueMax is how many "continue writing" follow-ups after truncation/short replies.
	localContinueMax = 2
)

// LocalConfig holds connection settings for a self-hosted OpenCode server.
type LocalConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Model    string // "auto", "provider/model", bare model id, or empty (= auto)
}

// LocalModel is one model entry returned to the admin UI.
type LocalModel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProviderID string `json:"providerID"`
	Free       bool   `json:"free"`
}

// LocalModelRef is a provider/model pair.
type LocalModelRef struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

// LocalModelsResponse matches the admin UI contract for listing OpenCode local models.
type LocalModelsResponse struct {
	Models             []LocalModel   `json:"models"`
	DefaultModel       *LocalModelRef `json:"defaultModel,omitempty"`
	ConnectedProviders []string       `json:"connectedProviders"`
}

// NormalizeBaseURL builds http(s)://host[:port] from host + optional port.
// Host may already include scheme and/or port. Empty port defaults to 4096
// when the host URL has no port either.
func NormalizeBaseURL(host, port string) (string, error) {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" {
		if port == "" {
			return "", errors.New("opencode local: empty host")
		}
		host = DefaultLocalHost
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("opencode local: invalid host: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("opencode local: unsupported scheme %q (use http or https)", u.Scheme)
	}
	if u.Hostname() == "" {
		return "", errors.New("opencode local: empty hostname")
	}

	effectivePort := port
	if effectivePort == "" {
		effectivePort = u.Port()
	}
	if effectivePort == "" {
		effectivePort = DefaultLocalPort
	}
	u.Host = net.JoinHostPort(u.Hostname(), effectivePort)
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return strings.TrimRight(u.String(), "/"), nil
}

// ListLocalModels fetches models from the OpenCode local server (/config/providers).
func ListLocalModels(ctx context.Context, cfg LocalConfig) (*LocalModelsResponse, error) {
	baseURL, err := NormalizeBaseURL(cfg.Host, cfg.Port)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	var raw localConfigProvidersResp
	if err := localDoJSON(ctx, client, http.MethodGet, baseURL+"/config/providers", cfg.Username, cfg.Password, nil, &raw); err != nil {
		return nil, err
	}
	return buildLocalModelsResponse(&raw), nil
}

// GenerateContentLocal creates a temporary OpenCode session, prompts it, and returns assistant text.
// Model "auto"/empty tries free models and falls through on quota/rate-limit.
func GenerateContentLocal(ctx context.Context, cfg LocalConfig, userInput string) (string, error) {
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return "", errors.New("opencode local: empty user input")
	}
	baseURL, err := NormalizeBaseURL(cfg.Host, cfg.Port)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: localHTTPTimeout}
	models := resolveLocalModelCandidates(ctx, client, baseURL, cfg)
	if len(models) == 0 {
		return "", errors.New("opencode local: tidak ada model yang bisa dipakai")
	}

	var lastQuota error
	var lastErr error
	triedQuota := 0
	for _, model := range models {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		text, genErr := generateOnceLocal(ctx, client, baseURL, cfg.Username, cfg.Password, model, userInput)
		if genErr == nil {
			if strings.TrimSpace(text) == "" {
				lastErr = errors.New("opencode local: empty assistant response")
				continue
			}
			return text, nil
		}
		lastErr = genErr
		if isLocalQuotaOrRateLimit(genErr) {
			triedQuota++
			lastQuota = genErr
			continue
		}
		// Non-quota errors on a specific free auto candidate: try next free model.
		if isLocalAutoModel(cfg.Model) && len(models) > 1 {
			continue
		}
		return "", genErr
	}
	if triedQuota > 0 && triedQuota == len(models) {
		return "", fmt.Errorf("%w", ErrAllFreeTokensUsed)
	}
	if lastQuota != nil && lastErr != nil && isLocalQuotaOrRateLimit(lastErr) {
		return "", fmt.Errorf("%w", ErrAllFreeTokensUsed)
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("%w", ErrAllFreeTokensUsed)
}

func generateOnceLocal(ctx context.Context, client *http.Client, baseURL, user, pass, model, userInput string) (string, error) {
	sessionID, err := createLocalSession(ctx, client, baseURL, user, pass)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = deleteLocalSession(context.Background(), client, baseURL, user, pass, sessionID)
	}()

	waitCtx, cancel := withLocalGenerateDeadline(ctx)
	defer cancel()

	if err := promptLocalSessionAsync(waitCtx, client, baseURL, user, pass, sessionID, model, userInput, localArticleSystem); err != nil {
		return "", err
	}
	if err := waitLocalSessionIdle(waitCtx, client, baseURL, user, pass, sessionID); err != nil {
		// Still try to salvage any partial assistant text (OpenCode may have been mid-retry).
		salvageCtx, salvageCancel := context.WithTimeout(context.Background(), 20*time.Second)
		partial, _ := collectLocalAssistantText(salvageCtx, client, baseURL, user, pass, sessionID)
		salvageCancel()
		if strings.TrimSpace(partial) != "" {
			return partial, nil
		}
		return "", err
	}

	text, err := collectLocalAssistantText(waitCtx, client, baseURL, user, pass, sessionID)
	if err != nil {
		return "", err
	}
	if looksLikeQuotaPayload(text) {
		return "", fmt.Errorf("%w", ErrQuotaOrRateLimit)
	}

	// Mirror OpenCode UI: on truncated / short drafts, ask the same session to continue.
	for i := 0; i < localContinueMax && shouldContinueLocalArticle(text, userInput); i++ {
		cont := "Lanjutkan artikel HTML dari potongan terakhir. Jangan ulangi bagian yang sudah ada. " +
			"Output hanya fragment HTML lanjutan (tanpa markdown, tanpa rencana, tanpa penjelasan)."
		if err := promptLocalSessionAsync(waitCtx, client, baseURL, user, pass, sessionID, model, cont, localArticleSystem); err != nil {
			break
		}
		if err := waitLocalSessionIdle(waitCtx, client, baseURL, user, pass, sessionID); err != nil {
			break
		}
		next, err2 := collectLocalAssistantText(waitCtx, client, baseURL, user, pass, sessionID)
		if err2 != nil || strings.TrimSpace(next) == "" {
			break
		}
		if utf8.RuneCountInString(next) <= utf8.RuneCountInString(text) {
			break
		}
		text = strings.TrimSpace(next)
	}

	if strings.TrimSpace(text) == "" {
		return "", errors.New("opencode local: empty assistant response")
	}
	return text, nil
}

func withLocalGenerateDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(localGenerateMaxWait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	return context.WithDeadline(ctx, deadline)
}

func buildLocalPromptBody(model, userInput, system string) ([]byte, error) {
	reqBody := localMessageReq{
		Agent:  "build",
		System: system,
		Tools:  localArticleTools(),
		Parts:  []localTextPartIn{{Type: "text", Text: userInput}},
	}
	if ref := parseLocalModel(model); ref != nil {
		reqBody.Model = ref
	}
	return json.Marshal(reqBody)
}

func promptLocalSessionAsync(ctx context.Context, client *http.Client, baseURL, user, pass, sessionID, model, userInput, system string) error {
	raw, err := buildLocalPromptBody(model, userInput, system)
	if err != nil {
		return err
	}
	endpoint := baseURL + "/session/" + url.PathEscape(sessionID) + "/prompt_async"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setLocalAuth(req, user, pass)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("opencode local: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if looksLikeQuotaPayload(msg) {
			return fmt.Errorf("%w", ErrQuotaOrRateLimit)
		}
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("opencode local: prompt_async: %s", msg)
	}
	return nil
}

type localSessionStatus struct {
	Type    string `json:"type"`
	Attempt int    `json:"attempt"`
	Message string `json:"message"`
	Next    int    `json:"next"`
}

func waitLocalSessionIdle(ctx context.Context, client *http.Client, baseURL, user, pass, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		st, err := getLocalSessionStatus(ctx, client, baseURL, user, pass, sessionID)
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("opencode local: menunggu respons: %w", ctx.Err())
			}
			// Transient status fetch errors: keep polling until deadline.
		} else {
			switch strings.ToLower(strings.TrimSpace(st.Type)) {
			case "idle", "":
				return nil
			case "retry":
				// OpenCode keeps generating after provider blips — wait like the TUI.
				if looksLikeQuotaPayload(st.Message) {
					return fmt.Errorf("%w", ErrQuotaOrRateLimit)
				}
				wait := time.Duration(st.Next) * time.Millisecond
				if wait < 2*time.Second {
					wait = 2 * time.Second
				}
				if wait > 30*time.Second {
					wait = 30 * time.Second
				}
				select {
				case <-ctx.Done():
					return fmt.Errorf("opencode local: menunggu retry: %w", ctx.Err())
				case <-time.After(wait):
				}
				continue
			case "busy":
				// keep polling
			default:
				// unknown → keep polling
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("opencode local: menunggu respons: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func getLocalSessionStatus(ctx context.Context, client *http.Client, baseURL, user, pass, sessionID string) (localSessionStatus, error) {
	var all map[string]localSessionStatus
	if err := localDoJSON(ctx, client, http.MethodGet, baseURL+"/session/status", user, pass, nil, &all); err != nil {
		return localSessionStatus{}, err
	}
	if st, ok := all[sessionID]; ok {
		return st, nil
	}
	// Missing entry usually means idle/completed and dropped from the active map.
	return localSessionStatus{Type: "idle"}, nil
}

type localMessageListItem struct {
	Info  localMessageInfo `json:"info"`
	Parts []localPartOut   `json:"parts"`
}

type localMessageInfo struct {
	Role  string          `json:"role"`
	Error json.RawMessage `json:"error"`
}

func collectLocalAssistantText(ctx context.Context, client *http.Client, baseURL, user, pass, sessionID string) (string, error) {
	var list []localMessageListItem
	endpoint := baseURL + "/session/" + url.PathEscape(sessionID) + "/message"
	if err := localDoJSON(ctx, client, http.MethodGet, endpoint, user, pass, nil, &list); err != nil {
		return "", err
	}
	var b strings.Builder
	var lastErr string
	for _, msg := range list {
		if !strings.EqualFold(msg.Info.Role, "assistant") {
			continue
		}
		if len(msg.Info.Error) > 0 && string(msg.Info.Error) != "null" {
			lastErr = strings.TrimSpace(string(msg.Info.Error))
			if looksLikeQuotaPayload(lastErr) {
				return "", fmt.Errorf("%w", ErrQuotaOrRateLimit)
			}
		}
		for _, p := range msg.Parts {
			if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(p.Text)
			}
		}
	}
	text := b.String()
	if strings.TrimSpace(text) == "" && lastErr != "" {
		return "", fmt.Errorf("opencode local: %s", lastErr)
	}
	return text, nil
}

func shouldContinueLocalArticle(text, userInput string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	// Output-length / aborted style errors sometimes leave short HTML.
	runes := utf8.RuneCountInString(t)
	wantLong := strings.Contains(strings.ToLower(userInput), "3000") ||
		strings.Contains(strings.ToLower(userInput), "2500") ||
		strings.Contains(strings.ToLower(userInput), "2000") ||
		strings.Contains(userInput, "minimal 1000")
	if wantLong && runes < 3500 {
		return true
	}
	if runes < 800 {
		return true
	}
	low := strings.ToLower(t)
	if strings.Contains(low, "output length") || strings.Contains(low, "truncated") {
		return true
	}
	// Obvious cut-off: ends mid-tag.
	trimRight := strings.TrimRight(t, " \t\n\r")
	if strings.HasSuffix(trimRight, "<") || strings.HasSuffix(trimRight, "</") {
		return true
	}
	return false
}

func isLocalAutoModel(raw string) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	return v == "" || v == LocalModelAuto
}

func resolveLocalModelCandidates(ctx context.Context, client *http.Client, baseURL string, cfg LocalConfig) []string {
	model := strings.TrimSpace(cfg.Model)
	if !isLocalAutoModel(model) {
		return []string{model}
	}

	var raw localConfigProvidersResp
	if err := localDoJSON(ctx, client, http.MethodGet, baseURL+"/config/providers", cfg.Username, cfg.Password, nil, &raw); err != nil {
		// Fall back to known free Zen models when listing fails.
		return defaultFreeLocalModels()
	}
	resp := buildLocalModelsResponse(&raw)
	free := make([]string, 0, len(resp.Models))
	seen := map[string]struct{}{}
	add := func(providerID, modelID string) {
		providerID = strings.TrimSpace(providerID)
		modelID = strings.TrimSpace(modelID)
		if providerID == "" || modelID == "" {
			return
		}
		key := providerID + "/" + modelID
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		free = append(free, key)
	}
	if resp.DefaultModel != nil {
		// Prefer server default when it is free.
		for _, m := range resp.Models {
			if m.Free && m.ProviderID == resp.DefaultModel.ProviderID && m.ID == resp.DefaultModel.ModelID {
				add(m.ProviderID, m.ID)
				break
			}
		}
	}
	for _, m := range resp.Models {
		if m.Free {
			add(m.ProviderID, m.ID)
		}
	}
	if len(free) == 0 {
		return defaultFreeLocalModels()
	}
	return free
}

func defaultFreeLocalModels() []string {
	return []string{
		"opencode/big-pickle",
		"opencode/deepseek-v4-flash-free",
		"opencode/mimo-v2.5-free",
		"opencode/laguna-s-2.1-free",
		"opencode/ling-3.0-flash-free",
		"opencode/north-mini-code-free",
		"opencode/nemotron-3-ultra-free",
	}
}

type localConfigProvidersResp struct {
	Providers []localProviderEntry `json:"providers"`
	Default   map[string]string    `json:"default"`
}

type localProviderEntry struct {
	ID     string                     `json:"id"`
	Name   string                     `json:"name"`
	Models map[string]localModelEntry `json:"models"`
}

type localModelEntry struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	ProviderID string          `json:"providerID"`
	Cost       *localModelCost `json:"cost"`
	Status     string          `json:"status"`
}

type localModelCost struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

func buildLocalModelsResponse(raw *localConfigProvidersResp) *LocalModelsResponse {
	out := &LocalModelsResponse{
		Models:             make([]LocalModel, 0, 32),
		ConnectedProviders: make([]string, 0, 8),
	}
	if raw == nil {
		return out
	}
	for _, p := range raw.Providers {
		pid := strings.TrimSpace(p.ID)
		if pid == "" {
			continue
		}
		out.ConnectedProviders = append(out.ConnectedProviders, pid)
		ids := make([]string, 0, len(p.Models))
		for id := range p.Models {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			m := p.Models[id]
			modelID := strings.TrimSpace(m.ID)
			if modelID == "" {
				modelID = strings.TrimSpace(id)
			}
			providerID := strings.TrimSpace(m.ProviderID)
			if providerID == "" {
				providerID = pid
			}
			name := strings.TrimSpace(m.Name)
			if name == "" {
				name = modelID
			}
			out.Models = append(out.Models, LocalModel{
				ID:         modelID,
				Name:       name,
				ProviderID: providerID,
				Free:       isLocalFreeModel(providerID, modelID, m.Cost),
			})
		}
	}
	sort.SliceStable(out.Models, func(i, j int) bool {
		a, b := out.Models[i], out.Models[j]
		if a.Free != b.Free {
			return a.Free
		}
		if a.ProviderID != b.ProviderID {
			return a.ProviderID < b.ProviderID
		}
		return a.ID < b.ID
	})
	// Prefer opencode default when present.
	if mid := strings.TrimSpace(raw.Default["opencode"]); mid != "" {
		out.DefaultModel = &LocalModelRef{ProviderID: "opencode", ModelID: mid}
	} else {
		for pid, mid := range raw.Default {
			pid = strings.TrimSpace(pid)
			mid = strings.TrimSpace(mid)
			if pid == "" || mid == "" {
				continue
			}
			out.DefaultModel = &LocalModelRef{ProviderID: pid, ModelID: mid}
			break
		}
	}
	return out
}

func isLocalFreeModel(providerID, modelID string, cost *localModelCost) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if strings.HasSuffix(id, "-free") || id == "big-pickle" {
		return true
	}
	// Zero-cost OpenCode Zen models only (avoid marking other connected providers as free).
	if !strings.EqualFold(strings.TrimSpace(providerID), "opencode") {
		return false
	}
	if cost == nil {
		return false
	}
	return cost.Input == 0 && cost.Output == 0
}

type localSessionResp struct {
	ID string `json:"id"`
}

type localMessageReq struct {
	Model  *localModelRef    `json:"model,omitempty"`
	Agent  string            `json:"agent,omitempty"`
	System string            `json:"system,omitempty"`
	Tools  map[string]bool   `json:"tools,omitempty"`
	Parts  []localTextPartIn `json:"parts"`
}

// localArticleTools keeps workspace tools off (so OpenCode does not edit the project)
// but allows websearch/webfetch so drafts can use live facts (same Exa path as the OpenCode TUI).
func localArticleTools() map[string]bool {
	return map[string]bool{
		"bash":        false,
		"read":        false,
		"glob":        false,
		"grep":        false,
		"edit":        false,
		"write":       false,
		"task":        false,
		"todowrite":   false,
		"skill":       false,
		"apply_patch": false,
		"question":    false,
		"webfetch":    true,
		"websearch":   true,
	}
}

const localArticleSystem = `You are writing a finished blog article body for a CMS.
Before writing about current events, deaths, scores, prices, schedules, or "terkini" claims, use websearch (and webfetch if needed) to verify facts with today's real sources.
Reply with an HTML fragment only: start with <p>, then <h2>/<h3>/<p>/<ul>/<ol>/<li>/<table>/<strong>/<em>/<blockquote> as needed.
Never use Plan mode. Do not outline, ask for confirmation, or describe what you will do.
Do not narrate tool use in the final answer. Do not use markdown. Do not wrap the answer in code fences.`

type localModelRef struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

type localTextPartIn struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type localPartOut struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func createLocalSession(ctx context.Context, client *http.Client, baseURL, user, pass string) (string, error) {
	body := []byte(`{"title":"bebii-blog"}`)
	var out localSessionResp
	if err := localDoJSON(ctx, client, http.MethodPost, baseURL+"/session", user, pass, body, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", errors.New("opencode local: empty session id")
	}
	return out.ID, nil
}

func deleteLocalSession(ctx context.Context, client *http.Client, baseURL, user, pass, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/session/"+url.PathEscape(sessionID), nil)
	if err != nil {
		return err
	}
	setLocalAuth(req, user, pass)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func parseLocalModel(raw string) *localModelRef {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, LocalModelAuto) {
		return nil
	}
	if i := strings.IndexByte(raw, '/'); i > 0 && i < len(raw)-1 {
		return &localModelRef{ProviderID: raw[:i], ModelID: raw[i+1:]}
	}
	return &localModelRef{ProviderID: "opencode", ModelID: raw}
}

func setLocalAuth(req *http.Request, user, pass string) {
	user = strings.TrimSpace(user)
	pass = strings.TrimSpace(pass)
	if pass == "" && user == "" {
		return
	}
	if user == "" {
		user = "opencode"
	}
	req.SetBasicAuth(user, pass)
}

func localDoJSON(ctx context.Context, client *http.Client, method, endpoint, user, pass string, body []byte, dest any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	setLocalAuth(req, user, pass)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("opencode local: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("opencode local: read response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusPaymentRequired {
		return fmt.Errorf("%w", ErrQuotaOrRateLimit)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New("opencode local: autentikasi gagal (periksa username/password)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if looksLikeQuotaPayload(msg) {
			return fmt.Errorf("%w", ErrQuotaOrRateLimit)
		}
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("opencode local: HTTP %d: %s", resp.StatusCode, msg)
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, dest); err != nil {
		return fmt.Errorf("opencode local: decode response: %w", err)
	}
	return nil
}

func isLocalQuotaOrRateLimit(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrQuotaOrRateLimit) || errors.Is(err, ErrAllFreeTokensUsed) {
		return true
	}
	return looksLikeQuotaPayload(err.Error())
}

func looksLikeQuotaPayload(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || s == "null" {
		return false
	}
	keys := []string{
		"rate limit",
		"rate_limit",
		"quota",
		"tokens already used",
		"token limit",
		"out of tokens",
		"insufficient_quota",
		"too many requests",
		"payment required",
		"kuota",
		"batas",
	}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

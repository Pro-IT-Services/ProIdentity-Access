package pushauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://verify.proidentity.cloud"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return NewClientWithBaseURL(apiKey, baseURL, nil)
}

func NewClientWithBaseURL(apiKey, url string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if url == "" {
		url = baseURL
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    url,
		httpClient: httpClient,
	}
}

type AuthRequest struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	ExpiresAt int64  `json:"expires_at"`
}

type AuthStatus struct {
	Status     string `json:"status"`
	ApprovedAt *int64 `json:"approved_at,omitempty"`
	TOTPCode   string `json:"totp_code,omitempty"`
}

type VerifyResult struct {
	Valid  bool `json:"valid"`
	Window int  `json:"window"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	UserEmail string `json:"user_email,omitempty"`
	Status    int    `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return fmt.Sprintf("push auth api error: status %d", e.Status)
}

func (c *Client) CreateAuthRequest(email, title, detail, clientIP string, expirySec int) (*AuthRequest, error) {
	return c.createAuthRequest(email, "", title, detail, clientIP, expirySec)
}

func (c *Client) EnsureUser(email, displayName, clientIP string) (string, error) {
	req, err := c.createAuthRequest(email, displayName, "ProIdentity Access account setup", "Account provisioning", clientIP, 120)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.Code == "user_created_needs_setup" {
			return apiErr.Code, nil
		}
		return "", err
	}
	if req.Status != "" {
		return req.Status, nil
	}
	return "ok", nil
}

func (c *Client) createAuthRequest(email, displayName, title, detail, clientIP string, expirySec int) (*AuthRequest, error) {
	if expirySec <= 0 {
		expirySec = 120
	}
	payload := map[string]any{
		"user_email":     email,
		"context_title":  title,
		"context_detail": detail,
		"client_ip":      clientIP,
		"expiry_seconds": expirySec,
	}
	if displayName != "" {
		payload["display_name"] = displayName
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", c.baseURL+"/api/v1/sp/auth-requests", bytes.NewReader(body))
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("push auth request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 202 {
		apiErr := &APIError{Status: resp.StatusCode}
		if err := json.Unmarshal(data, apiErr); err == nil && (apiErr.Code != "" || apiErr.Message != "") {
			return nil, apiErr
		}
		return nil, fmt.Errorf("push auth request: %s (%s)", resp.Status, string(data))
	}
	var out AuthRequest
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("push auth parse: %w", err)
	}
	if out.RequestID == "" {
		apiErr := &APIError{Status: resp.StatusCode}
		if err := json.Unmarshal(data, apiErr); err == nil && (apiErr.Code != "" || apiErr.Message != "") {
			return nil, apiErr
		}
		return nil, fmt.Errorf("push auth response missing request_id")
	}
	return &out, nil
}

func (c *Client) PollStatus(requestID string) (*AuthStatus, error) {
	req, _ := http.NewRequest("GET", c.baseURL+"/api/v1/sp/auth-requests/"+requestID, nil)
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll push auth: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("poll push auth: %s (%s)", resp.Status, string(data))
	}
	var out AuthStatus
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("poll parse: %w", err)
	}
	return &out, nil
}

func (c *Client) VerifyTOTP(email, code string) (bool, error) {
	body, _ := json.Marshal(map[string]string{
		"user_email": email,
		"code":       code,
	})
	req, _ := http.NewRequest("POST", c.baseURL+"/api/v1/sp/verify-totp", bytes.NewReader(body))
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("verify totp: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out VerifyResult
	json.Unmarshal(data, &out)
	return out.Valid, nil
}

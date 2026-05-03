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
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
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

func (c *Client) CreateAuthRequest(email, title, detail, clientIP string, expirySec int) (*AuthRequest, error) {
	if expirySec <= 0 {
		expirySec = 120
	}
	body, _ := json.Marshal(map[string]any{
		"user_email":     email,
		"context_title":  title,
		"context_detail": detail,
		"client_ip":      clientIP,
		"expiry_seconds": expirySec,
	})
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/sp/auth-requests", bytes.NewReader(body))
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("push auth request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		return nil, fmt.Errorf("push auth request: %s (%s)", resp.Status, string(data))
	}
	var out AuthRequest
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("push auth parse: %w", err)
	}
	return &out, nil
}

func (c *Client) PollStatus(requestID string) (*AuthStatus, error) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/sp/auth-requests/"+requestID, nil)
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
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/sp/verify-totp", bytes.NewReader(body))
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

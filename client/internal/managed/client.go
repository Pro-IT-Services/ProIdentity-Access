package managed

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ErrDeviceRevoked is returned when the server indicates the device registration has been revoked.
var ErrDeviceRevoked = errors.New("device revoked")

// ErrAuthInvalid is returned when an authenticated request is rejected because
// the login token is expired, revoked, or no longer authorized.
var ErrAuthInvalid = errors.New("auth invalid")

// Client is an HTTP client for the ProIdentity management API.
type Client struct {
	BaseURL    string
	Token      string
	DeviceID   string
	aesKey     []byte
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewEncryptedClient creates a Client that transparently encrypts/decrypts
// request and response bodies using the derived X25519 shared key.
func NewEncryptedClient(baseURL, token, deviceID, clientPrivKey, serverPubKey string) (*Client, error) {
	key, err := DeriveAESKey(clientPrivKey, serverPubKey)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		DeviceID:   deviceID,
		aesKey:     key,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// LoginResponse is the response from POST /api/v1/auth/login.
type LoginResponse struct {
	Token           string `json:"token"`
	UserID          string `json:"user_id"`
	Username        string `json:"username"`
	IsAdmin         bool   `json:"is_admin"`
	RequireTOTP     bool   `json:"require_totp"`
	TOTPEnabled     bool   `json:"totp_enabled"`
	PushAuthEnabled bool   `json:"push_auth_enabled"`
	PushRequestID   string `json:"push_request_id"`
}

// ServerInfo is a WireGuard server the user can connect to.
type ServerInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Endpoint  string  `json:"endpoint"`
	Port      int     `json:"port"`
	PublicKey string  `json:"public_key"`
	Subnet    string  `json:"subnet"`
	DNS       *string `json:"dns"`
}

// SessionResponse is the response from POST /api/v1/sessions.
type SessionResponse struct {
	SessionID       string `json:"session_id"`
	AssignedIP      string `json:"assigned_ip"`
	ServerID        string `json:"server_id"`
	WGConfig        string `json:"wg_config"`
	VPNName         string `json:"vpn_name"`
	RequireTOTP     bool   `json:"require_totp"`
	PushAuthEnabled bool   `json:"push_auth_enabled"`
}

// PushAuthResponse is the response from POST /api/v1/auth/push.
type PushAuthResponse struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	ExpiresAt int64  `json:"expires_at"`
}

// PushAuthStatus is the response from GET /api/v1/auth/push/{id}.
type PushAuthStatus struct {
	Status string `json:"status"`
}

// InfoResponse is the response from GET /api/v1/info.
type InfoResponse struct {
	VPNName string `json:"vpn_name"`
}

// RegisterResponse is the response from POST /api/v1/register.
type RegisterResponse struct {
	DeviceID        string `json:"device_id"`
	ServerPublicKey string `json:"server_public_key"`
}

// UserConfigInfo is config metadata returned by GET /api/v1/user/configs.
type UserConfigInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (c *Client) GetInfo() (*InfoResponse, error) {
	if err := requireSecureBaseURL(c.BaseURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", c.BaseURL+"/api/v1/info", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer res.Body.Close()
	var info InfoResponse
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Register registers this device with the server and returns the server's public key.
func (c *Client) Register(deviceName, clientPublicKey string) (*RegisterResponse, error) {
	var resp RegisterResponse
	if err := c.post("/register", map[string]string{
		"device_name":       deviceName,
		"client_public_key": clientPublicKey,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Login(username, password, totpCode string) (*LoginResponse, error) {
	body := map[string]any{"username": username, "password": password}
	if totpCode != "" {
		body["totp_code"] = totpCode
	}
	var resp LoginResponse
	if err := c.post("/auth/login", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) LoginWithPush(username, password, pushAuthID string) (*LoginResponse, error) {
	body := map[string]any{
		"username":             username,
		"password":             password,
		"push_auth_request_id": pushAuthID,
	}
	var resp LoginResponse
	if err := c.post("/auth/login", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PollPushStatus polls the public push auth status endpoint (no auth required).
func (c *Client) PollPushStatus(requestID string) (*PushAuthStatus, error) {
	var resp PushAuthStatus
	if err := c.do("GET", "/auth/push-status/"+requestID, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CheckAuth calls /auth/me to verify the current token is still valid.
func (c *Client) CheckAuth() error {
	return c.getAuth("/auth/me", nil)
}

// ListServers returns the WireGuard servers accessible to the authenticated user.
func (c *Client) ListServers() ([]ServerInfo, error) {
	var servers []ServerInfo
	if err := c.getAuth("/servers", &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

// CreateSession creates a VPN session on the given server with the client's public key.
// totpCode is optional; pass "" if not needed.
func (c *Client) CreateSession(serverID, clientPublicKey, totpCode string) (*SessionResponse, error) {
	body := map[string]any{
		"server_id":         serverID,
		"client_public_key": clientPublicKey,
	}
	if totpCode != "" {
		body["totp_code"] = totpCode
	}
	var resp SessionResponse
	if err := c.postAuth("/sessions", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreatePushAuth initiates a push auth request for the current user.
func (c *Client) CreatePushAuth(context string) (*PushAuthResponse, error) {
	body := map[string]string{"context": context}
	var resp PushAuthResponse
	if err := c.postAuth("/auth/push", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PollPushAuth returns the current status of a push auth request.
func (c *Client) PollPushAuth(requestID string) (*PushAuthStatus, error) {
	var resp PushAuthStatus
	if err := c.getAuth("/auth/push/"+requestID, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateSessionWithPush creates a session using an approved push auth request ID.
func (c *Client) CreateSessionWithPush(serverID, clientPublicKey, pushAuthID string) (*SessionResponse, error) {
	body := map[string]any{
		"server_id":            serverID,
		"client_public_key":    clientPublicKey,
		"push_auth_request_id": pushAuthID,
	}
	var resp SessionResponse
	if err := c.postAuth("/sessions", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Keepalive(sessionID string) error {
	return c.postAuth("/sessions/"+sessionID+"/keepalive", nil, nil)
}

func (c *Client) DeleteSession(sessionID string) error {
	return c.deleteAuth("/sessions/" + sessionID)
}

// GetUserConfigKey returns the user's 32-byte config encryption key from the server.
func (c *Client) GetUserConfigKey() ([]byte, error) {
	var resp struct {
		Key string `json:"key"`
	}
	if err := c.getAuth("/user/config-key", &resp); err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(resp.Key)
	if err != nil {
		return nil, fmt.Errorf("decode config key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("unexpected key length %d", len(key))
	}
	return key, nil
}

// ListUserConfigs returns metadata for all configs stored on the server.
func (c *Client) ListUserConfigs() ([]UserConfigInfo, error) {
	var configs []UserConfigInfo
	if err := c.getAuth("/user/configs", &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// UploadUserConfig encrypts and uploads a WireGuard config to the server.
// encryptedContent is the binary ciphertext (will be base64-encoded for transport).
// Returns the server-assigned config ID.
func (c *Client) UploadUserConfig(name string, encryptedContent []byte) (string, error) {
	body := map[string]string{
		"name":              name,
		"encrypted_content": base64.StdEncoding.EncodeToString(encryptedContent),
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.postAuth("/user/configs", body, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// DownloadUserConfig fetches the encrypted content for a config by ID.
func (c *Client) DownloadUserConfig(id string) ([]byte, error) {
	var resp struct {
		EncryptedContent string `json:"encrypted_content"`
	}
	if err := c.getAuth("/user/configs/"+id, &resp); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(resp.EncryptedContent)
}

// DeleteUserConfig deletes a config from the server.
func (c *Client) DeleteUserConfig(id string) error {
	return c.deleteAuth("/user/configs/" + id)
}

// --- Internal helpers ---

func (c *Client) post(path string, body any, out any) error {
	return c.do("POST", path, nil, body, out)
}

func (c *Client) postAuth(path string, body any, out any) error {
	headers := map[string]string{"Authorization": "Bearer " + c.Token}
	return c.do("POST", path, headers, body, out)
}

func (c *Client) getAuth(path string, out any) error {
	headers := map[string]string{"Authorization": "Bearer " + c.Token}
	return c.do("GET", path, headers, nil, out)
}

func (c *Client) deleteAuth(path string) error {
	headers := map[string]string{"Authorization": "Bearer " + c.Token}
	return c.do("DELETE", path, headers, nil, nil)
}

func (c *Client) do(method, path string, headers map[string]string, body any, out any) error {
	if err := requireSecureBaseURL(c.BaseURL); err != nil {
		return err
	}
	authRequest := headers != nil && headers["Authorization"] != ""

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}

	// Encrypt the body if this client has a device key
	var reqBody []byte
	if c.aesKey != nil && buf.Len() > 0 {
		aad := []byte(c.DeviceID)
		encrypted, err := encryptBody(c.aesKey, buf.Bytes(), aad)
		if err != nil {
			return fmt.Errorf("encrypt request: %w", err)
		}
		reqBody = encrypted
	} else {
		reqBody = buf.Bytes()
	}

	req, err := http.NewRequest(method, c.BaseURL+"/api/v1"+path, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ProIdentity-Access")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.DeviceID != "" {
		req.Header.Set("X-Device-ID", c.DeviceID)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Decrypt the response if this client has a device key
	if c.aesKey != nil && len(respBytes) > 0 {
		aad := []byte(c.DeviceID)
		plain, err := decryptBody(c.aesKey, respBytes, aad)
		if err != nil {
			// Middleware may reject before encrypting (e.g. revoked device sends plain JSON error)
			var plainErr struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(respBytes, &plainErr) == nil && plainErr.Error != "" {
				if res.StatusCode == 401 && (plainErr.Error == "device revoked" || plainErr.Error == "unknown device") {
					return ErrDeviceRevoked
				}
				if authRequest && (res.StatusCode == 401 || res.StatusCode == 403) {
					return ErrAuthInvalid
				}
				return fmt.Errorf("%s", plainErr.Error)
			}
			return fmt.Errorf("decrypt response: %w", err)
		}
		respBytes = plain
	}

	if res.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(respBytes, &errResp)
		if errResp.Error != "" {
			if res.StatusCode == 401 && (errResp.Error == "device revoked" || errResp.Error == "unknown device") {
				return ErrDeviceRevoked
			}
			if authRequest && (res.StatusCode == 401 || res.StatusCode == 403) {
				return ErrAuthInvalid
			}
			return fmt.Errorf("%s", errResp.Error)
		}
		if authRequest && (res.StatusCode == 401 || res.StatusCode == 403) {
			return ErrAuthInvalid
		}
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}

	if out != nil {
		return json.Unmarshal(respBytes, out)
	}
	return nil
}

func requireSecureBaseURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		if isLocalHost(host) || os.Getenv("PROIDENTITY_ALLOW_INSECURE_HTTP") == "1" {
			return nil
		}
	}
	return fmt.Errorf("server URL must use HTTPS")
}

func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

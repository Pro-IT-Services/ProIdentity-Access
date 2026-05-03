package ipc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// tokenLength is the number of random bytes in the IPC auth token.
const tokenLength = 32

// tokenFilePath returns the path where the daemon writes the session token.
func tokenFilePath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "ProIdentity", "ipc.token")
	}
	return "/var/run/proidentity.token"
}

// GenerateToken creates a cryptographically random token and persists it to
// the well-known token file so the GUI can read it.
func GenerateToken() (string, error) {
	raw := make([]byte, tokenLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(raw)

	path := tokenFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create token dir: %w", err)
	}
	// 0644: owner writes, everyone reads — file is on a path only
	// accessible to local/interactive users (ProgramData / /var/run).
	if err := os.WriteFile(path, []byte(token), 0644); err != nil {
		return "", fmt.Errorf("write token file: %w", err)
	}
	return token, nil
}

// ReadToken reads the session token written by the daemon.
func ReadToken() (string, error) {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	return string(data), nil
}

// RemoveTokenFile deletes the token file on daemon shutdown.
func RemoveTokenFile() {
	_ = os.Remove(tokenFilePath())
}

package ipc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const tokenLength = 32

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

// GenerateToken is kept for compatibility with older daemon builds. New IPC
// authorization uses OS peer identity and per-user ownership instead.
func GenerateToken() (string, error) {
	raw := make([]byte, tokenLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(raw)

	path := tokenFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("create token dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return "", fmt.Errorf("write token file: %w", err)
	}
	return token, nil
}

func ReadToken() (string, error) {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	return string(data), nil
}

func RemoveTokenFile() {
	_ = os.Remove(tokenFilePath())
}

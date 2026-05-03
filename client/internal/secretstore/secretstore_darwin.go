//go:build darwin

package secretstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = "ProIdentity Access"

func Put(name string, value []byte) error {
	encoded := base64.StdEncoding.EncodeToString(value)
	cmd := exec.Command("security", "add-generic-password", "-a", safeName(name), "-s", keychainService, "-w", encoded, "-U")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain store %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Get(name string) ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password", "-a", safeName(name), "-s", keychainService, "-w")
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrNotFound
	}
	value, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("keychain decode %s: %w", name, err)
	}
	return value, nil
}

func Delete(name string) error {
	cmd := exec.Command("security", "delete-generic-password", "-a", safeName(name), "-s", keychainService)
	if out, err := cmd.CombinedOutput(); err != nil && !errors.Is(err, exec.ErrNotFound) {
		_ = out
	}
	return nil
}

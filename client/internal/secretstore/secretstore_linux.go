//go:build linux

package secretstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func Put(name string, value []byte) error {
	encoded := base64.StdEncoding.EncodeToString(value)
	cmd := exec.Command("secret-tool", "store", "--label=ProIdentity Access", "application", "ProIdentity", "name", safeName(name))
	cmd.Stdin = strings.NewReader(encoded)
	if out, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("secret-tool is required for secure storage")
		}
		return fmt.Errorf("secret-tool store %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Get(name string) ([]byte, error) {
	cmd := exec.Command("secret-tool", "lookup", "application", "ProIdentity", "name", safeName(name))
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("secret-tool is required for secure storage")
		}
		return nil, ErrNotFound
	}
	value, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("secret-tool decode %s: %w", name, err)
	}
	return value, nil
}

func Delete(name string) error {
	cmd := exec.Command("secret-tool", "clear", "application", "ProIdentity", "name", safeName(name))
	_ = cmd.Run()
	return nil
}

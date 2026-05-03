package secretstore

import (
	"os"
	"path/filepath"
	"strings"
)

func secretDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ProIdentity", "secrets"), nil
}

func secretPath(name string) (string, error) {
	dir, err := secretDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, safeName(name)+".bin"), nil
}

func safeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == 0 {
		return "secret"
	}
	return b.String()
}

func writeSecretFile(name string, data []byte) error {
	path, err := secretPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func readSecretFile(name string) ([]byte, error) {
	path, err := secretPath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return data, err
}

func deleteSecretFile(name string) error {
	path, err := secretPath(name)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

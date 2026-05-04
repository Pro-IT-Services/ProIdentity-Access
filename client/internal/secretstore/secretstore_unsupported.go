//go:build !windows && !darwin && !linux

package secretstore

import "fmt"

func Put(name string, value []byte) error {
	return fmt.Errorf("secure secret storage is not supported on this platform")
}

func Get(name string) ([]byte, error) {
	return nil, ErrNotFound
}

func Delete(name string) error {
	return nil
}

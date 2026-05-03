//go:build !windows

package daemon

func hardenStorageACL(storageDir string) error {
	return nil
}

//go:build windows

package secretstore

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cryptProtectUIForbidden = 0x1

// Put stores value encrypted with the current Windows user profile via DPAPI.
func Put(name string, value []byte) error {
	protected, err := protect(name, value)
	if err != nil {
		return err
	}
	return writeSecretFile(name, protected)
}

func Get(name string) ([]byte, error) {
	protected, err := readSecretFile(name)
	if err != nil {
		return nil, err
	}
	return unprotect(name, protected)
}

func Delete(name string) error {
	return deleteSecretFile(name)
}

func protect(name string, value []byte) ([]byte, error) {
	var out windows.DataBlob
	desc, err := windows.UTF16PtrFromString("ProIdentity " + name)
	if err != nil {
		return nil, err
	}
	if err := windows.CryptProtectData(dataBlob(value), desc, dataBlob(entropy(name)), 0, nil, cryptProtectUIForbidden, &out); err != nil {
		return nil, fmt.Errorf("dpapi protect %s: %w", name, err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) //nolint:errcheck
	return bytesFromBlob(out), nil
}

func unprotect(name string, value []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(dataBlob(value), nil, dataBlob(entropy(name)), 0, nil, cryptProtectUIForbidden, &out); err != nil {
		return nil, fmt.Errorf("dpapi unprotect %s: %w", name, err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) //nolint:errcheck
	return bytesFromBlob(out), nil
}

func dataBlob(data []byte) *windows.DataBlob {
	if len(data) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func bytesFromBlob(blob windows.DataBlob) []byte {
	if blob.Size == 0 || blob.Data == nil {
		return nil
	}
	out := make([]byte, blob.Size)
	copy(out, unsafe.Slice(blob.Data, blob.Size))
	return out
}

func entropy(name string) []byte {
	return []byte("ProIdentity secret store v1:" + name)
}

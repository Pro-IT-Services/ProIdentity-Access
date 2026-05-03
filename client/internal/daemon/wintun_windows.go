//go:build windows

package daemon

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed wintun_amd64.dll
var wintunDLL []byte

var extractOnce sync.Once

// EnsureWintun extracts the embedded wintun.dll next to the running executable
// so that the wintun package can load it via the normal DLL search path.
// Safe to call multiple times; extraction happens only once.
func EnsureWintun() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	var extractErr error
	extractOnce.Do(func() {
		extractErr = extractWintun()
	})
	return extractErr
}

func extractWintun() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dllPath := filepath.Join(filepath.Dir(exe), "wintun.dll")

	// Skip if already present and non-empty
	if info, err := os.Stat(dllPath); err == nil && info.Size() > 0 {
		return nil
	}

	return os.WriteFile(dllPath, wintunDLL, 0644)
}

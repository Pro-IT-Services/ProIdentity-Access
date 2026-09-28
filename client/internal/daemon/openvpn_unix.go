//go:build !windows

package daemon

import (
	"errors"
	"runtime"
)

// prepareOpenVPNAdapter: macOS and Linux create tun devices on demand; macOS
// has no TAP support at all.
func prepareOpenVPNAdapter(_, devType string, _ int) ([]string, error) {
	if runtime.GOOS == "darwin" && devType == "tap" {
		return nil, errors.New("TAP profiles aren't supported on macOS. Ask your administrator for a TUN profile")
	}
	return nil, nil
}

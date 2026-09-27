//go:build !windows && !darwin

package daemon

import (
	"errors"

	"wg-client/internal/ipc"
)

const updatesSupported = false

func prepareUpdateDir(string) error { return errors.New("updates are not supported on this platform") }

func runInstaller(string, string, string, []ipc.Principal, func(error)) error {
	return errors.New("updates are not supported on this platform")
}

func relaunchApp([]string) {}

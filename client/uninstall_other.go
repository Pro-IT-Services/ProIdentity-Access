//go:build !darwin

package main

import "errors"

func (a *App) UninstallApp(_ bool) error {
	return errors.New("uninstall not supported on this platform")
}
